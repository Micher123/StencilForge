package handlers

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"log"
	"strings"
	"sync"

	"stencilforge/cluster"
	"stencilforge/processor"
)

// workerImageStore — отдельное хранилище для worker-нод
var (
	workerImageStore = make(map[string]image.Image)
	workerMu         sync.Mutex
)

// HandleUploadJob обрабатывает задание загрузки изображения на worker-ноде
func HandleUploadJob(job cluster.JobRequest) *cluster.JobResult {
	if len(job.ImageData) == 0 {
		return &cluster.JobResult{JobID: job.JobID, Error: "no image data"}
	}

	ext := strings.ToLower(job.Ext)
	img, err := decodeImageBytes(job.ImageData, ext)
	if err != nil {
		log.Printf("[worker] upload: decode error: %v", err)
		return &cluster.JobResult{JobID: job.JobID, Error: "failed to decode image"}
	}

	sessionID := job.JobID
	workerMu.Lock()
	workerImageStore[sessionID] = img
	workerMu.Unlock()

	return &cluster.JobResult{
		JobID: job.JobID,
		Upload: &cluster.UploadResult{
			SessionID: sessionID,
			Width:     img.Bounds().Dx(),
			Height:    img.Bounds().Dy(),
		},
	}
}

// HandleLayersJob обрабатывает задание генерации слоёв на worker-ноде
func HandleLayersJob(job cluster.JobRequest) *cluster.JobResult {
	workerMu.Lock()
	src, ok := workerImageStore[job.SessionID]
	workerMu.Unlock()

	if !ok {
		return &cluster.JobResult{JobID: job.JobID, Error: "session not found"}
	}

	bounds := src.Bounds()
	wImg, hImg := bounds.Dx(), bounds.Dy()

	nLayers := job.NumLayers
	if nLayers < 2 {
		nLayers = 2
	}
	if nLayers > 32 {
		nLayers = 32
	}

	pixels := make([]processor.LabPixel, 0, wImg*hImg)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := src.At(x, y).RGBA()
			pr, pg, pb := uint8(r>>8), uint8(g>>8), uint8(b>>8)
			l, a, bbLab := processor.RGBToLab(pr, pg, pb)
			pixels = append(pixels, processor.LabPixel{X: x, Y: y, L: l, A: a, BLab: bbLab, R: pr, G: pg, B: pb})
		}
	}

	clusters := processor.KMeansPP(pixels, nLayers, 50)
	assignments := processor.AssignPixels(pixels, clusters)

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

	mask2Ds := make([]*processor.Mask2D, nLayers)
	for i := 0; i < nLayers; i++ {
		mask2Ds[i] = processor.NewMask2D(wImg, hImg)
	}
	for i, p := range pixels {
		lx := p.X - bounds.Min.X
		ly := p.Y - bounds.Min.Y
		mask2Ds[assignments[i]].Set(lx, ly, true)
	}

	for i := 0; i < nLayers; i++ {
		mask2Ds[i].MedianFilter()
		mask2Ds[i].MorphologicalClose()
		mask2Ds[i].FilterSmallComponents(0.001)
	}

	order := processor.SortLayersByBrightness(clusterCounts, clusterSumR, clusterSumG, clusterSumB)

	previews := make([]*image.RGBA, nLayers)
	for i := 0; i < nLayers; i++ {
		previews[i] = image.NewRGBA(bounds)
		white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				previews[i].SetRGBA(x, y, white)
			}
		}
	}

	for newIdx, origIdx := range order {
		m := mask2Ds[origIdx]
		rgb := clusterRGB[origIdx]
		for y := 0; y < hImg; y++ {
			for x := 0; x < wImg; x++ {
				if m.At(x, y) {
					ax := x + bounds.Min.X
					ay := y + bounds.Min.Y
					previews[newIdx].SetRGBA(ax, ay, rgb)
				}
			}
		}
	}

	// Сохраняем preview-слои для последующего скачивания
	for i, p := range previews {
		key := job.SessionID + "_layer_" + itoaCluster(i)
		workerMu.Lock()
		workerImageStore[key] = p
		workerMu.Unlock()
	}

	layers := make([]cluster.LayerInfo, nLayers)
	for i := range previews {
		layers[i] = cluster.LayerInfo{
			Index:       i,
			DownloadURL: "/api/download-all?session=" + job.SessionID + "&layer=" + itoaCluster(i),
			DataURL:     encodeRGBAToDataURL(previews[i]),
		}
	}

	return &cluster.JobResult{
		JobID: job.JobID,
		Layers: &cluster.LayersResult{
			SessionID: job.SessionID,
			Layers:    layers,
		},
	}
}

// HandleDownloadJob возвращает PNG-изображение слоя из workerImageStore
func HandleDownloadJob(job cluster.JobRequest) *cluster.JobResult {
	key := job.SessionID + "_layer_" + job.LayerIdx
	workerMu.Lock()
	img, ok := workerImageStore[key]
	workerMu.Unlock()

	if !ok {
		return &cluster.JobResult{JobID: job.JobID, Error: "layer not found: " + key}
	}

	// Кодируем в PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return &cluster.JobResult{JobID: job.JobID, Error: "encode png: " + err.Error()}
	}

	return &cluster.JobResult{
		JobID: job.JobID,
		Download: &cluster.DownloadResult{
			SessionID: job.SessionID,
			LayerIdx:  job.LayerIdx,
			PNGData:   buf.Bytes(),
		},
	}
}

func decodeImageBytes(data []byte, ext string) (image.Image, error) {
	reader := bytes.NewReader(data)
	return decodeImage(reader, ext)
}

func itoaCluster(i int) string {
	if i == 0 {
		return "0"
	}
	res := ""
	for i > 0 {
		res = string(rune('0'+i%10)) + res
		i /= 10
	}
	return res
}
