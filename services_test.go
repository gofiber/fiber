package fiber

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/log"
	"github.com/stretchr/testify/require"
)

const (
	terminateErrorMessage = "terminate error"
	startErrorMessage     = "start error"
)

// mockService implements Service interface for testing
type mockService struct {
	startError     error
	terminateError error
	stateError     error
	name           string
	started        bool
	terminated     bool
	startDelay     time.Duration
	terminateDelay time.Duration
}

func (m *mockService) Start(ctx context.Context) error {
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

	if m.startError != nil {
		m.started = false
		return m.startError
	}

	m.started = true
	return nil
}

func (m *mockService) String() string {
	return m.name
}

func (m *mockService) State(ctx context.Context) (string, error) {
	if ctx.Err() != nil {
		return "", fmt.Errorf("context canceled: %w", ctx.Err())
	}

	if m.stateError != nil {
		return "error", m.stateError
	}

	if m.started {
		return "running", nil
	}

	if m.terminated {
		return "stopped", nil
	}

	return "unknown", nil
}

func (m *mockService) Terminate(ctx context.Context) error {
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

	if m.terminateError != nil {
		m.terminated = false
		return m.terminateError
	}

	m.started = false
	m.terminated = true
	return nil
}

func Test_HasConfiguredServices(t *testing.T) {
	testHasConfiguredServicesFn := func(t *testing.T, app *App, expected bool) {
		t.Helper()

		result := app.hasConfiguredServices()
		require.Equal(t, expected, result)
	}

	t.Run("no-services", func(t *testing.T) {
		testHasConfiguredServicesFn(t, &App{configured: Config{}}, false)
	})

	t.Run("has-services", func(t *testing.T) {
		testHasConfiguredServicesFn(t, &App{configured: Config{Services: []Service{&mockService{name: "test-dep"}}}}, true)
	})
}

func Test_InitServices(t *testing.T) {
	t.Run("no-services", func(t *testing.T) {
		app := &App{configured: Config{}}
		require.NotPanics(t, app.initServices)
	})

	t.Run("start/success", func(t *testing.T) {
		// Initialize the app using the struct and defining the state and hooks manually,
		// because we are not checking the shutdown hooks in this test.
		app := &App{
			configured: Config{
				Services: []Service{
					&mockService{name: "dep1"},
					&mockService{name: "dep2"},
				},
			},
			state: newState(),
		}

		app.hooks = newHooks(app)

		require.NotPanics(t, app.initServices)
	})

	t.Run("start/error", func(t *testing.T) {
		// Initialize the app using the struct and defining the state and hooks manually,
		// because we are not checking the shutdown hooks in this test.
		app := &App{
			configured: Config{
				Services: []Service{
					&mockService{name: "dep1", startError: errors.New(startErrorMessage + " 1")},
					&mockService{name: "dep2", startError: errors.New(startErrorMessage + " 2")},
					&mockService{name: "dep3"},
				},
			},
			state: newState(),
		}

		app.hooks = newHooks(app)

		require.Panics(t, app.initServices)
	})

	t.Run("shutdown-hooks/success", func(t *testing.T) {
		// Initialize the app using the New function to verify that the shutdown hooks are registered
		// and the app mutex is not causing a deadlock.
		app := New(Config{
			Services: []Service{&mockService{name: "dep1"}},
		})

		require.NotPanics(t, app.initServices)

		type stringsLogger struct {
			strings.Builder
		}

		var buf stringsLogger
		log.SetOutput(&buf)

		app.Hooks().executeOnPostShutdownHooks(nil)

		require.NotContains(t, buf.String(), "failed to call post shutdown hook:")
	})

	t.Run("shutdown-hooks/error", func(t *testing.T) {
		// Initialize the app using the New function to verify that the shutdown hooks are registered
		// and the app mutex is not causing a deadlock.
		app := New(Config{
			Services: []Service{
				&mockService{name: "dep1"},
				&mockService{name: "dep2", terminateError: errors.New(terminateErrorMessage + " 2")},
			},
		})

		require.NotPanics(t, app.initServices)

		type stringsLogger struct {
			strings.Builder
		}

		var buf stringsLogger
		log.SetOutput(&buf)

		app.Hooks().executeOnPostShutdownHooks(nil)

		require.Contains(t, buf.String(), "failed to shutdown services: service dep2 terminate: terminate error 2")
	})
}

