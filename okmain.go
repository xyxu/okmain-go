// Package okmain provides dominant color extraction from images using K-means clustering
// in the Oklab color space. The algorithm is deterministic.
//
// Supports JPEG, PNG, GIF, WebP, and other formats via standard library and pure-Go decoders.
package okmain

import (
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"math"
	"net/http"
	"os"

	xwebp "golang.org/x/image/webp"
	"github.com/xyxu/okmain-go/internal/conversion"
	"github.com/xyxu/okmain-go/internal/kmeans"
	"github.com/xyxu/okmain-go/internal/rng"
	"github.com/xyxu/okmain-go/internal/sampling"
)

const (
	DefaultMaskSaturatedThreshold float32 = 0.3
	DefaultMaskWeight             float32 = 1.0
	DefaultWeightedCountsWeight   float32 = 0.3
	DefaultChromaWeight           float32 = 0.7
)

const maxSRGBOklabChroma = 0.32

var (
	ErrEmptyBuffer   = errors.New("buffer is empty")
	ErrZeroImageSize = errors.New("image size must be positive")
)

type RGB struct {
	R uint8
	G uint8
	B uint8
}

func (c RGB) Hex() string {
	return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
}

type InputImage struct {
	Width  uint16
	Height uint16
	Buf    []uint8
}

func NewInputImage(width, height uint16, buf []uint8) (InputImage, error) {
	if len(buf) == 0 {
		return InputImage{}, ErrEmptyBuffer
	}
	if width == 0 || height == 0 {
		return InputImage{}, ErrZeroImageSize
	}
	if len(buf)%3 != 0 {
		return InputImage{}, fmt.Errorf("buffer length %d is not a multiple of 3", len(buf))
	}
	if len(buf) != int(width)*int(height)*3 {
		return InputImage{}, fmt.Errorf("image size (%dx%d) doesn't match the buffer size (%d)", width, height, len(buf))
	}
	return InputImage{Width: width, Height: height, Buf: buf}, nil
}

func NewInputImageFromImage(img image.Image) (InputImage, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return InputImage{}, ErrZeroImageSize
	}
	if width > int(^uint16(0)) || height > int(^uint16(0)) {
		return InputImage{}, fmt.Errorf("image dimensions are too large, max image size is %dx%d, got %dx%d", ^uint16(0), ^uint16(0), width, height)
	}
	buf := make([]uint8, 0, width*height*3)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			buf = append(buf, uint8(r>>8), uint8(g>>8), uint8(b>>8))
		}
	}
	return NewInputImage(uint16(width), uint16(height), buf)
}

// NewInputImageFromFile loads an image file (JPEG, PNG, GIF, WebP, etc.)
// and returns the pixel buffer.
func NewInputImageFromFile(path string) (InputImage, error) {
	file, err := os.Open(path)
	if err != nil {
		return InputImage{}, err
	}
	defer file.Close()

	mimeType, err := detectImageMIME(file)
	if err != nil {
		return InputImage{}, err
	}

	if _, err := file.Seek(0, 0); err != nil {
		return InputImage{}, err
	}

	var img image.Image
	if mimeType == "image/webp" {
		img, err = xwebp.Decode(file)
	} else {
		img, _, err = image.Decode(file)
	}
	if err != nil {
		return InputImage{}, err
	}

	fmt.Printf("mimeType: %s\n", mimeType)

	return NewInputImageFromImage(img)
}

func detectImageMIME(file *os.File) (string, error) {
	var header [512]byte
	n, err := file.Read(header[:])
	if err != nil {
		return "", err
	}
	return http.DetectContentType(header[:n]), nil
}



type Config struct {
	MaskSaturatedThreshold   float32
	MaskWeight               float32
	MaskWeightedCountsWeight float32
	ChromaWeight             float32
}

func DefaultConfig() Config {
	return Config{
		MaskSaturatedThreshold:   DefaultMaskSaturatedThreshold,
		MaskWeight:               DefaultMaskWeight,
		MaskWeightedCountsWeight: DefaultWeightedCountsWeight,
		ChromaWeight:             DefaultChromaWeight,
	}
}

type Oklab struct {
	L float32
	A float32
	B float32
}

type ScoredCentroid struct {
	Oklab                   Oklab
	RGB                     RGB
	MaskWeightedCounts      float32
	MaskWeightedCountsScore float32
	Chroma                  float32
	ChromaScore             float32
	FinalScore              float32
}

type DebugInfo struct {
	ScoredCentroids      []ScoredCentroid
	KMeansLoopIterations []int
	KMeansConverged      []bool
}

func Colors(input InputImage) []RGB {
	out, err := ColorsWithConfig(input, DefaultConfig())
	if err != nil {
		panic(err)
	}
	return out
}

