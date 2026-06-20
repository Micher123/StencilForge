package processor

import (
	"math"
	"math/rand"
	"sort"
)

type LabPixel struct {
	X, Y       int
	L, A, BLab float64
	R, G, B    uint8
}

type Cluster struct {
	L, A, B float64
}

func RGBToLab(r, g, b uint8) (float64, float64, float64) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	// sRGB to linear
	if rf > 0.04045 {
		rf = math.Pow((rf+0.055)/1.055, 2.4)
	} else {
		rf /= 12.92
	}
	if gf > 0.04045 {
		gf = math.Pow((gf+0.055)/1.055, 2.4)
	} else {
		gf /= 12.92
	}
	if bf > 0.04045 {
		bf = math.Pow((bf+0.055)/1.055, 2.4)
	} else {
		bf /= 12.92
	}

	// linear RGB to XYZ (D65)
	x := rf*0.4124564 + gf*0.3575761 + bf*0.1804375
	y := rf*0.2126729 + gf*0.7151522 + bf*0.0721750
	z := rf*0.0193339 + gf*0.1191920 + bf*0.9503041

	// XYZ to Lab (D65 reference)
	xr := x / 0.95047
	yr := y / 1.00000
	zr := z / 1.08883

	fx := labF(xr)
	fy := labF(yr)
	fz := labF(zr)

	l := 116.0*fy - 16.0
	a := 500.0 * (fx - fy)
	bb := 200.0 * (fy - fz)

	return l, a, bb
}

func labF(t float64) float64 {
	delta := 6.0 / 29.0
	if t > delta*delta*delta {
		return math.Cbrt(t)
	}
	return t/(3.0*delta*delta) + 4.0/29.0
}

func labFInv(t float64) float64 {
	delta := 6.0 / 29.0
	if t > delta {
		return t * t * t
	}
	return 3.0 * delta * delta * (t - 4.0/29.0)
}

func LabToRGB(l, a, bb float64) (uint8, uint8, uint8) {
	// Lab to XYZ
	fy := (l + 16.0) / 116.0
	fx := a/500.0 + fy
	fz := fy - bb/200.0

	xr := labFInv(fx)
	yr := labFInv(fy)
	zr := labFInv(fz)

	x := xr * 0.95047
	y := yr * 1.00000
	z := zr * 1.08883

	// XYZ to linear RGB
	rf := x*3.2404542 + y*-1.5371385 + z*-0.4985314
	gf := x*-0.9692660 + y*1.8760108 + z*0.0415560
	bf := x*0.0556434 + y*-0.2040259 + z*1.0572252

	// linear to sRGB
	r := linearToSRGB(rf)
	g := linearToSRGB(gf)
	b := linearToSRGB(bf)

	return r, g, b
}

func linearToSRGB(c float64) uint8 {
	if c <= 0.0031308 {
		c *= 12.92
	} else {
		c = 1.055*math.Pow(c, 1.0/2.4) - 0.055
	}
	c = math.Max(0, math.Min(1, c))
	return uint8(c*255.0 + 0.5)
}

// KMeansPP — k-means++ инициализация + 50 итераций
func KMeansPP(pixels []LabPixel, k int, maxIter int) []Cluster {
	n := len(pixels)
	if n == 0 || k == 0 {
		return nil
	}
	if k > n {
		k = n
	}

	clusters := make([]Cluster, k)

	// k-means++: первый центроид случайный, остальные — с вероятностью пропорционально квадрату расстояния
	used := make([]bool, n)
	first := rand.Intn(n)
	used[first] = true
	clusters[0] = Cluster{L: pixels[first].L, A: pixels[first].A, B: pixels[first].BLab}

	// расстояния до ближайшего центроида
	dists := make([]float64, n)
	for i := range dists {
		dists[i] = 1e18
	}

	for c := 1; c < k; c++ {
		sumDist := 0.0
		for i := 0; i < n; i++ {
			dl := pixels[i].L - clusters[c-1].L
			da := pixels[i].A - clusters[c-1].A
			db := pixels[i].BLab - clusters[c-1].B
			d := dl*dl + da*da + db*db
			if d < dists[i] {
				dists[i] = d
			}
			sumDist += dists[i]
		}

		threshold := rand.Float64() * sumDist
		cumulative := 0.0
		chosen := 0
		for i := 0; i < n; i++ {
			cumulative += dists[i]
			if cumulative >= threshold {
				chosen = i
				break
			}
		}
		used[chosen] = true
		clusters[c] = Cluster{L: pixels[chosen].L, A: pixels[chosen].A, B: pixels[chosen].BLab}
	}

	assignments := make([]int, n)

	for iter := 0; iter < maxIter; iter++ {
		changed := false

		// assignment step
		for i, p := range pixels {
			best := 0
			bestDist := 1e18
			for j, cl := range clusters {
				dl := p.L - cl.L
				da := p.A - cl.A
				db := p.BLab - cl.B
				dist := dl*dl + da*da + db*db
				if dist < bestDist {
					bestDist = dist
					best = j
				}
			}
			if assignments[i] != best {
				assignments[i] = best
				changed = true
			}
		}

		if !changed {
			break
		}

		// update step
		counts := make([]int, k)
		sumL := make([]float64, k)
		sumA := make([]float64, k)
		sumB := make([]float64, k)

		for i, p := range pixels {
			ci := assignments[i]
			counts[ci]++
			sumL[ci] += p.L
			sumA[ci] += p.A
			sumB[ci] += p.BLab
		}

		for i := range clusters {
			if counts[i] > 0 {
				clusters[i].L = sumL[i] / float64(counts[i])
				clusters[i].A = sumA[i] / float64(counts[i])
				clusters[i].B = sumB[i] / float64(counts[i])
			}
		}
	}

	return clusters
}

