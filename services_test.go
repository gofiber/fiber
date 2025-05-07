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
	testStartServicesFn := func(t *testing.T, services []Service, wantErr bool) {
		t.Helper()

		app := &App{
			configured: Config{
				Services: services,
			},
		}

		err := app.startServices(context.Background())
		if wantErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}

	t.Run("no-services", func(t *testing.T) {
		testStartServicesFn(t, []Service{}, false)
	})

	t.Run("successful-start", func(t *testing.T) {
		testStartServicesFn(
			t,
			[]Service{
				&mockService{name: "dep1"},
				&mockService{name: "dep2"},
			},
			false,
		)
	})

	t.Run("failed-start", func(t *testing.T) {
		testStartServicesFn(
			t,
			[]Service{
				&mockService{name: "dep1", startError: errors.New("start error")},
				&mockService{name: "dep2"},
			},
			true,
		)
	})
}

func TestShutdownServices(t *testing.T) {
	testShutdownServicesFn := func(t *testing.T, services []Service, wantErr bool) {
		t.Helper()

		app := &App{
			configured: Config{
				Services: services,
			},
		}

		err := app.shutdownServices(context.Background())
		if wantErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}

	t.Run("no-services", func(t *testing.T) {
		testShutdownServicesFn(t, []Service{}, false)
	})

	t.Run("successful-shutdown", func(t *testing.T) {
		testShutdownServicesFn(t, []Service{&mockService{name: "dep1"}, &mockService{name: "dep2"}}, false)
	})

	t.Run("failed-shutdown", func(t *testing.T) {
		testShutdownServicesFn(
			t,
			[]Service{
				&mockService{name: "dep1", terminateError: errors.New("terminate error")},
				&mockService{name: "dep2"},
			},
			true,
		)
	})
}

func TestStartServicesWithContextAlreadyCanceled(t *testing.T) {
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
	require.Contains(t, err.Error(), "context canceled while starting services")
}

func TestServicesStartWithContextCancellation(t *testing.T) {
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

	// Start services with canceled context
	err := app.startServices(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestServicesTerminateWithContextCancellation(t *testing.T) {
	// Create a service that takes some time to terminate
	slowDep := &mockService{name: "slow-dep", terminateDelay: 200 * time.Millisecond}

	app := &App{
		configured: Config{
			Services: []Service{slowDep},
		},
	}

	err := app.startServices(context.Background())
	require.NoError(t, err)

	// Create a new context for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Shutdown services with canceled context
	err = app.shutdownServices(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestShutdownServicesWithContextAlreadyCanceled(t *testing.T) {
	app := &App{
		configured: Config{
			Services: []Service{
				&mockService{name: "dep1"},
			},
		},
	}

	err := app.startServices(context.Background())
	require.NoError(t, err)

	// Create a context that is already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = app.shutdownServices(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	require.Contains(t, err.Error(), "service dep1 terminate")
}

func TestServicesTerminate_ContextCanceledOrDeadlineExceeded(t *testing.T) {
	testFn := func(t *testing.T, terminateErr error, wantErr error) {
		t.Helper()

		app := &App{
			configured: Config{
				Services: []Service{
					&mockService{name: "dep1", terminateError: terminateErr},
					&mockService{name: "dep2"}, // Should still be called
				},
			},
		}

		err := app.shutdownServices(context.Background())
		require.Error(t, err)
		require.ErrorIs(t, err, wantErr)
		require.Contains(t, err.Error(), "service dep1 terminate")
	}

	t.Run("context-canceled", func(t *testing.T) {
		testFn(t, context.Canceled, context.Canceled)
	})

	t.Run("deadline-exceeded", func(t *testing.T) {
		testFn(t, context.DeadlineExceeded, context.DeadlineExceeded)
	})
}

func TestMultipleDependenciesStartErrorHandling(t *testing.T) {
	deps := []Service{
		&mockService{name: "dep1", startError: errors.New("start error 1")},
		&mockService{name: "dep2", startError: errors.New("start error 2")},
		&mockService{name: "dep3"},
	}

	app := &App{
		configured: Config{
			Services: deps,
		},
	}

	// Test start errors
	err := app.startServices(context.Background())
	require.Error(t, err)

	// Verify error message contains both error messages
	errMsg := err.Error()
	require.Contains(t, errMsg, "start error 1")
	require.Contains(t, errMsg, "start error 2")
}

func TestMultipleDependenciesTerminateErrorHandling(t *testing.T) {
	deps := []Service{
		&mockService{name: "dep1", terminateError: errors.New("terminate error 1")},
		&mockService{name: "dep2", terminateError: errors.New("terminate error 2")},
		&mockService{name: "dep3"},
	}

	app := &App{
		configured: Config{
			Services: deps,
		},
	}

	err := app.shutdownServices(context.Background())
	require.Error(t, err)

	// Verify error message contains both error messages
	errMsg := err.Error()
	require.Contains(t, errMsg, "terminate error 1")
	require.Contains(t, errMsg, "terminate error 2")
}

func TestLogServices(t *testing.T) {
	// Service with successful State
	runningService := &mockService{name: "running", started: true}
	// Service with State error
	errorService := &mockService{name: "error", stateError: errors.New("state error")}

	app := &App{
		configured: Config{
			Services: []Service{runningService, errorService},
		},
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