func ColorsWithConfig(input InputImage, config Config) ([]RGB, error) {
	colors, _, err := ColorsDebug(input, config)
	return colors, err
}

func ColorsDebug(input InputImage, config Config) ([]RGB, DebugInfo, error) {
	if !(config.MaskSaturatedThreshold >= 0.0 && config.MaskSaturatedThreshold < 0.5) {
		return nil, DebugInfo{}, fmt.Errorf("invalid mask_saturated_threshold: %v (must be in [0, 0.5))", config.MaskSaturatedThreshold)
	}
	if !(config.MaskWeight >= 0.0 && config.MaskWeight <= 1.0) {
		return nil, DebugInfo{}, fmt.Errorf("invalid mask_weight: %v (must be in [0, 1])", config.MaskWeight)
	}
	if !(config.MaskWeightedCountsWeight >= 0.0 && config.MaskWeightedCountsWeight <= 1.0) {
		return nil, DebugInfo{}, fmt.Errorf("invalid weighted_counts_weight: %v (must be in [0, 1])", config.MaskWeightedCountsWeight)
	}
	if !(config.ChromaWeight >= 0.0 && config.ChromaWeight <= 1.0) {
		return nil, DebugInfo{}, fmt.Errorf("invalid chroma_weight: %v (must be in [0, 1])", config.ChromaWeight)
	}
	weightSum := config.MaskWeightedCountsWeight + config.ChromaWeight
	if float32(math.Abs(float64(weightSum-1.0))) >= 1e-5 {
		return nil, DebugInfo{}, fmt.Errorf("mask_weighted_counts_weight (%v) and chroma_weight (%v) don't add up to 1.0 (sum: %v)", config.MaskWeightedCountsWeight, config.ChromaWeight, weightSum)
	}

	rng := rng.NewXoshiro256PlusPlus(314159)
	sampled := sampling.Sample(input.Width, input.Height, input.Buf)
	centroidsResult := kmeans.FindAdaptiveCentroids(&rng, sampled)

	weightedCounts := make([]float32, len(centroidsResult.Centroids))
	for i, assignment := range centroidsResult.Assignments {
		x := uint16(i % int(sampled.Width))
		y := uint16(i / int(sampled.Width))
		maskValue := DistanceMask(config.MaskSaturatedThreshold, sampled.Width, sampled.Height, x, y)
		w := float32(1.0) - config.MaskWeight*(float32(1.0)-maskValue)
		weightedCounts[assignment] += w
	}

	var total float32
	for _, wc := range weightedCounts {
		total += wc
	}
	if total > 0 {
		for i := range weightedCounts {
			weightedCounts[i] /= total
		}
	}

	scored := make([]ScoredCentroid, len(centroidsResult.Centroids))
	for i, lab := range centroidsResult.Centroids {
		crgb := conversion.OklabToSRGB(lab)
		rgb := RGB{crgb.R, crgb.G, crgb.B}
		oklab := Oklab{lab.L, lab.A, lab.B}
		maskWeightedCounts := weightedCounts[i]
		maskWeightedCountsScore := maskWeightedCounts * config.MaskWeightedCountsWeight
		chroma := conversion.Sqrt32(lab.A*lab.A+lab.B*lab.B) / maxSRGBOklabChroma
		chromaScore := chroma * config.ChromaWeight
		finalScore := maskWeightedCountsScore + chromaScore
		scored[i] = ScoredCentroid{oklab, rgb, maskWeightedCounts, maskWeightedCountsScore, chroma, chromaScore, finalScore}
	}

	sortSlice(scored)

	result := make([]RGB, len(scored))
	for i, sc := range scored {
		result[i] = sc.RGB
	}
	return result, DebugInfo{scored, centroidsResult.LoopIterations, centroidsResult.Converged}, nil
}

func DistanceMask(saturatedThreshold float32, width, height, x, y uint16) float32 {
	wf := float32(width)
	hf := float32(height)
	xf := float32(x)
	yf := float32(y)
	middleX := wf / 2.0
	if xf > middleX {
		xf = wf - xf
	}
	middleY := hf / 2.0
	if yf > middleY {
		yf = hf - yf
	}
	xThreshold := wf * saturatedThreshold
	yThreshold := hf * saturatedThreshold
	xContribution := min32(0.1+0.9*(xf/xThreshold), 1.0)
	yContribution := min32(0.1+0.9*(yf/yThreshold), 1.0)
	return min32(xContribution, yContribution)
}

func sortSlice(scored []ScoredCentroid) {
	for i := 0; i < len(scored); i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[j].FinalScore > scored[i].FinalScore {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
