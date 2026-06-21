package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"path/filepath"

	"stencilforge/cluster"
)

// UploadHandlerWithCluster — проксирует upload на worker, если есть свободный
func UploadHandlerWithCluster(node *cluster.Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		peer := node.GetLeastBusyPeer()
		if peer == "" {
			UploadHandler(w, r)
			return
		}

		if err := r.ParseMultipartForm(50 << 20); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"Не удалось обработать загружаемый файл."}`))
			return
		}

		file, header, err := r.FormFile("image")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"Файл изображения не найден."}`))
			return
		}
		defer file.Close()

		imageData, err := io.ReadAll(file)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"Ошибка чтения файла."}`))
			return
		}

		ext := filepath.Ext(header.Filename)
		jobID := fmt.Sprintf("upload_%s_%d", header.Filename, header.Size)
		userID := getUserIDFromRequestCluster(r)

		job := cluster.JobRequest{
			JobID:     jobID,
			Type:      "upload",
			ImageData: imageData,
			Ext:       ext,
			UserID:    userID,
		}

		log.Printf("[cluster] forwarding upload job %s to peer %s", jobID, peer)
		result, err := node.ForwardJob(peer, job)
		if err != nil {
			log.Printf("[cluster] forward upload failed: %v, falling back to local", err)
			UploadHandler(w, r)
			return
		}

		if result.Upload == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "worker returned empty result"})
			return
		}

		// Sticky-сессия: запоминаем, что эта сессия на этом worker'е
		sid := result.Upload.SessionID
		node.BindSession(sid, peer)
		log.Printf("[cluster] bound session %s -> peer %s", sid, peer)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result.Upload)
	}
}

// LayersHandlerWithCluster — проксирует генерацию слоёв на worker
func LayersHandlerWithCluster(node *cluster.Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Читаем всё тело (сохраняем для fallback)
		bodyBytes, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"Ошибка чтения запроса."}`))
			return
		}

		var req struct {
			SessionID  string `json:"session_id"`
			NumLayers  int    `json:"num_layers"`
			AutoLayers bool   `json:"auto_layers"`
		}
		if err := json.Unmarshal(bodyBytes, &req); err != nil {
			// Тело не JSON или не та структура — обрабатываем локально
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			LayersHandler(w, r)
			return
		}

		// Sticky-сессия: если есть peer, привязанный к этой сессии, используем его
		peer := node.GetSessionPeer(req.SessionID)
		if peer == "" {
			peer = node.GetLeastBusyPeer()
		}
		if peer == "" {
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			LayersHandler(w, r)
			return
		}

		jobID := fmt.Sprintf("layers_%s_%d", req.SessionID, req.NumLayers)
		userID := getUserIDFromRequestCluster(r)

		job := cluster.JobRequest{
			JobID:     jobID,
			Type:      "layers",
			SessionID: req.SessionID,
			NumLayers: req.NumLayers,
			UserID:    userID,
		}

		log.Printf("[cluster] forwarding layers job %s to peer %s (sticky=%v)", jobID, peer, node.GetSessionPeer(req.SessionID) != "")
		result, err := node.ForwardJob(peer, job)
		if err != nil {
			log.Printf("[cluster] forward layers failed: %v, falling back to local", err)
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			LayersHandler(w, r)
			return
		}

		// Обновляем sticky-привязку
		node.BindSession(req.SessionID, peer)

		if result.Layers == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "worker returned empty result"})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result.Layers)
	}
}

// DownloadAllHandlerWithCluster — проксирует скачивание слоя на worker
func DownloadAllHandlerWithCluster(node *cluster.Node) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.URL.Query().Get("session")
		layerIdx := r.URL.Query().Get("layer")

		if sessionID == "" || layerIdx == "" {
			// Без параметров — обрабатываем локально (список или ZIP)
			DownloadAllHandler(w, r)
			return
		}

		// Sticky: ищем peer по сессии
		peer := node.GetSessionPeer(sessionID)
		if peer == "" {
			// Если нет peer, обрабатываем локально
			DownloadAllHandler(w, r)
			return
		}

		// Отправляем download-job на worker
		job := cluster.JobRequest{
			JobID:     fmt.Sprintf("download_%s_%s", sessionID, layerIdx),
			Type:      "download",
			SessionID: sessionID,
			LayerIdx:  layerIdx,
			UserID:    getUserIDFromRequestCluster(r),
		}

		log.Printf("[cluster] forwarding download %s/%s to peer %s", sessionID, layerIdx, peer)
		result, err := node.ForwardJob(peer, job)
		if err != nil {
			log.Printf("[cluster] forward download failed: %v, falling back to local", err)
			DownloadAllHandler(w, r)
			return
		}

		if result.Download == nil || len(result.Download.PNGData) == 0 {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "worker returned empty download"})
			return
		}

		// Отдаём PNG
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=layer_%s.png", layerIdx))
		w.Write(result.Download.PNGData)
	}
}

func getUserIDFromRequestCluster(r *http.Request) int64 {
	var uid int64
	fmt.Sscanf(r.Header.Get("X-User-ID"), "%d", &uid)
	return uid
}
