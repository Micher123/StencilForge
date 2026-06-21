package cluster

import (
	"encoding/json"
	"net/http"
)

// ClusterHandlers содержит HTTP-обработчики для кластерных endpoint'ов
type ClusterHandlers struct {
	Node *Node
	// WorkerJobHandler — функция, вызываемая когда приходит задание на worker
	WorkerJobHandler func(w http.ResponseWriter, r *http.Request, job JobRequest) *JobResult
}

// RegisterHandler — POST /api/cluster/register — регистрация нового worker'а (main)
func (h *ClusterHandlers) RegisterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if !h.Node.Config.IsMain() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only main node accepts registrations"})
		return
	}

	var body struct {
		NodeID       string `json:"node_id"`
		AdvertiseURL string `json:"advertise_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if body.NodeID == "" || body.AdvertiseURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id and advertise_url are required"})
		return
	}

	h.Node.RegisterPeer(body.NodeID, body.AdvertiseURL)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// HeartbeatHandler — POST /api/cluster/heartbeat — heartbeat от worker'ов (main)
func (h *ClusterHandlers) HeartbeatHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var body struct {
		NodeID       string `json:"node_id"`
		AdvertiseURL string `json:"advertise_url"`
		IsBusy       bool   `json:"is_busy"`
		ActiveJobs   int    `json:"active_jobs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	h.Node.UpdatePeerHeartbeat(body.NodeID, body.AdvertiseURL, body.IsBusy, body.ActiveJobs)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// PingHandler — GET /api/cluster/ping — проверка живости ноды
func (h *ClusterHandlers) PingHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"node_id": h.Node.Config.NodeID,
		"mode":    h.Node.Config.Mode,
	})
}

// JobHandler — POST /api/cluster/job — приём задания на выполнение (worker)
func (h *ClusterHandlers) JobHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	if !h.Node.Config.IsWorker() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only worker nodes process jobs"})
		return
	}

	var job JobRequest
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid job request"})
		return
	}

	if h.WorkerJobHandler == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "worker job handler not configured"})
		return
	}

	h.Node.SetBusy(true)
	defer h.Node.SetBusy(false)

	result := h.WorkerJobHandler(w, r, job)
	if result == nil {
		result = &JobResult{JobID: job.JobID, Error: "unknown error"}
	}
	writeJSON(w, http.StatusOK, result)
}

// ClusterMiddleware проверяет ClusterToken на cluster-эндпоинтах
func (h *ClusterHandlers) ClusterMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Cluster-Token")
		if token == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"cluster token required"}`))
			return
		}
		if token != h.Node.Config.ClusterToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error":"invalid cluster token"}`))
			return
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
