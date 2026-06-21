package cluster

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// NodeStatus представляет состояние ноды в кластере
type NodeStatus struct {
	NodeID       string    `json:"node_id"`
	AdvertiseURL string    `json:"advertise_url"`
	IsMain       bool      `json:"is_main"`
	IsBusy       bool      `json:"is_busy"`
	LastSeen     time.Time `json:"last_seen"`
	ActiveJobs   int       `json:"active_jobs"`
}

// Node управляет нодой в кластере
type Node struct {
	Config   *Config
	Status   NodeStatus
	mu       sync.RWMutex
	peers    map[string]time.Time // peer URL -> last_seen
	peersMu  sync.RWMutex
	jobQueue chan JobRequest
	jobs     map[string]chan JobResult
	jobsMu   sync.Mutex
	// sessionPeer — связка sessionID -> peer URL для sticky-маршрутизации
	sessionPeer   map[string]string
	sessionPeerMu sync.RWMutex
}

// JobRequest — запрос на выполнение работы на удалённой ноде
type JobRequest struct {
	JobID     string `json:"job_id"`
	Type      string `json:"type"` // "upload", "layers", "download"
	SessionID string `json:"session_id,omitempty"`
	ImageData []byte `json:"image_data,omitempty"` // base64 encoded
	Ext       string `json:"ext,omitempty"`
	NumLayers int    `json:"num_layers,omitempty"`
	LayerIdx  string `json:"layer_idx,omitempty"` // для download-job
	UserID    int64  `json:"user_id,omitempty"`
}

// JobResult — результат работы удалённой ноды
type JobResult struct {
	JobID    string          `json:"job_id"`
	Error    string          `json:"error,omitempty"`
	Upload   *UploadResult   `json:"upload,omitempty"`
	Layers   *LayersResult   `json:"layers,omitempty"`
	Download *DownloadResult `json:"download,omitempty"`
}

// DownloadResult — результат загрузки одного слоя (PNG-данные)
type DownloadResult struct {
	SessionID string `json:"session_id"`
	LayerIdx  string `json:"layer_idx"`
	PNGData   []byte `json:"png_data"`
}

