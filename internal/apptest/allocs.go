package apptest

import (
	"testing"
)

// FewestAllocs returns the lowest allocation count per run across samples.
// Sampling limits noise from other goroutines and sync.Pool under the race detector.
// Both counts must be positive. Call it only from sequential tests.
func FewestAllocs(samples, runs int, fn func()) float64 {
	fewest := testing.AllocsPerRun(runs, fn)
	for sample := 1; sample < samples && fewest > 0; sample++ {
		fewest = min(fewest, testing.AllocsPerRun(runs, fn))
	}
	return fewest
}
