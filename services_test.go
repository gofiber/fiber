package fiber

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mockService implements Service interface for testing
type mockService struct {
	startError     error
	terminateError error
	name           string
	started        bool
	terminated     bool
}

func (m *mockService) Start(ctx context.Context) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context canceled: %w", ctx.Err())
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

	if m.started {
		return "running", nil
	}

	if m.terminated {
		return "stopped", nil
	}

	return "unknown", nil
}

func (m *mockService) Terminate(ctx context.Context) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context canceled: %w", ctx.Err())
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

func TestServicesStartWithContextCancellation(t *testing.T) {
	// Create a service that takes some time to start
	slowDep := &mockService{name: "slow-dep"}

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
	slowDep := &mockService{name: "slow-dep"}

	app := &App{
		configured: Config{
			Services: []Service{slowDep},
		},
	}

	// Start services with canceled context
	err := app.startServices(context.Background())
	require.NoError(t, err)

	// Create a new context for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Shutdown services with canceled context
	err = app.shutdownServices(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
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
