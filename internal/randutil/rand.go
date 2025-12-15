// Package randutil provides thread-safe random number generation
// optimized for high-concurrency scenarios.
//
// The standard math/rand package uses a global mutex-protected source,
// which can become a bottleneck under high CPS (connections per second).
// This package provides per-goroutine random sources via sync.Pool.
package randutil

import (
	"math/rand"
	"sync"
	"time"
)

// pool maintains a pool of *rand.Rand instances for reuse.
// Each goroutine gets its own Rand from the pool, eliminating lock contention.
var pool = sync.Pool{
	New: func() interface{} {
		// Use crypto/rand for seed would be ideal, but time-based is sufficient
		// for load testing randomization (not security-sensitive).
		return rand.New(rand.NewSource(time.Now().UnixNano() + int64(rand.Int63())))
	},
}

// Rand represents a pooled random source that should be released after use.
type Rand struct {
	*rand.Rand
}

// Get retrieves a random source from the pool.
// The caller MUST call Release() when done, typically via defer.
//
// Example:
//
//	rng := randutil.Get()
//	defer rng.Release()
//	value := rng.Intn(100)
func Get() *Rand {
	return &Rand{Rand: pool.Get().(*rand.Rand)}
}

// Release returns the random source to the pool.
func (r *Rand) Release() {
	if r.Rand != nil {
		pool.Put(r.Rand)
		r.Rand = nil
	}
}

// Quick convenience functions for simple cases where pool overhead isn't worth it.
// These still use the global rand but are provided for API consistency.

// Intn returns a random int in [0, n) using a pooled source.
func Intn(n int) int {
	rng := Get()
	defer rng.Release()
	return rng.Rand.Intn(n)
}

// Int63n returns a random int64 in [0, n) using a pooled source.
func Int63n(n int64) int64 {
	rng := Get()
	defer rng.Release()
	return rng.Rand.Int63n(n)
}

// Float32 returns a random float32 in [0.0, 1.0) using a pooled source.
func Float32() float32 {
	rng := Get()
	defer rng.Release()
	return rng.Rand.Float32()
}

// Float64 returns a random float64 in [0.0, 1.0) using a pooled source.
func Float64() float64 {
	rng := Get()
	defer rng.Release()
	return rng.Rand.Float64()
}

// Perm returns a random permutation of [0, n) using a pooled source.
func Perm(n int) []int {
	rng := Get()
	defer rng.Release()
	return rng.Rand.Perm(n)
}

// Shuffle randomizes the order of elements using a pooled source.
func Shuffle(n int, swap func(i, j int)) {
	rng := Get()
	defer rng.Release()
	rng.Rand.Shuffle(n, swap)
}
