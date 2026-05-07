// Package sampling handles image downsampling via block averaging.
// Large images are reduced to a maximum of 250,000 pixels while preserving
// the overall color distribution for clustering.
package sampling

import (
	"math"

	"github.com/xyxu/okmain-go/internal/conversion"
)

const maxSampleSize = 250_000

type SampledOklabSoA struct {
	Width  uint16
	Height uint16
	L      []float32
	A      []float32
	B      []float32
}

func BlockSize(width, height uint16) int {
	total := int(width) * int(height)
	if total <= maxSampleSize {
		return 1
	}
	n := int(math.Ceil(math.Sqrt(float64(total) / float64(maxSampleSize))))
	return (n + 3) &^ 3
}

func Sample(width, height uint16, buf []uint8) SampledOklabSoA {
	w := int(width)
	h := int(height)
	n := BlockSize(width, height)
	blocksX := divCeil(w, n)
	blocksY := divCeil(h, n)
	numBlocks := blocksX * blocksY
	result := SampledOklabSoA{
		Width:  uint16(blocksX),
		Height: uint16(blocksY),
		L:      make([]float32, 0, numBlocks),
		A:      make([]float32, 0, numBlocks),
		B:      make([]float32, 0, numBlocks),
	}
	accR := make([]float32, blocksX)
	accG := make([]float32, blocksX)
	accB := make([]float32, blocksX)
	accCount := make([]uint32, blocksX)

	for by := 0; by < blocksY; by++ {
		yStart := by * n
		yEnd := minInt(yStart+n, h)
		for y := yStart; y < yEnd; y++ {
			rowOffset := y * w * 3
			row := buf[rowOffset : rowOffset+w*3]
			for bx := 0; bx < blocksX; bx++ {
				xStart := bx * n
				xEnd := minInt(xStart+n, w)
				for p := xStart * 3; p < xEnd*3; p += 3 {
					accR[bx] += conversion.Srgb8ToF32(row[p])
					accG[bx] += conversion.Srgb8ToF32(row[p+1])
					accB[bx] += conversion.Srgb8ToF32(row[p+2])
					accCount[bx]++
				}
			}
		}
		for bx := 0; bx < blocksX; bx++ {
			count := float32(accCount[bx])
			lab := conversion.LinearSRGBToOklab(accR[bx]/count, accG[bx]/count, accB[bx]/count)
			result.L = append(result.L, lab.L)
			result.A = append(result.A, lab.A)
			result.B = append(result.B, lab.B)
			accR[bx], accG[bx], accB[bx], accCount[bx] = 0, 0, 0, 0
		}
	}
	return result
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func divCeil(a, b int) int { return (a + b - 1) / b }
