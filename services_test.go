package fiber

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

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

func TestHasServices(t *testing.T) {
	testHasServicesFn := func(t *testing.T, app *App, expected bool) {
		t.Helper()

		result := app.hasServices()
		require.Equal(t, expected, result)
	}

	t.Run("no-services", func(t *testing.T) {
		testHasServicesFn(t, &App{configured: Config{}}, false)
	})

	t.Run("has-services", func(t *testing.T) {
		testHasServicesFn(t, &App{configured: Config{Services: []Service{&mockService{name: "test-dep"}}}}, true)
	})
}

func TestStartServices(t *testing.T) {
	t.Run("no-services", func(t *testing.T) {
		app := &App{
			configured: Config{
				Services: []Service{},
			},
		}

		err := app.startServices(context.Background())
		require.NoError(t, err)
		require.Empty(t, app.startedServices)
	})

	t.Run("successful-start", func(t *testing.T) {
		app := &App{
			configured: Config{
				Services: []Service{
					&mockService{name: "dep1"},
					&mockService{name: "dep2"},
				},
			},
		}

		err := app.startServices(context.Background())
		require.NoError(t, err)
		require.Len(t, app.startedServices, 2)
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
		}

		err := app.startServices(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), startErrorMessage+" 1")
		require.Contains(t, err.Error(), startErrorMessage+" 2")
		require.Len(t, app.startedServices, 1)
	})

	t.Run("context", func(t *testing.T) {
		t.Run("already-canceled", func(t *testing.T) {
			app := &App{
				configured: Config{
					Services: []Service{
						&mockService{name: "dep1"},
					},
				},
			}

			// Create a context that is already canceled
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := app.startServices(ctx)
			require.ErrorIs(t, err, context.Canceled)
			require.Empty(t, app.startedServices)
		})

		t.Run("cancellation", func(t *testing.T) {
			// Create a service that takes some time to start
			slowDep := &mockService{name: "slow-dep", startDelay: 200 * time.Millisecond}

			app := &App{
				configured: Config{
					Services: []Service{slowDep},
				},
			}

			// Create a context that will be canceled immediately
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Start services with a delay that is longer than the timeout
			err := app.startServices(ctx)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.Empty(t, app.startedServices)
		})
	})
}

func TestShutdownServices(t *testing.T) {
	t.Run("no-services", func(t *testing.T) {
		app := New(Config{
			Services: []Service{},
		})

		err := app.shutdownServices(context.Background())
		require.NoError(t, err)
	})

	t.Run("successful-shutdown", func(t *testing.T) {
		app := New(Config{
			Services: []Service{
				&mockService{name: "dep1"},
				&mockService{name: "dep2"},
			},
		})

		err := app.shutdownServices(context.Background())
		require.NoError(t, err)
	})

	t.Run("failed-shutdown", func(t *testing.T) {
		app := New(Config{
			Services: []Service{
				&mockService{name: "dep1", terminateError: errors.New(terminateErrorMessage + " 1")},
				&mockService{name: "dep2", terminateError: errors.New(terminateErrorMessage + " 2")},
				&mockService{name: "dep3"},
			},
		})

		err := app.shutdownServices(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), terminateErrorMessage+" 1")
		require.Contains(t, err.Error(), terminateErrorMessage+" 2")
	})

	t.Run("context", func(t *testing.T) {
		t.Run("already-canceled", func(t *testing.T) {
			app := New(Config{
				Services: []Service{
					&mockService{name: "dep1"},
				},
			})

			// Create a context that is already canceled
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := app.shutdownServices(ctx)
			require.Error(t, err)
			require.ErrorIs(t, err, context.Canceled)
			require.Contains(t, err.Error(), "service dep1 terminate")
		})

		t.Run("cancellation", func(t *testing.T) {
			// Create a service that takes some time to terminate
			slowDep := &mockService{name: "slow-dep", terminateDelay: 200 * time.Millisecond}

			app := New(Config{
				Services: []Service{slowDep},
			})

			// Create a new context for shutdown
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			// Shutdown services with canceled context
			err := app.shutdownServices(ctx)
			require.ErrorIs(t, err, context.DeadlineExceeded)
		})
	})
}

func TestLogServices(t *testing.T) {
	// Service with successful State
	runningService := &mockService{name: "running", started: true}
	// Service with State error
	errorService := &mockService{name: "error", stateError: errors.New("state error")}

	app := New(Config{
		Services: []Service{runningService, errorService},
	})

	var buf bytes.Buffer

	colors := Colors{
		Green: "\033[32m",
		Reset: "\033[0m",
		Blue:  "\033[34m",
		Red:   "\033[31m",
	}

	app.logServices(context.Background(), &buf, colors)

	output := buf.String()

	expecteds := []string{
		fmt.Sprintf("%sINFO%s Services: \t%s%d%s\n", colors.Green, colors.Reset, colors.Blue, len(app.configured.Services), colors.Reset),
	}

	for _, dep := range app.configured.Services {
		stateColor := colors.Blue
		state := "RUNNING"
		if _, err := dep.State(context.Background()); err != nil {
			stateColor = colors.Red
			state = "ERROR"
		}

		expected := fmt.Sprintf("%sINFO%s    🥡 %s[ %s ] %s%s\n", colors.Green, colors.Reset, stateColor, strings.ToUpper(state), dep.String(), colors.Reset)
		expecteds = append(expecteds, expected)
	}

	for _, expected := range expecteds {
		require.Contains(t, output, expected)
	}
}

func TestServiceContextProviders(t *testing.T) {
	t.Run("no-provider", func(t *testing.T) {
		app := New()
		require.Equal(t, context.Background(), app.servicesStartupCtx())
		require.Equal(t, context.Background(), app.servicesShutdownCtx())
	})

	t.Run("with-provider", func(t *testing.T) {
		ctx := context.TODO()
		app := New(Config{
			ServicesStartupContextProvider: func() context.Context {
				return ctx
			},
			ServicesShutdownContextProvider: func() context.Context {
				return ctx
			},
		})

		require.Equal(t, ctx, app.servicesStartupCtx())
		require.Equal(t, ctx, app.servicesShutdownCtx())
	})
}
