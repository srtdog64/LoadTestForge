package randutil

import (
	"sync"
	"testing"
)

func TestGetRelease(t *testing.T) {
	rng := Get()
	if rng == nil {
		t.Fatal("Get() returned nil")
	}
	if rng.Rand == nil {
		t.Fatal("Get() returned Rand with nil inner rand")
	}

	// Use it
	_ = rng.Intn(100)

	// Release it
	rng.Release()

	// After release, Rand should be nil
	if rng.Rand != nil {
		t.Error("Release() did not nil out Rand")
	}
}

func TestConcurrentAccess(t *testing.T) {
	const goroutines = 100
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				rng := Get()
				_ = rng.Intn(100)
				_ = rng.Float32()
				rng.Release()
			}
		}()
	}

	wg.Wait()
}

func TestConvenienceFunctions(t *testing.T) {
	// Test Intn
	for i := 0; i < 100; i++ {
		n := Intn(10)
		if n < 0 || n >= 10 {
			t.Errorf("Intn(10) returned %d, want [0, 10)", n)
		}
	}

	// Test Int63n
	for i := 0; i < 100; i++ {
		n := Int63n(1000)
		if n < 0 || n >= 1000 {
			t.Errorf("Int63n(1000) returned %d, want [0, 1000)", n)
		}
	}

	// Test Float32
	for i := 0; i < 100; i++ {
		f := Float32()
		if f < 0 || f >= 1 {
			t.Errorf("Float32() returned %f, want [0.0, 1.0)", f)
		}
	}

	// Test Float64
	for i := 0; i < 100; i++ {
		f := Float64()
		if f < 0 || f >= 1 {
			t.Errorf("Float64() returned %f, want [0.0, 1.0)", f)
		}
	}

	// Test Perm
	perm := Perm(10)
	if len(perm) != 10 {
		t.Errorf("Perm(10) returned slice of length %d, want 10", len(perm))
	}

	// Verify it's a permutation (contains all numbers 0-9)
	seen := make(map[int]bool)
	for _, v := range perm {
		if v < 0 || v >= 10 {
			t.Errorf("Perm(10) contained invalid value %d", v)
		}
		if seen[v] {
			t.Errorf("Perm(10) contained duplicate value %d", v)
		}
		seen[v] = true
	}

	// Test Shuffle
	slice := []int{1, 2, 3, 4, 5}
	original := make([]int, len(slice))
	copy(original, slice)

	Shuffle(len(slice), func(i, j int) {
		slice[i], slice[j] = slice[j], slice[i]
	})

	// Verify same elements (order may differ)
	sum := 0
	for _, v := range slice {
		sum += v
	}
	if sum != 15 {
		t.Errorf("Shuffle changed element values, sum=%d want 15", sum)
	}
}

func BenchmarkGlobalRand(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = Intn(1000)
		}
	})
}

func BenchmarkPooledRand(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rng := Get()
			_ = rng.Intn(1000)
			rng.Release()
		}
	})
}

func BenchmarkPooledRandBatch(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			rng := Get()
			for i := 0; i < 10; i++ {
				_ = rng.Intn(1000)
			}
			rng.Release()
		}
	})
}