func Test_StartServices(t *testing.T) {
	t.Run("no-services", func(t *testing.T) {
		app := &App{
			configured: Config{
				Services: []Service{},
			},
			state: newState(),
		}

		err := app.startServices(context.Background())
		require.NoError(t, err)
		require.Zero(t, app.state.ServicesLen())
	})

	t.Run("successful-start", func(t *testing.T) {
		app := &App{
			configured: Config{
				Services: []Service{
					&mockService{name: "dep1"},
					&mockService{name: "dep2"},
				},
			},
			state: newState(),
		}

		err := app.startServices(context.Background())
		require.NoError(t, err)
		require.Equal(t, 2, app.state.ServicesLen())
	})

	t.Run("failed-start", func(t *testing.T) {
		app := &App{
			configured: Config{
				Services: []Service{
					&mockService{name: "dep1", startError: errors.New(startErrorMessage + " 1")},
					&mockService{name: "dep2", startError: errors.New(startErrorMessage + " 2")},
					&mockService{name: "dep3"},
				},
			},
			state: newState(),
		}

		err := app.startServices(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), startErrorMessage+" 1")
		require.Contains(t, err.Error(), startErrorMessage+" 2")
		require.Equal(t, 1, app.state.ServicesLen())
	})

	t.Run("context", func(t *testing.T) {
		t.Run("already-canceled", func(t *testing.T) {
			app := &App{
				configured: Config{
					Services: []Service{
						&mockService{name: "dep1"},
					},
				},
				state: newState(),
			}

			// Create a context that is already canceled
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := app.startServices(ctx)
			require.ErrorIs(t, err, context.Canceled)
			require.Zero(t, app.state.ServicesLen())
		})

		t.Run("cancellation", func(t *testing.T) {
			// Create a service that takes some time to start
			slowDep := &mockService{name: "slow-dep", startDelay: 200 * time.Millisecond}

			app := &App{
				configured: Config{
					Services: []Service{slowDep},
				},
				state: newState(),
			}

			// Create a context that will be canceled immediately
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Start services with a delay that is longer than the timeout
			err := app.startServices(ctx)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.Zero(t, app.state.ServicesLen())
		})
	})
}

func Test_ShutdownServices(t *testing.T) {
	t.Run("no-services", func(t *testing.T) {
		app := &App{
			configured: Config{
				Services: []Service{},
			},
			state: newState(),
		}

		err := app.shutdownServices(context.Background())
		require.NoError(t, err)
		require.Zero(t, app.state.ServicesLen())
	})

	t.Run("successful-shutdown", func(t *testing.T) {
		srv1 := &mockService{name: "dep1"}
		srv2 := &mockService{name: "dep2"}

		// Expected state, including the two started services
		expectedState := newState()
		expectedState.setService(srv1)
		expectedState.setService(srv2)

		app := &App{
			configured: Config{
				Services: []Service{srv1, srv2},
			},
			state: expectedState,
		}

		err := app.shutdownServices(context.Background())
		require.NoError(t, err)
		require.Zero(t, app.state.ServicesLen())
	})

	t.Run("failed-shutdown", func(t *testing.T) {
		srv1 := &mockService{name: "dep1", terminateError: errors.New(terminateErrorMessage + " 1")}
		srv2 := &mockService{name: "dep2", terminateError: errors.New(terminateErrorMessage + " 2")}
		srv3 := &mockService{name: "dep3"}

		// Expected state, including the two started services
		expectedState := newState()
		expectedState.setService(srv1)
		expectedState.setService(srv2)
		expectedState.setService(srv3)

		app := &App{
			configured: Config{
				Services: []Service{srv1, srv2, srv3},
			},
			state: expectedState,
		}

		err := app.shutdownServices(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), terminateErrorMessage+" 1")
		require.Contains(t, err.Error(), terminateErrorMessage+" 2")
		require.Equal(t, 2, app.state.ServicesLen()) // 2 services failed to terminate
	})

	t.Run("context", func(t *testing.T) {
		t.Run("already-canceled", func(t *testing.T) {
			srv1 := &mockService{name: "dep1"}

			// Expected state, including the two started services
			expectedState := newState()
			expectedState.setService(srv1)

			app := &App{
				configured: Config{
					Services: []Service{srv1},
				},
				state: expectedState,
			}

			// Create a context that is already canceled
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := app.shutdownServices(ctx)
			require.Error(t, err)
			require.ErrorIs(t, err, context.Canceled)
			require.Contains(t, err.Error(), "service dep1 terminate")
			require.Equal(t, 1, app.state.ServicesLen())
		})

		t.Run("cancellation", func(t *testing.T) {
			// Create a service that takes some time to terminate
			slowDep := &mockService{name: "slow-dep", terminateDelay: 200 * time.Millisecond}

			// Expected state, including the two started services
			expectedState := newState()
			expectedState.setService(slowDep)

			app := &App{
				configured: Config{
					Services: []Service{slowDep},
				},
				state: expectedState,
			}

			// Create a new context for shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Shutdown services with canceled context
			err := app.shutdownServices(ctx)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.Equal(t, 1, app.state.ServicesLen())
		})
	})
}

