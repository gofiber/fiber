package fiber

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mockDependency implements DevTimeDependency interface for testing
type mockDependency struct {
	startError     error
	terminateError error
	name           string
	started        bool
	terminated     bool
}

func (m *mockDependency) Start(_ context.Context) error {
	m.started = true
	return m.startError
}

func (m *mockDependency) String() string {
	return m.name
}

func (m *mockDependency) Terminate(_ context.Context) error {
	m.terminated = true
	return m.terminateError
}

func TestHasDevTimeDependencies(t *testing.T) {
	testHasDevTimeDependenciesFn := func(t *testing.T, app *App, expected bool) {
		t.Helper()

		result := app.hasDevTimeDependencies()
		require.Equal(t, expected, result)
	}

	t.Run("no-dependencies", func(t *testing.T) {
		testHasDevTimeDependenciesFn(t, &App{configured: Config{}}, false)
	})

	t.Run("has-dependencies", func(t *testing.T) {
		testHasDevTimeDependenciesFn(t, &App{configured: Config{DevTimeDependencies: []DevTimeDependency{&mockDependency{name: "test-dep"}}}}, true)
	})
}

func TestStartDevTimeDependencies(t *testing.T) {
	testStartDevTimeDependenciesFn := func(t *testing.T, dependencies []DevTimeDependency, wantErr bool) {
		t.Helper()

		app := &App{
			configured: Config{
				DevTimeDependencies: dependencies,
			},
		}

		err := app.startDevTimeDependencies(context.Background())
		if wantErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}

	t.Run("no-dependencies", func(t *testing.T) {
		testStartDevTimeDependenciesFn(t, []DevTimeDependency{}, false)
	})

	t.Run("successful-start", func(t *testing.T) {
		testStartDevTimeDependenciesFn(
			t,
			[]DevTimeDependency{
				&mockDependency{name: "dep1"},
				&mockDependency{name: "dep2"},
			},
			false,
		)
	})

	t.Run("failed-start", func(t *testing.T) {
		testStartDevTimeDependenciesFn(
			t,
			[]DevTimeDependency{
				&mockDependency{name: "dep1", startError: errors.New("start error")},
				&mockDependency{name: "dep2"},
			},
			true,
		)
	})
}

func TestShutdownDevTimeDependencies(t *testing.T) {
	testShutdownDevTimeDependenciesFn := func(t *testing.T, dependencies []DevTimeDependency, wantErr bool) {
		t.Helper()

		app := &App{
			configured: Config{
				DevTimeDependencies: dependencies,
			},
		}

		err := app.shutdownDevTimeDependencies(context.Background())
		if wantErr {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
		}
	}

	t.Run("no-dependencies", func(t *testing.T) {
		testShutdownDevTimeDependenciesFn(t, []DevTimeDependency{}, false)
	})

	t.Run("successful-shutdown", func(t *testing.T) {
		testShutdownDevTimeDependenciesFn(t, []DevTimeDependency{&mockDependency{name: "dep1"}, &mockDependency{name: "dep2"}}, false)
	})

	t.Run("failed-shutdown", func(t *testing.T) {
		testShutdownDevTimeDependenciesFn(
			t,
			[]DevTimeDependency{
				&mockDependency{name: "dep1", terminateError: errors.New("terminate error")},
				&mockDependency{name: "dep2"},
			},
			true,
		)
	})
}

func TestDevTimeDependenciesStartWithContextCancellation(t *testing.T) {
	// Create a dependency that takes some time to start
	slowDep := &mockDependency{name: "slow-dep"}

	app := &App{
		configured: Config{
			DevTimeDependencies: []DevTimeDependency{slowDep},
		},
	}

	// Create a context that will be canceled immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Start dependencies with canceled context
	err := app.startDevTimeDependencies(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestDevTimeDependenciesTerminateWithContextCancellation(t *testing.T) {
	// Create a dependency that takes some time to terminate
	slowDep := &mockDependency{name: "slow-dep"}

	app := &App{
		configured: Config{
			DevTimeDependencies: []DevTimeDependency{slowDep},
		},
	}

	// Start dependencies with canceled context
	err := app.startDevTimeDependencies(context.Background())
	require.NoError(t, err)

	// Create a new context for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Shutdown dependencies with canceled context
	err = app.shutdownDevTimeDependencies(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestMultipleDependenciesStartErrorHandling(t *testing.T) {
	deps := []DevTimeDependency{
		&mockDependency{name: "dep1", startError: errors.New("start error 1")},
		&mockDependency{name: "dep2", startError: errors.New("start error 2")},
		&mockDependency{name: "dep3"},
	}

	app := &App{
		configured: Config{
			DevTimeDependencies: deps,
		},
	}

	// Test start errors
	err := app.startDevTimeDependencies(context.Background())
	require.Error(t, err)

	// Verify error message contains both error messages
	errMsg := err.Error()
	require.Contains(t, errMsg, "start error 1")
	require.Contains(t, errMsg, "start error 2")
}

func TestMultipleDependenciesTerminateErrorHandling(t *testing.T) {
	deps := []DevTimeDependency{
		&mockDependency{name: "dep1", terminateError: errors.New("terminate error 1")},
		&mockDependency{name: "dep2", terminateError: errors.New("terminate error 2")},
		&mockDependency{name: "dep3"},
	}

	app := &App{
		configured: Config{
			DevTimeDependencies: deps,
		},
	}

	err := app.shutdownDevTimeDependencies(context.Background())
	require.Error(t, err)

	// Verify error message contains both error messages
	errMsg := err.Error()
	require.Contains(t, errMsg, "terminate error 1")
	require.Contains(t, errMsg, "terminate error 2")
}
