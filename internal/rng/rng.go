// Package rng provides the Xoshiro256++ pseudo-random number generator.
// Used for deterministic cluster initialization in the K-means algorithm.
package rng

// Xoshiro256PlusPlus is a random number generator.
type Xoshiro256PlusPlus struct{ s [4]uint64 }

func NewXoshiro256PlusPlus(seed uint64) Xoshiro256PlusPlus {
	sm := splitMix64{x: seed}
	return Xoshiro256PlusPlus{s: [4]uint64{sm.nextU64(), sm.nextU64(), sm.nextU64(), sm.nextU64()}}
}

func (r *Xoshiro256PlusPlus) nextU64() uint64 {
	result := bitsRotateLeft64(r.s[0]+r.s[3], 23) + r.s[0]
	t := r.s[1] << 17
	r.s[2] ^= r.s[0]
	r.s[3] ^= r.s[1]
	r.s[1] ^= r.s[2]
	r.s[0] ^= r.s[3]
	r.s[2] ^= t
	r.s[3] = bitsRotateLeft64(r.s[3], 45)
	return result
}

func (r *Xoshiro256PlusPlus) nextU32() uint32 { return uint32(r.nextU64() >> 32) }

func (r *Xoshiro256PlusPlus) RandomFloat32() float32 {
	v := r.nextU32() >> 8
	return float32(v) * (1.0 / (1 << 24))
}

func (r *Xoshiro256PlusPlus) RandomRange(low, high int) int {
	if high <= low {
		panic("cannot sample empty range")
	}
	rangeU := uint32(high - low)
	product := uint64(r.nextU32()) * uint64(rangeU)
	result := uint32(product >> 32)
	loOrder := uint32(product)
	if loOrder > -rangeU {
		newProduct := uint64(r.nextU32()) * uint64(rangeU)
		newHiOrder := uint32(newProduct >> 32)
		if uint64(loOrder)+uint64(newHiOrder) > maxUint32 {
			result++
		}
	}
	return low + int(result)
}

type splitMix64 struct{ x uint64 }

func (s *splitMix64) nextU64() uint64 {
	s.x += 0x9e3779b97f4a7c15
	z := s.x
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func bitsRotateLeft64(x uint64, k int) uint64 {
	return (x << k) | (x >> (64 - k))
}

const maxUint32 = 1<<32 - 1