// UploadResult — результат загрузки
type UploadResult struct {
	SessionID string `json:"session_id"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// LayerInfo — информация о слое
type LayerInfo struct {
	Index       int    `json:"index"`
	DownloadURL string `json:"download_url"`
	DataURL     string `json:"data_url"`
}

// LayersResult — результат генерации слоёв
type LayersResult struct {
	SessionID string      `json:"session_id"`
	Layers    []LayerInfo `json:"layers"`
}

// NewNode создаёт новую ноду кластера
func NewNode(cfg *Config) *Node {
	n := &Node{
		Config:      cfg,
		peers:       make(map[string]time.Time),
		jobQueue:    make(chan JobRequest, 100),
		jobs:        make(map[string]chan JobResult),
		sessionPeer: make(map[string]string),
	}
	n.Status.NodeID = cfg.NodeID
	n.Status.AdvertiseURL = cfg.AdvertiseURL
	n.Status.IsMain = cfg.IsMain()
	return n
}

// Start запускает циклы поддержания кластера
func (n *Node) Start() {
	// Heartbeat: уведомляем главную ноду о себе (worker) или проверяем worker'ы (main)
	go n.heartbeatLoop()

	// Регистрируемся на главной при старте если мы worker
	if n.Config.IsWorker() {
		if err := n.registerWithMain(); err != nil {
			log.Printf("[cluster] worker %s: failed to register with main %s: %v", n.Config.NodeID, n.Config.MainURL, err)
		}
	}

	// Проверяем peer'ы (main)
	if n.Config.IsMain() {
		go n.checkPeersLoop()
		log.Printf("[cluster] main node %s (%s) started with %d peers", n.Config.NodeID, n.Config.AdvertiseURL, len(n.Config.Peers))
	}
}

// SetBusy обновляет статус занятости
func (n *Node) SetBusy(busy bool) {
	n.mu.Lock()
	n.Status.IsBusy = busy
	n.mu.Unlock()
	if busy {
		n.Status.ActiveJobs++
	} else {
		n.Status.ActiveJobs--
		if n.Status.ActiveJobs < 0 {
			n.Status.ActiveJobs = 0
		}
	}
}

// IsBusy возвращает загружена ли нода
func (n *Node) IsBusy() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.Status.IsBusy
}

// heartbeatLoop циклически отправляет heartbeat
func (n *Node) heartbeatLoop() {
	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		if n.Config.IsWorker() {
			n.sendHeartbeat()
		}
	}
}

// sendHeartbeat отправляет heartbeat главной ноде
func (n *Node) sendHeartbeat() {
	n.mu.RLock()
	busy := n.Status.IsBusy
	jobs := n.Status.ActiveJobs
	n.mu.RUnlock()

	payload := map[string]interface{}{
		"node_id":       n.Config.NodeID,
		"advertise_url": n.Config.AdvertiseURL,
		"is_busy":       busy,
		"active_jobs":   jobs,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", n.Config.MainURL+"/api/cluster/heartbeat", bytes.NewReader(body))
	if err != nil {
		log.Printf("[cluster] heartbeat: create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Cluster-Token", n.Config.ClusterToken)

	cli := &http.Client{Timeout: 5 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		log.Printf("[cluster] heartbeat to %s failed: %v", n.Config.MainURL, err)
		return
	}
	resp.Body.Close()
}

// registerWithMain регистрирует worker на главной ноде
func (n *Node) registerWithMain() error {
	payload := map[string]interface{}{
		"node_id":       n.Config.NodeID,
		"advertise_url": n.Config.AdvertiseURL,
	}

	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", n.Config.MainURL+"/api/cluster/register", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Cluster-Token", n.Config.ClusterToken)

	cli := &http.Client{Timeout: 10 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return fmt.Errorf("register: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("register: http %d %s", resp.StatusCode, string(respBody))
	}

	log.Printf("[cluster] worker %s registered with main %s", n.Config.NodeID, n.Config.MainURL)
	return nil
}

// checkPeersLoop опрашивает worker'ы на доступность (main)
func (n *Node) checkPeersLoop() {
	ticker := time.NewTicker(15 * time.Second)
	for range ticker.C {
		n.peersMu.RLock()
		peers := make([]string, 0, len(n.peers))
		for url := range n.peers {
			peers = append(peers, url)
		}
		n.peersMu.RUnlock()

		for _, url := range peers {
			n.checkPeer(url)
		}

		// Удаляем мёртвые ноды
		now := time.Now()
		n.peersMu.Lock()
		for url, lastSeen := range n.peers {
			if now.Sub(lastSeen) > 45*time.Second {
				delete(n.peers, url)
				log.Printf("[cluster] removed dead peer: %s", url)
			}
		}
		n.peersMu.Unlock()
	}
}

// checkPeer пингует конкретную ноду
func (n *Node) checkPeer(url string) {
	req, err := http.NewRequest("GET", url+"/api/cluster/ping", nil)
	if err != nil {
		return
	}
	req.Header.Set("X-Cluster-Token", n.Config.ClusterToken)

	cli := &http.Client{Timeout: 5 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()

	n.peersMu.Lock()
	n.peers[url] = time.Now()
	n.peersMu.Unlock()
}

// RegisterPeer регистрирует нового пира (main handler)
func (n *Node) RegisterPeer(nodeID, advertiseURL string) {
	n.peersMu.Lock()
	n.peers[advertiseURL] = time.Now()
	n.peersMu.Unlock()
	log.Printf("[cluster] registered peer: %s -> %s", nodeID, advertiseURL)
}

// UpdatePeerHeartbeat обновляет last_seen (main handler)
func (n *Node) UpdatePeerHeartbeat(nodeID, advertiseURL string, busy bool, activeJobs int) {
	n.peersMu.Lock()
	n.peers[advertiseURL] = time.Now()
	n.peersMu.Unlock()
}

// GetLeastBusyPeer находит наименее загруженный worker (main)
func (n *Node) GetLeastBusyPeer() string {
	n.peersMu.RLock()
	defer n.peersMu.RUnlock()

	// Если нет пиров — возвращаем пустую строку (обработка локально)
	if len(n.peers) == 0 {
		return ""
	}

	// Возвращаем первого попавшегося (упрощённо, т.к. у нас нет метрик занятости)
	// В реальном кластере нужно опрашивать статусы
	for url := range n.peers {
		return url
	}
	return ""
}

// ForwardJob отправляет задание на удалённую ноду
func (n *Node) ForwardJob(peerURL string, job JobRequest) (*JobResult, error) {
	body, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("marshal job: %w", err)
	}

	req, err := http.NewRequest("POST", peerURL+"/api/cluster/job", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Cluster-Token", n.Config.ClusterToken)

	cli := &http.Client{Timeout: 120 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("forward job: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("job failed: http %d %s", resp.StatusCode, string(respBody))
	}

	var result JobResult
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}

	if result.Error != "" {
		return nil, fmt.Errorf("job error: %s", result.Error)
	}

	return &result, nil
}

// SubmitJob добавляет задание в локальную очередь
func (n *Node) SubmitJob(job JobRequest) {
	select {
	case n.jobQueue <- job:
		log.Printf("[cluster] job %s queued", job.JobID)
	default:
		log.Printf("[cluster] job queue full, dropping job %s", job.JobID)
	}
}

// JobQueue возвращает канал очереди заданий
func (n *Node) JobQueue() <-chan JobRequest {
	return n.jobQueue
}

// BindSession связывает sessionID с peer'ом (sticky-сессия)
func (n *Node) BindSession(sessionID, peerURL string) {
	n.sessionPeerMu.Lock()
	n.sessionPeer[sessionID] = peerURL
	n.sessionPeerMu.Unlock()
}

// GetSessionPeer возвращает peer для sessionID (nil если нет)
func (n *Node) GetSessionPeer(sessionID string) string {
	n.sessionPeerMu.RLock()
	defer n.sessionPeerMu.RUnlock()
	return n.sessionPeer[sessionID]
}
