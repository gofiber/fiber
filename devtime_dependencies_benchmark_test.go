package fiber

import (
	"context"
	"testing"
	"time"
)

// benchmarkDependency implements DevTimeDependency interface for benchmarking
type benchmarkDependency struct {
	name           string
	startDelay     time.Duration
	terminateDelay time.Duration
}

func (m *benchmarkDependency) Start(_ context.Context) error {
	if m.startDelay > 0 {
		time.Sleep(m.startDelay)
	}
	return nil
}

func (m *benchmarkDependency) String() string {
	return m.name
}

func (m *benchmarkDependency) Terminate(_ context.Context) error {
	if m.terminateDelay > 0 {
		time.Sleep(m.terminateDelay)
	}
	return nil
}

func BenchmarkStartDevTimeDependencies(b *testing.B) {
	benchmarkFn := func(b *testing.B, dependencies []DevTimeDependency) {
		app := &App{
			configured: Config{
				DevTimeDependencies: dependencies,
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			_ = app.startDevTimeDependencies(ctx)
		}
	}

	b.Run("no-dependencies", func(b *testing.B) {
		benchmarkFn(b, []DevTimeDependency{})
	})

	b.Run("single-dependency", func(b *testing.B) {
		benchmarkFn(b, []DevTimeDependency{
			&benchmarkDependency{name: "dep1"},
		})
	})

	b.Run("multiple-dependencies", func(b *testing.B) {
		benchmarkFn(b, []DevTimeDependency{
			&benchmarkDependency{name: "dep1"},
			&benchmarkDependency{name: "dep2"},
			&benchmarkDependency{name: "dep3"},
		})
	})

	b.Run("multiple-dependencies-with-delays", func(b *testing.B) {
		benchmarkFn(b, []DevTimeDependency{
			&benchmarkDependency{name: "dep1", startDelay: 1 * time.Millisecond},
			&benchmarkDependency{name: "dep2", startDelay: 2 * time.Millisecond},
			&benchmarkDependency{name: "dep3", startDelay: 3 * time.Millisecond},
		})
	})
}

func BenchmarkShutdownDevTimeDependencies(b *testing.B) {
	benchmarkFn := func(b *testing.B, dependencies []DevTimeDependency) {
		app := &App{
			configured: Config{
				DevTimeDependencies: dependencies,
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			_ = app.shutdownDevTimeDependencies(ctx)
		}
	}

	b.Run("no-dependencies", func(b *testing.B) {
		benchmarkFn(b, []DevTimeDependency{})
	})

	b.Run("single-dependency", func(b *testing.B) {
		benchmarkFn(b, []DevTimeDependency{
			&benchmarkDependency{name: "dep1"},
		})
	})

	b.Run("multiple-dependencies", func(b *testing.B) {
		benchmarkFn(b, []DevTimeDependency{
			&benchmarkDependency{name: "dep1"},
			&benchmarkDependency{name: "dep2"},
			&benchmarkDependency{name: "dep3"},
		})
	})

	b.Run("multiple-dependencies-with-delays", func(b *testing.B) {
		benchmarkFn(b, []DevTimeDependency{
			&benchmarkDependency{name: "dep1", terminateDelay: 1 * time.Millisecond},
			&benchmarkDependency{name: "dep2", terminateDelay: 2 * time.Millisecond},
			&benchmarkDependency{name: "dep3", terminateDelay: 3 * time.Millisecond},
		})
	})
}

func BenchmarkDevTimeDependenciesWithContextCancellation(b *testing.B) {
	benchmarkFn := func(b *testing.B, dependencies []DevTimeDependency, timeout time.Duration) {
		app := &App{
			configured: Config{
				DevTimeDependencies: dependencies,
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			_ = app.startDevTimeDependencies(ctx)
			cancel()
		}
	}

	b.Run("single-dependency/immediate-cancellation", func(b *testing.B) {
		benchmarkFn(b, []DevTimeDependency{
			&benchmarkDependency{name: "dep1", startDelay: 10 * time.Millisecond},
		}, 1*time.Nanosecond)
	})

	b.Run("multiple-dependencies/immediate-cancellation", func(b *testing.B) {
		benchmarkFn(b, []DevTimeDependency{
			&benchmarkDependency{name: "dep1", startDelay: 10 * time.Millisecond},
			&benchmarkDependency{name: "dep2", startDelay: 20 * time.Millisecond},
			&benchmarkDependency{name: "dep3", startDelay: 30 * time.Millisecond},
		}, 1*time.Nanosecond)
	})
}

func BenchmarkDevTimeDependenciesMemory(b *testing.B) {
	benchmarkFn := func(b *testing.B, dependencies []DevTimeDependency) {
		app := &App{
			configured: Config{
				DevTimeDependencies: dependencies,
			},
		}

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			_ = app.startDevTimeDependencies(ctx)
			_ = app.shutdownDevTimeDependencies(ctx)
		}
	}

	b.Run("no-dependencies", func(b *testing.B) {
		benchmarkFn(b, []DevTimeDependency{})
	})

	b.Run("single-dependency", func(b *testing.B) {
		benchmarkFn(b, []DevTimeDependency{
			&benchmarkDependency{name: "dep1"},
		})
	})

	b.Run("multiple-dependencies", func(b *testing.B) {
		benchmarkFn(b, []DevTimeDependency{
			&benchmarkDependency{name: "dep1"},
			&benchmarkDependency{name: "dep2"},
			&benchmarkDependency{name: "dep3"},
		})
	})
}
