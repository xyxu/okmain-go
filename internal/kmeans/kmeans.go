// Package kmeans implements adaptive K-means clustering in Oklab color space.
// It finds 1-4 dominant colors using K-means++ initialization and Lloyd's
// algorithm with deterministic random restarts.
package kmeans

import (
	"github.com/xyxu/okmain-go/internal/conversion"
	"github.com/xyxu/okmain-go/internal/rng"
	"github.com/xyxu/okmain-go/internal/sampling"
)

const (
	maxCentroids                       = 4
	lloydsMaxIterations                = 300
	lloydsConvergenceTolerance         = 1e-3
	greedyInitNCandidates              = 3
	adaptiveMinCentroidDistanceSquared = 0.005
)

type AdaptiveResult struct {
	Centroids      []conversion.Oklab
	Assignments    []int
	LoopIterations []int
	Converged      []bool
}

type lloydsResult struct {
	centroids      []conversion.Oklab
	assignments    []int
	loopIterations int
	converged      bool
}

type centroidSoA struct {
	l [maxCentroids]float32
	a [maxCentroids]float32
	b [maxCentroids]float32
}

func FindAdaptiveCentroids(rng *rng.Xoshiro256PlusPlus, sample sampling.SampledOklabSoA) AdaptiveResult {
	k := maxCentroids
	loopIterations := make([]int, 0, maxCentroids)
	converged := make([]bool, 0, maxCentroids)
	for {
		result := findLloydsCentroids(rng, sample, k)
		loopIterations = append(loopIterations, result.loopIterations)
		converged = append(converged, result.converged)
		if countSimilarClusters(result.centroids) == 0 || k <= 1 {
			return AdaptiveResult{result.centroids, result.assignments, loopIterations, converged}
		}
		k--
	}
}

func countSimilarClusters(centroids []conversion.Oklab) int {
	count := 0
	for i := 0; i < len(centroids); i++ {
		for j := i + 1; j < len(centroids); j++ {
			if squaredDistance(centroids[i], centroids[j]) < adaptiveMinCentroidDistanceSquared {
				count++
			}
		}
	}
	return count
}

func squaredDistance(x, y conversion.Oklab) float32 {
	dl := x.L - y.L
	da := x.A - y.A
	db := x.B - y.B
	return conversion.Fma32(dl, dl, conversion.Fma32(da, da, db*db))
}

func findLloydsCentroids(rng *rng.Xoshiro256PlusPlus, sample sampling.SampledOklabSoA, k int) lloydsResult {
	n := len(sample.L)
	if k > n {
		k = n
	}
	initPoints := findInitial(rng, sample, k)
	var centroids centroidSoA
	for i := 0; i < maxCentroids; i++ {
		centroids.l[i], centroids.a[i], centroids.b[i] = conversion.MaxFloat32(), conversion.MaxFloat32(), conversion.MaxFloat32()
	}
	for j, idx := range initPoints {
		centroids.l[j] = sample.L[idx]
		centroids.a[j] = sample.A[idx]
		centroids.b[j] = sample.B[idx]
	}
	assignments := make([]uint8, n)
	iterations, converged := lloydsLoop(rng, sample, k, assignments, &centroids)

	centroidsVec := make([]conversion.Oklab, k)
	for j := 0; j < k; j++ {
		centroidsVec[j] = conversion.Oklab{L: centroids.l[j], A: centroids.a[j], B: centroids.b[j]}
	}
	assignmentsInt := make([]int, n)
	for i, a := range assignments {
		assignmentsInt[i] = int(a)
	}
	return lloydsResult{centroidsVec, assignmentsInt, iterations, converged}
}

func lloydsLoop(rng *rng.Xoshiro256PlusPlus, sample sampling.SampledOklabSoA, k int, assignments []uint8, centroids *centroidSoA) (int, bool) {
	for i := 0; i < lloydsMaxIterations; i++ {
		assignPoints(sample, centroids, assignments)
		shiftSquared, counts := updateCentroids(sample, assignments, centroids)
		for j := 0; j < k; j++ {
			if counts[j] == 0 {
				randomPoint := rng.RandomRange(0, len(sample.L))
				centroids.l[j] = sample.L[randomPoint]
				centroids.a[j] = sample.A[randomPoint]
				centroids.b[j] = sample.B[randomPoint]
			}
		}
		if shiftSquared < lloydsConvergenceTolerance {
			return i + 1, true
		}
	}
	return lloydsMaxIterations, false
}

