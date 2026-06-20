package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"stencilforge/db"
	"stencilforge/processor"

	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

type UploadResponse struct {
	SessionID string `json:"session_id"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

type LayersRequest struct {
	SessionID  string `json:"session_id"`
	NumLayers  int    `json:"num_layers"`
	AutoLayers bool   `json:"auto_layers"`
}

type LayerInfo struct {
	Index       int    `json:"index"`
	DownloadURL string `json:"download_url"`
	DataURL     string `json:"data_url"`
}

type LayersResponse struct {
	SessionID string      `json:"session_id"`
	Layers    []LayerInfo `json:"layers"`
}

var (
	imageStore = make(map[string]image.Image)
	mu         sync.Mutex
)

func CORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next(w, r)
	}
}

func UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Метод не поддерживается"})
		return
	}

	err := r.ParseMultipartForm(50 << 20)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Не удалось обработать загружаемый файл. Попробуйте снова."})
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Файл изображения не найден в запросе. Выберите изображение для загрузки."})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".bmp" && ext != ".tiff" && ext != ".tif" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Неподдерживаемый формат: %s. Поддерживаются PNG, JPG, BMP, TIFF.", ext)})
		return
	}

	img, err := decodeImage(file, ext)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Не удалось прочитать изображение. Возможно, файл повреждён."})
		return
	}

	sessionID := fmt.Sprintf("%x", filepath.Base(header.Filename)) + "_" + fmt.Sprintf("%d", header.Size)
	mu.Lock()
	imageStore[sessionID] = img
	mu.Unlock()

	resp := UploadResponse{
		SessionID: sessionID,
		Width:     img.Bounds().Dx(),
		Height:    img.Bounds().Dy(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func LayersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Метод не поддерживается"})
		return
	}

	var req LayersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Неверный формат запроса. Пожалуйста, обновите страницу и попробуйте снова."})
		return
	}

	// Проверка тарифного лимита пользователя
	userIDStr := r.Header.Get("X-User-ID")
	if userIDStr != "" {
		if uid, err := strconv.Atoi(userIDStr); err == nil {
			user, _ := db.GetUserByID(int64(uid))
			if user != nil && req.NumLayers > user.MaxLayers {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": fmt.Sprintf("Ваш тариф позволяет не более %d слоёв. Уменьшите количество или смените тариф.", user.MaxLayers)})
				return
			}
		}
	}

	mu.Lock()
	src, ok := imageStore[req.SessionID]
	mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Сессия не найдена. Пожалуйста, загрузите изображение заново."})
		return
	}

	bounds := src.Bounds()
	wImg, hImg := bounds.Dx(), bounds.Dy()

	nLayers := req.NumLayers
	if nLayers < 2 {
		nLayers = 2
	}
	if nLayers > 16 {
		nLayers = 16
	}

	pixels := make([]processor.LabPixel, 0, wImg*hImg)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := src.At(x, y).RGBA()
			pr, pg, pb := uint8(r>>8), uint8(g>>8), uint8(b>>8)
			l, a, bb := processor.RGBToLab(pr, pg, pb)
			pixels = append(pixels, processor.LabPixel{X: x, Y: y, L: l, A: a, BLab: bb, R: pr, G: pg, B: pb})
		}
	}

	clusters := processor.KMeansPP(pixels, nLayers, 50)
	assignments := processor.AssignPixels(pixels, clusters)

	// Подсчёт средних RGB для каждого кластера
	clusterCounts := make([]int, nLayers)
	clusterSumR := make([]uint64, nLayers)
	clusterSumG := make([]uint64, nLayers)
	clusterSumB := make([]uint64, nLayers)

	for i, p := range pixels {
		c := assignments[i]
		clusterCounts[c]++
		clusterSumR[c] += uint64(p.R)
		clusterSumG[c] += uint64(p.G)
		clusterSumB[c] += uint64(p.B)
	}

	// Средние цвета кластеров в RGB
	clusterRGB := make([]color.RGBA, nLayers)
	for i := 0; i < nLayers; i++ {
		if clusterCounts[i] > 0 {
			clusterRGB[i] = color.RGBA{
				R: uint8(clusterSumR[i] / uint64(clusterCounts[i])),
				G: uint8(clusterSumG[i] / uint64(clusterCounts[i])),
				B: uint8(clusterSumB[i] / uint64(clusterCounts[i])),
				A: 255,
			}
		} else {
			clusterRGB[i] = color.RGBA{R: 128, G: 128, B: 128, A: 255}
		}
	}

	// Строим бинарные маски и применяем постобработку
	mask2Ds := make([]*processor.Mask2D, nLayers)
	for i := 0; i < nLayers; i++ {
		mask2Ds[i] = processor.NewMask2D(wImg, hImg)
	}
	for i, p := range pixels {
		// x, y относительно bounds.Min
		lx := p.X - bounds.Min.X
		ly := p.Y - bounds.Min.Y
		mask2Ds[assignments[i]].Set(lx, ly, true)
	}

	for i := 0; i < nLayers; i++ {
		mask2Ds[i].MedianFilter()
		mask2Ds[i].MorphologicalClose()
		mask2Ds[i].FilterSmallComponents(0.001) // удаляем компоненты < 0.1% площади
	}

	// Сортировка слоёв от тёмного к светлому
	order := processor.SortLayersByBrightness(clusterCounts, clusterSumR, clusterSumG, clusterSumB)

	// Генерируем image.Gray маски для скачивания и image.RGBA превью
	// в порядке наложения (от тёмного к светлому)
	masks := make([]*image.Gray, nLayers)
	previews := make([]*image.RGBA, nLayers)
	for i := 0; i < nLayers; i++ {
		masks[i] = image.NewGray(bounds)
		previews[i] = image.NewRGBA(bounds)
		white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				masks[i].SetGray(x, y, color.Gray{Y: 255})
				previews[i].SetRGBA(x, y, white)
			}
		}
	}

	// Заполняем в порядке наложения
	for newIdx, origIdx := range order {
		m := mask2Ds[origIdx]
		rgb := clusterRGB[origIdx]
		for y := 0; y < hImg; y++ {
			for x := 0; x < wImg; x++ {
				if m.At(x, y) {
					ax := x + bounds.Min.X
					ay := y + bounds.Min.Y
					masks[newIdx].SetGray(ax, ay, color.Gray{Y: 0})
					previews[newIdx].SetRGBA(ax, ay, rgb)
				}
			}
		}
	}

	layers := make([]LayerInfo, nLayers)
	for i := range masks {
		layers[i] = LayerInfo{
			Index:       i,
			DownloadURL: fmt.Sprintf("/api/download-all?session=%s&layer=%d", req.SessionID, i),
			DataURL:     encodeRGBAToDataURL(previews[i]),
		}
	}

	mu.Lock()
	for i, m := range masks {
		storeKey := fmt.Sprintf("%s_layer_%d", req.SessionID, i)
		imageStore[storeKey] = m
	}
	mu.Unlock()

	resp := LayersResponse{
		SessionID: req.SessionID,
		Layers:    layers,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func DownloadAllHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "Метод не поддерживается"})
		return
	}

	sessionID := r.URL.Query().Get("session")
	layerIdx := r.URL.Query().Get("layer")

	mu.Lock()
	defer mu.Unlock()

	if layerIdx != "" {
		key := fmt.Sprintf("%s_layer_%s", sessionID, layerIdx)
		img, ok := imageStore[key]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Слой не найден. Пожалуйста, сгенерируйте слои заново."})
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=layer_%s.png", layerIdx))
		if err := png.Encode(w, img); err != nil {
			log.Printf("encode error: %v", err)
		}
		return
	}

	// загрузка всех слоев через ZIP
	// пока просто вернем 200
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "use /download-all?session=SESS&layer=N"})
}

func decodeImage(r io.Reader, ext string) (image.Image, error) {
	switch ext {
	case ".png":
		return png.Decode(r)
	case ".jpg", ".jpeg":
		return jpeg.Decode(r)
	case ".bmp":
		return bmp.Decode(r)
	case ".tiff", ".tif":
		return tiff.Decode(r)
	default:
		return nil, fmt.Errorf("unsupported format: %s", ext)
	}
}

func encodeGrayToDataURL(img *image.Gray) string {
	return encodePngToDataURL(img)
}

func encodeRGBAToDataURL(img *image.RGBA) string {
	return encodePngToDataURL(img)
}

func encodePngToDataURL(img image.Image) string {
	var pngBuf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.NoCompression}
	if err := encoder.Encode(&pngBuf, img); err != nil {
		log.Printf("encode png error: %v", err)
		return ""
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBuf.Bytes())
}
