package fiber

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// benchmarkDependency implements DevTimeDependency interface for benchmarking
//
//nolint:fieldalignment
type benchmarkDependency struct {
	startDelay     time.Duration
	terminateDelay time.Duration
	name           string
}

func (m *benchmarkDependency) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("context canceled: %w", ctx.Err())
	default:
	}

	if m.startDelay > 0 {
		timer := time.NewTimer(m.startDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("context canceled: %w", ctx.Err())
		case <-timer.C:
			// Continue after delay
		}
	}
	return nil
}

func (m *benchmarkDependency) String() string {
	return m.name
}

func (m *benchmarkDependency) Terminate(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("context canceled: %w", ctx.Err())
	default:
	}

	if m.terminateDelay > 0 {
		timer := time.NewTimer(m.terminateDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("context canceled: %w", ctx.Err())
		case <-timer.C:
			// Continue after delay
		}
	}
	return nil
}

func BenchmarkStartDevTimeDependencies(b *testing.B) {
	benchmarkFn := func(b *testing.B, dependencies []DevTimeDependency) {
		b.Helper()

		app := &App{
			configured: Config{
				DevTimeDependencies: dependencies,
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			if err := app.startDevTimeDependencies(ctx); err != nil {
				b.Fatal("Expected no error but got", err)
			}
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
		b.Helper()

		app := &App{
			configured: Config{
				DevTimeDependencies: dependencies,
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			if err := app.shutdownDevTimeDependencies(ctx); err != nil {
				b.Fatal("Expected no error but got", err)
			}
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
		b.Helper()

		app := &App{
			configured: Config{
				DevTimeDependencies: dependencies,
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := app.startDevTimeDependencies(ctx)
			// We expect an error here due to the short timeout
			if err == nil && timeout < time.Microsecond {
				b.Fatal("Expected error due to context cancellation but got none")
			}
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
		b.Helper()

		app := &App{
			configured: Config{
				DevTimeDependencies: dependencies,
			},
		}

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			if err := app.startDevTimeDependencies(ctx); err != nil {
				b.Fatal("Expected no error but got", err)
			}
			if err := app.shutdownDevTimeDependencies(ctx); err != nil {
				b.Fatal("Expected no error but got", err)
			}
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