func assignPoints(sample sampling.SampledOklabSoA, centroids *centroidSoA, assignments []uint8) {
	for i := range assignments {
		minD := conversion.MaxFloat32()
		minIdx := 0
		for j := 0; j < maxCentroids; j++ {
			d := squaredDistanceFlat(centroids.l[j], centroids.a[j], centroids.b[j], sample.L[i], sample.A[i], sample.B[i])
			if d < minD {
				minD = d
				minIdx = j
			}
		}
		assignments[i] = uint8(minIdx)
	}
}

func updateCentroids(sample sampling.SampledOklabSoA, assignments []uint8, centroids *centroidSoA) (float32, [maxCentroids]uint32) {
	var countsF [maxCentroids]float32
	var sumsL [maxCentroids]float32
	var sumsA [maxCentroids]float32
	var sumsB [maxCentroids]float32
	for i, assignedC := range assignments {
		l, a, b := sample.L[i], sample.A[i], sample.B[i]
		for k := 0; k < maxCentroids; k++ {
			var mask float32
			if assignedC == uint8(k) {
				mask = 1
			}
			countsF[k] += mask
			sumsL[k] = conversion.Fma32(mask, l, sumsL[k])
			sumsA[k] = conversion.Fma32(mask, a, sumsA[k])
			sumsB[k] = conversion.Fma32(mask, b, sumsB[k])
		}
	}
	var counts [maxCentroids]uint32
	for i := 0; i < maxCentroids; i++ {
		counts[i] = uint32(countsF[i])
	}
	var shiftSquared float32
	for i := 0; i < maxCentroids; i++ {
		if counts[i] == 0 {
			continue
		}
		newL := sumsL[i] / countsF[i]
		newA := sumsA[i] / countsF[i]
		newB := sumsB[i] / countsF[i]
		dl := centroids.l[i] - newL
		da := centroids.a[i] - newA
		db := centroids.b[i] - newB
		centroids.l[i], centroids.a[i], centroids.b[i] = newL, newA, newB
		shiftSquared += conversion.Fma32(dl, dl, conversion.Fma32(da, da, db*db))
	}
	return shiftSquared, counts
}

func squaredDistanceFlat(cL, cA, cB, l, a, b float32) float32 {
	dl := cL - l
	da := cA - a
	db := cB - b
	return conversion.Fma32(dl, dl, conversion.Fma32(da, da, db*db))
}

func findInitial(rng *rng.Xoshiro256PlusPlus, sample sampling.SampledOklabSoA, k int) []int {
	n := len(sample.L)
	if k > n {
		k = n
	}
	initPoints := make([]int, 0, k)
	c0 := rng.RandomRange(0, n)
	initPoints = append(initPoints, c0)
	c0l, c0a, c0b := sample.L[c0], sample.A[c0], sample.B[c0]
	minDistances := make([]float32, n)
	var minDistancesSum float32
	for i := 0; i < n; i++ {
		d := squaredDistanceFlat(sample.L[i], sample.A[i], sample.B[i], c0l, c0a, c0b)
		minDistances[i] = d
		minDistancesSum += d
	}
	candidateMinDistances := [greedyInitNCandidates][]float32{}
	for i := range candidateMinDistances {
		candidateMinDistances[i] = make([]float32, n)
	}
	for step := 1; step < k; step++ {
		var candidates [greedyInitNCandidates]int
		for i := range candidates {
			candidates[i] = sampleByDistance(rng, minDistances, minDistancesSum)
		}
		var potentials [greedyInitNCandidates]float32
		for i := 0; i < n; i++ {
			li, ai, bi, currentMin := sample.L[i], sample.A[i], sample.B[i], minDistances[i]
			for j := 0; j < greedyInitNCandidates; j++ {
				c := candidates[j]
				d := min32(squaredDistanceFlat(sample.L[c], sample.A[c], sample.B[c], li, ai, bi), currentMin)
				candidateMinDistances[j][i] = d
				potentials[j] += d
			}
		}
		bestPotential := conversion.MaxFloat32()
		best := 0
		for i, p := range potentials {
			if p < bestPotential {
				bestPotential = p
				best = i
			}
		}
		minDistances, candidateMinDistances[best] = candidateMinDistances[best], minDistances
		minDistancesSum = bestPotential
		initPoints = append(initPoints, candidates[best])
	}
	return initPoints
}

func sampleByDistance(rng *rng.Xoshiro256PlusPlus, minDistances []float32, sum float32) int {
	randomThreshold := rng.RandomFloat32() * sum
	var cumsum float32
	for i, distance := range minDistances {
		cumsum += distance
		if cumsum > randomThreshold {
			return i
		}
	}
	return len(minDistances) - 1
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