func Test_LogServices(t *testing.T) {
	// Service with successful State
	runningService := &mockService{name: "running", started: true}
	// Service with State error
	errorService := &mockService{name: "error", stateError: errors.New("state error")}

	expectedState := newState()
	expectedState.setService(runningService)
	expectedState.setService(errorService)

	app := &App{
		configured: Config{
			Services: []Service{runningService, errorService},
		},
		state: expectedState,
	}

	var buf bytes.Buffer

	colors := Colors{
		Green: "\033[32m",
		Reset: "\033[0m",
		Blue:  "\033[34m",
		Red:   "\033[31m",
	}

	app.logServices(context.Background(), &buf, colors)

	output := buf.String()

	for _, srv := range app.state.Services() {
		stateColor := colors.Blue
		state := "RUNNING"
		if _, err := srv.State(context.Background()); err != nil {
			stateColor = colors.Red
			state = "ERROR"
		}

		expected := fmt.Sprintf("%sINFO%s    🥡 %s[ %s ] %s%s\n", colors.Green, colors.Reset, stateColor, strings.ToUpper(state), srv.String(), colors.Reset)
		require.Contains(t, output, expected)
	}
}

func Test_ServiceContextProviders(t *testing.T) {
	t.Run("no-provider", func(t *testing.T) {
		app := &App{
			configured: Config{},
			state:      newState(),
		}
		require.Equal(t, context.Background(), app.servicesStartupCtx())
		require.Equal(t, context.Background(), app.servicesShutdownCtx())
	})

	t.Run("with-provider", func(t *testing.T) {
		ctx := context.TODO()
		app := &App{
			configured: Config{
				ServicesStartupContextProvider: func() context.Context {
					return ctx
				},
				ServicesShutdownContextProvider: func() context.Context {
					return ctx
				},
			},
			state: newState(),
		}

		require.Equal(t, ctx, app.servicesStartupCtx())
		require.Equal(t, ctx, app.servicesShutdownCtx())
	})
}

func Benchmark_StartServices(b *testing.B) {
	benchmarkFn := func(b *testing.B, services []Service) {
		b.Helper()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			app := New(Config{
				Services: services,
			})

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

func Benchmark_ShutdownServices(b *testing.B) {
	benchmarkFn := func(b *testing.B, services []Service) {
		b.Helper()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			app := New(Config{
				Services: services,
			})

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

func Benchmark_StartServices_withContextCancellation(b *testing.B) {
	benchmarkFn := func(b *testing.B, services []Service, timeout time.Duration) {
		b.Helper()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			app := New(Config{
				Services: services,
			})

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
		const timeout = 500 * time.Millisecond

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			app := New(Config{
				Services: []Service{
					&mockService{name: "dep1", startDelay: 10 * time.Millisecond},
					&mockService{name: "dep2", startDelay: 20 * time.Millisecond},
					&mockService{name: "dep3", startDelay: 30 * time.Millisecond},
				},
			})

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := app.startServices(ctx)
			if err != nil {
				b.Fatal("Expected no error but got", err)
			}
			cancel()
		}
	})
}

func Benchmark_ShutdownServices_withContextCancellation(b *testing.B) {
	benchmarkFn := func(b *testing.B, services []Service, timeout time.Duration) {
		b.Helper()

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			app := New(Config{
				Services: services,
			})

			err := app.startServices(context.Background())
			if err != nil {
				b.Fatal("Expected no error during startup but got", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err = app.shutdownServices(ctx)
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
		const timeout = 500 * time.Millisecond

		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			app := New(Config{
				Services: []Service{
					&mockService{name: "dep1", terminateDelay: 10 * time.Millisecond},
					&mockService{name: "dep2", terminateDelay: 20 * time.Millisecond},
					&mockService{name: "dep3", terminateDelay: 30 * time.Millisecond},
				},
			})

			err := app.startServices(context.Background())
			if err != nil {
				b.Fatal("Expected no error but got", err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err = app.shutdownServices(ctx)
			if err != nil {
				b.Fatal("Expected no error but got", err)
			}
			cancel()
		}
	})
}

func Benchmark_ServicesMemory(b *testing.B) {
	benchmarkFn := func(b *testing.B, services []Service) {
		b.Helper()

		b.ResetTimer()
		b.ReportAllocs()

		var err error
		for i := 0; i < b.N; i++ {
			app := New(Config{
				Services: services,
			})

			ctx := context.Background()
			err = app.startServices(ctx)
			if err != nil {
				continue
			}
			err = app.shutdownServices(ctx)
		}
		require.NoError(b, err)
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
