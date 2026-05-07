package conversion

import "math"

func Fma32(x, y, z float32) float32 { return float32(math.FMA(float64(x), float64(y), float64(z))) }
func Sqrt32(x float32) float32      { return float32(math.Sqrt(float64(x))) }
func Min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func MinInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func DivCeil(a, b int) int { return (a + b - 1) / b }
func MaxFloat32() float32  { return math.Float32frombits(0x7f7fffff) }