// AssignPixels возвращает индексы кластеров для каждого пикселя
func AssignPixels(pixels []LabPixel, clusters []Cluster) []int {
	assignments := make([]int, len(pixels))
	for i, p := range pixels {
		best := 0
		bestDist := 1e18
		for j, cl := range clusters {
			dl := p.L - cl.L
			da := p.A - cl.A
			db := p.BLab - cl.B
			dist := dl*dl + da*da + db*db
			if dist < bestDist {
				bestDist = dist
				best = j
			}
		}
		assignments[i] = best
	}
	return assignments
}

// --- постобработка масок ---

// Mask2D — бинарная маска в памяти (true = закрашено)
type Mask2D struct {
	W, H int
	Data []bool
}

func NewMask2D(w, h int) *Mask2D {
	return &Mask2D{W: w, H: h, Data: make([]bool, w*h)}
}

func (m *Mask2D) At(x, y int) bool {
	return m.Data[y*m.W+x]
}

func (m *Mask2D) Set(x, y int, v bool) {
	m.Data[y*m.W+x] = v
}

func (m *Mask2D) Clone() *Mask2D {
	clone := NewMask2D(m.W, m.H)
	copy(clone.Data, m.Data)
	return clone
}

// MedianFilter 3×3 — убирает шум соль/перец
func (m *Mask2D) MedianFilter() {
	filtered := NewMask2D(m.W, m.H)
	for y := 1; y < m.H-1; y++ {
		for x := 1; x < m.W-1; x++ {
			count := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if m.At(x+dx, y+dy) {
						count++
					}
				}
			}
			filtered.Set(x, y, count >= 5)
		}
	}
	copy(m.Data, filtered.Data)
}

// Dilate 3×3
func (m *Mask2D) Dilate() {
	clone := m.Clone()
	for y := 1; y < m.H-1; y++ {
		for x := 1; x < m.W-1; x++ {
			if clone.At(x, y) {
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						m.Set(x+dx, y+dy, true)
					}
				}
			}
		}
	}
}

// Erode 3×3
func (m *Mask2D) Erode() {
	clone := m.Clone()
	for y := 1; y < m.H-1; y++ {
		for x := 1; x < m.W-1; x++ {
			allSet := true
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if !clone.At(x+dx, y+dy) {
						allSet = false
					}
				}
			}
			m.Set(x, y, allSet)
		}
	}
}

// MorphologicalClose = Dilate → Erode (заполняет дырки, сглаживает)
func (m *Mask2D) MorphologicalClose() {
	m.Dilate()
	m.Erode()
}

// MorphologicalOpen = Erode → Dilate (удаляет мелкий шум)
func (m *Mask2D) MorphologicalOpen() {
	m.Erode()
	m.Dilate()
}

// FilterSmallComponents удаляет компоненты < minFraction от всей площади
func (m *Mask2D) FilterSmallComponents(minFraction float64) {
	total := 0
	for _, v := range m.Data {
		if v {
			total++
		}
	}
	minPixels := int(float64(total) * minFraction)

	visited := make([]bool, len(m.Data))
	components := make([][]int, 0) // slices of indices

	for i, v := range m.Data {
		if v && !visited[i] {
			// BFS
			comp := make([]int, 0)
			queue := []int{i}
			visited[i] = true
			for len(queue) > 0 {
				idx := queue[0]
				queue = queue[1:]
				comp = append(comp, idx)
				x := idx % m.W
				y := idx / m.W
				for _, nb := range neighbors4(x, y, m.W, m.H) {
					if m.Data[nb] && !visited[nb] {
						visited[nb] = true
						queue = append(queue, nb)
					}
				}
			}
			components = append(components, comp)
		}
	}

	// удаляем мелкие компоненты
	for _, comp := range components {
		if len(comp) < minPixels {
			for _, idx := range comp {
				m.Data[idx] = false
			}
		}
	}
}

func neighbors4(x, y, w, h int) []int {
	var res []int
	if x > 0 {
		res = append(res, y*w+(x-1))
	}
	if x < w-1 {
		res = append(res, y*w+(x+1))
	}
	if y > 0 {
		res = append(res, (y-1)*w+x)
	}
	if y < h-1 {
		res = append(res, (y+1)*w+x)
	}
	return res
}

// --- сортировка слоёв ---

type LayerInfo struct {
	Index     int     // исходный индекс
	Luminance float64 // средняя яркость
}

// SortLayersByBrightness сортирует индексы слоёв от тёмного к светлому
func SortLayersByBrightness(counts []int, sumR, sumG, sumB []uint64) []int {
	n := len(counts)
	infos := make([]LayerInfo, n)
	for i := 0; i < n; i++ {
		lum := 0.0
		if counts[i] > 0 {
			r := float64(sumR[i]) / float64(counts[i])
			g := float64(sumG[i]) / float64(counts[i])
			b := float64(sumB[i]) / float64(counts[i])
			// относительная яркость (Rec. 601)
			lum = 0.299*r + 0.587*g + 0.114*b
		}
		infos[i] = LayerInfo{Index: i, Luminance: lum}
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Luminance < infos[j].Luminance
	})

	order := make([]int, n)
	for i, info := range infos {
		order[i] = info.Index
	}
	return order
}
