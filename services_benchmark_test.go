package fiber

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// benchmarkService implements Service interface for benchmarking
//
//nolint:govet // no need to align fields in this mock implementation
type benchmarkService struct {
	startDelay     time.Duration
	terminateDelay time.Duration
	name           string
}

func (m *benchmarkService) Start(ctx context.Context) error {
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

func (m *benchmarkService) String() string {
	return m.name
}

func (m *benchmarkService) State(ctx context.Context) (string, error) {
	return "", nil
}

func (m *benchmarkService) Terminate(ctx context.Context) error {
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

func BenchmarkStartServices(b *testing.B) {
	benchmarkFn := func(b *testing.B, services []Service) {
		b.Helper()

		app := &App{
			configured: Config{
				Services: services,
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			if err := app.startServices(ctx); err != nil {
				b.Fatal("Expected no error but got", err)
			}
		}
	}

	b.Run("no-services", func(b *testing.B) {
		benchmarkFn(b, []Service{})
	})

	b.Run("single-service", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&benchmarkService{name: "dep1"},
		})
	})

	b.Run("multiple-services", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&benchmarkService{name: "dep1"},
			&benchmarkService{name: "dep2"},
			&benchmarkService{name: "dep3"},
		})
	})

	b.Run("multiple-services-with-delays", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&benchmarkService{name: "dep1", startDelay: 1 * time.Millisecond},
			&benchmarkService{name: "dep2", startDelay: 2 * time.Millisecond},
			&benchmarkService{name: "dep3", startDelay: 3 * time.Millisecond},
		})
	})
}

func BenchmarkShutdownServices(b *testing.B) {
	benchmarkFn := func(b *testing.B, services []Service) {
		b.Helper()

		app := &App{
			configured: Config{
				Services: services,
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			if err := app.shutdownServices(ctx); err != nil {
				b.Fatal("Expected no error but got", err)
			}
		}
	}

	b.Run("no-services", func(b *testing.B) {
		benchmarkFn(b, []Service{})
	})

	b.Run("single-service", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&benchmarkService{name: "dep1"},
		})
	})

	b.Run("multiple-services", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&benchmarkService{name: "dep1"},
			&benchmarkService{name: "dep2"},
			&benchmarkService{name: "dep3"},
		})
	})

	b.Run("multiple-services-with-delays", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&benchmarkService{name: "dep1", terminateDelay: 1 * time.Millisecond},
			&benchmarkService{name: "dep2", terminateDelay: 2 * time.Millisecond},
			&benchmarkService{name: "dep3", terminateDelay: 3 * time.Millisecond},
		})
	})
}

func BenchmarkServicesWithContextCancellation(b *testing.B) {
	benchmarkFn := func(b *testing.B, services []Service, timeout time.Duration) {
		b.Helper()

		app := &App{
			configured: Config{
				Services: services,
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := app.startServices(ctx)
			// We expect an error here due to the short timeout
			if err == nil && timeout < time.Microsecond {
				b.Fatal("Expected error due to context cancellation but got none")
			}
			cancel()
		}
	}

	b.Run("single-service/immediate-cancellation", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&benchmarkService{name: "dep1", startDelay: 10 * time.Millisecond},
		}, 100*time.Millisecond)
	})

	b.Run("multiple-services/immediate-cancellation", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&benchmarkService{name: "dep1", startDelay: 10 * time.Millisecond},
			&benchmarkService{name: "dep2", startDelay: 20 * time.Millisecond},
			&benchmarkService{name: "dep3", startDelay: 30 * time.Millisecond},
		}, 100*time.Millisecond)
	})
}

func BenchmarkServicesMemory(b *testing.B) {
	benchmarkFn := func(b *testing.B, services []Service) {
		b.Helper()

		app := &App{
			configured: Config{
				Services: services,
			},
		}

		b.ResetTimer()
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ctx := context.Background()
			if err := app.startServices(ctx); err != nil {
				b.Fatal("Expected no error but got", err)
			}
			if err := app.shutdownServices(ctx); err != nil {
				b.Fatal("Expected no error but got", err)
			}
		}
	}

	b.Run("no-services", func(b *testing.B) {
		benchmarkFn(b, []Service{})
	})

	b.Run("single-service", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&benchmarkService{name: "dep1"},
		})
	})

	b.Run("multiple-services", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&benchmarkService{name: "dep1"},
			&benchmarkService{name: "dep2"},
			&benchmarkService{name: "dep3"},
		})
	})
}
