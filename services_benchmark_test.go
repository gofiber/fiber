package fiber

import (
	"context"
	"testing"
	"time"
)

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
			&mockService{name: "dep1"},
		})
	})

	b.Run("multiple-services", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&mockService{name: "dep1"},
			&mockService{name: "dep2"},
			&mockService{name: "dep3"},
		})
	})

	b.Run("multiple-services-with-delays", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&mockService{name: "dep1", startDelay: 1 * time.Millisecond},
			&mockService{name: "dep2", startDelay: 2 * time.Millisecond},
			&mockService{name: "dep3", startDelay: 3 * time.Millisecond},
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
			&mockService{name: "dep1"},
		})
	})

	b.Run("multiple-services", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&mockService{name: "dep1"},
			&mockService{name: "dep2"},
			&mockService{name: "dep3"},
		})
	})

	b.Run("multiple-services-with-delays", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&mockService{name: "dep1", terminateDelay: 1 * time.Millisecond},
			&mockService{name: "dep2", terminateDelay: 2 * time.Millisecond},
			&mockService{name: "dep3", terminateDelay: 3 * time.Millisecond},
		})
	})
}

func BenchmarkStartServices_withContextCancellation(b *testing.B) {
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
			if err == nil && timeout < time.Second {
				b.Fatal("Expected error due to context cancellation but got none")
			}
			cancel()
		}
	}

	b.Run("single-service/immediate-cancellation", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&mockService{name: "dep1", startDelay: 100 * time.Millisecond},
		}, 10*time.Millisecond)
	})

	b.Run("multiple-services/immediate-cancellation", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&mockService{name: "dep1", startDelay: 100 * time.Millisecond},
			&mockService{name: "dep2", startDelay: 200 * time.Millisecond},
			&mockService{name: "dep3", startDelay: 300 * time.Millisecond},
		}, 10*time.Millisecond)
	})

	b.Run("multiple-services/successful-completion", func(b *testing.B) {
		app := &App{
			configured: Config{
				Services: []Service{
					&mockService{name: "dep1", startDelay: 10 * time.Millisecond},
					&mockService{name: "dep2", startDelay: 20 * time.Millisecond},
					&mockService{name: "dep3", startDelay: 30 * time.Millisecond},
				},
			},
		}

		const timeout = 500 * time.Millisecond

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := app.startServices(ctx)
			if err != nil {
				b.Fatal("Expected no error but got", err)
			}
			cancel()
		}
	})
}

func BenchmarkShutdownServices_withContextCancellation(b *testing.B) {
	benchmarkFn := func(b *testing.B, services []Service, timeout time.Duration) {
		b.Helper()

		app := &App{
			startedServices: services,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := app.shutdownServices(ctx)
			// We expect an error here due to the short timeout
			if err == nil && timeout < time.Second {
				b.Fatal("Expected error due to context cancellation but got none")
			}
			cancel()
		}
	}

	b.Run("single-service/immediate-cancellation", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&mockService{name: "dep1", terminateDelay: 100 * time.Millisecond},
		}, 10*time.Millisecond)
	})

	b.Run("multiple-services/immediate-cancellation", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&mockService{name: "dep1", terminateDelay: 100 * time.Millisecond},
			&mockService{name: "dep2", terminateDelay: 200 * time.Millisecond},
			&mockService{name: "dep3", terminateDelay: 300 * time.Millisecond},
		}, 10*time.Millisecond)
	})

	b.Run("multiple-services/successful-completion", func(b *testing.B) {
		app := &App{
			configured: Config{
				Services: []Service{
					&mockService{name: "dep1", terminateDelay: 10 * time.Millisecond},
					&mockService{name: "dep2", terminateDelay: 20 * time.Millisecond},
					&mockService{name: "dep3", terminateDelay: 30 * time.Millisecond},
				},
			},
		}

		const timeout = 500 * time.Millisecond

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := app.shutdownServices(ctx)
			if err != nil {
				b.Fatal("Expected no error but got", err)
			}
			cancel()
		}
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
			&mockService{name: "dep1"},
		})
	})

	b.Run("multiple-services", func(b *testing.B) {
		benchmarkFn(b, []Service{
			&mockService{name: "dep1"},
			&mockService{name: "dep2"},
			&mockService{name: "dep3"},
		})
	})
}
