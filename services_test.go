package fiber

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	stdlog "log" //nolint:depguard // Test needs the concrete stdlib logger type to restore the previous writer.
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3/internal/storagetest"
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

var servicesTestLogOutputMu sync.Mutex

func withCapturedLogOutput(t *testing.T, writer io.Writer) {
	t.Helper()

	servicesTestLogOutputMu.Lock()
	cleanupRegistered := false
	defer func() {
		if !cleanupRegistered {
			servicesTestLogOutputMu.Unlock()
		}
	}()

	currentOutput := log.DefaultLogger[*stdlog.Logger]().Logger().Writer()
	t.Cleanup(func() {
		log.SetOutput(currentOutput)
		servicesTestLogOutputMu.Unlock()
	})
	cleanupRegistered = true

	log.SetOutput(writer)
}

// mockServiceWait fails if ctx is done at once, or before the delay has passed.
func mockServiceWait(ctx context.Context, delay time.Duration) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("context canceled: %w", ctx.Err())
	default:
	}

	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return fmt.Errorf("context canceled: %w", ctx.Err())
		case <-timer.C:
			// Continue after delay
		}
	}

	return nil
}

func (m *mockService) Start(ctx context.Context) error {
	if err := mockServiceWait(ctx, m.startDelay); err != nil {
		return err
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
	if err := mockServiceWait(ctx, m.terminateDelay); err != nil {
		return err
	}

	if m.terminateError != nil {
		m.terminated = false
		return m.terminateError
	}

	m.started = false
	m.terminated = true
	return nil
}

// rootMiscAServicesApp returns an App configured with services and an empty
// state, built from the struct so that nothing is started and no hooks are set.
func rootMiscAServicesApp(services ...Service) *App {
	return &App{configured: Config{Services: services}, state: newState()}
}

// rootMiscAStartedServicesApp is rootMiscAServicesApp for an app whose services
// have all been started already, in the given order, as shutdownServices and
// logServices expect to find them.
func rootMiscAStartedServicesApp(services ...Service) *App {
	app := rootMiscAServicesApp(services...)
	for _, srv := range services {
		app.state.setService(srv)
	}
	return app
}

func Test_HasConfiguredServices(t *testing.T) {
	t.Parallel()
	t.Run("no-services", func(t *testing.T) {
		t.Parallel()
		require.False(t, (&App{configured: Config{}}).hasConfiguredServices())
	})

	t.Run("has-services", func(t *testing.T) {
		t.Parallel()
		app := &App{configured: Config{Services: []Service{&mockService{name: "test-dep"}}}}
		require.True(t, app.hasConfiguredServices())
	})
}

func Test_InitServices(t *testing.T) {
	t.Parallel()
	t.Run("no-services", func(t *testing.T) {
		app := &App{configured: Config{}}
		require.NotPanics(t, app.initServices)
	})

	t.Run("start/success", func(t *testing.T) {
		// Initialize the app using the struct and defining the state and hooks manually,
		// because we are not checking the shutdown hooks in this test.
		app := rootMiscAServicesApp(&mockService{name: "dep1"}, &mockService{name: "dep2"})

		app.hooks = newHooks(app)

		require.NotPanics(t, app.initServices)
	})

	t.Run("start/error", func(t *testing.T) {
		// Initialize the app using the struct and defining the state and hooks manually,
		// because we are not checking the shutdown hooks in this test.
		app := rootMiscAServicesApp(
			&mockService{name: "dep1", startError: errors.New(startErrorMessage + " 1")},
			&mockService{name: "dep2", startError: errors.New(startErrorMessage + " 2")},
			&mockService{name: "dep3"},
		)

		app.hooks = newHooks(app)

		require.Panics(t, app.initServices)
	})

	// An app made with New has the shutdown hooks registered, which also checks that the app mutex does not deadlock.
	for _, tc := range []struct {
		storage  *storagetest.Failing // the shared storage of the app, if it has one
		name     string
		services []Service
		contains []string // what the post shutdown hooks log
		absent   []string
	}{
		{
			name: "shutdown-hooks/success", services: []Service{&mockService{name: "dep1"}},
			absent: []string{"failed to call post shutdown hook:"},
		},
		{
			name: "shutdown-hooks/error",
			services: []Service{
				&mockService{name: "dep1"},
				&mockService{name: "dep2", terminateError: errors.New(terminateErrorMessage + " 2")},
			},
			contains: []string{"failed to shutdown services: service dep2 terminate: terminate error 2"},
		},
		{
			name: "shutdown-hooks/close-shared-state", services: []Service{&mockService{name: "dep1"}},
			storage: &storagetest.Failing{}, absent: []string{"failed to close sharedState:"},
		},
		{
			name: "shutdown-hooks/close-shared-state-after-service-error",
			services: []Service{
				&mockService{name: "dep1"},
				&mockService{name: "dep2", terminateError: errors.New(terminateErrorMessage + " 2")},
			},
			storage: &storagetest.Failing{CloseErr: errors.New("close error")},
			contains: []string{
				"failed to shutdown services: service dep2 terminate: terminate error 2",
				"failed to close sharedState: close error",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := Config{Services: tc.services}
			if tc.storage != nil {
				cfg.SharedStorage = tc.storage
			}
			app := New(cfg)

			require.NotPanics(t, app.initServices)

			var buf strings.Builder
			withCapturedLogOutput(t, &buf)

			app.Hooks().executeOnPostShutdownHooks(nil)

			if tc.storage != nil {
				require.True(t, tc.storage.Closed())
			}
			rootMiscBRequireContains(t, buf.String(), tc.contains...)
			rootMiscBRequireNotContains(t, buf.String(), tc.absent...)
		})
	}
}

func Test_StartServices(t *testing.T) {
	t.Run("no-services", func(t *testing.T) {
		app := rootMiscAServicesApp()

		require.NoError(t, app.startServices(context.Background()))
		require.Zero(t, app.state.ServicesLen())
	})

	t.Run("successful-start", func(t *testing.T) {
		app := rootMiscAServicesApp(&mockService{name: "dep1"}, &mockService{name: "dep2"})

		require.NoError(t, app.startServices(context.Background()))
		require.Equal(t, 2, app.state.ServicesLen())
	})

	t.Run("failed-start", func(t *testing.T) {
		app := rootMiscAServicesApp(
			&mockService{name: "dep1", startError: errors.New(startErrorMessage + " 1")},
			&mockService{name: "dep2", startError: errors.New(startErrorMessage + " 2")},
			&mockService{name: "dep3"},
		)

		err := app.startServices(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), startErrorMessage+" 1")
		require.Contains(t, err.Error(), startErrorMessage+" 2")
		require.Equal(t, 1, app.state.ServicesLen())
	})

	t.Run("context", func(t *testing.T) {
		t.Run("already-canceled", func(t *testing.T) {
			app := rootMiscAServicesApp(&mockService{name: "dep1"})
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			require.ErrorIs(t, app.startServices(ctx), context.Canceled)
			require.Zero(t, app.state.ServicesLen())
		})

		t.Run("cancellation", func(t *testing.T) {
			// The service takes longer to start than the context lasts.
			app := rootMiscAServicesApp(&mockService{name: "slow-dep", startDelay: 200 * time.Millisecond})
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			require.ErrorIs(t, app.startServices(ctx), context.DeadlineExceeded)
			require.Zero(t, app.state.ServicesLen())
		})
	})
}

func Test_ShutdownServices(t *testing.T) {
	t.Run("no-services", func(t *testing.T) {
		app := rootMiscAServicesApp()

		require.NoError(t, app.shutdownServices(context.Background()))
		require.Zero(t, app.state.ServicesLen())
	})

	t.Run("successful-shutdown", func(t *testing.T) {
		app := rootMiscAStartedServicesApp(&mockService{name: "dep1"}, &mockService{name: "dep2"})

		require.NoError(t, app.shutdownServices(context.Background()))
		require.Zero(t, app.state.ServicesLen())
	})

	t.Run("failed-shutdown", func(t *testing.T) {
		srv1 := &mockService{name: "dep1", terminateError: errors.New(terminateErrorMessage + " 1")}
		srv2 := &mockService{name: "dep2", terminateError: errors.New(terminateErrorMessage + " 2")}
		srv3 := &mockService{name: "dep3"}
		app := rootMiscAStartedServicesApp(srv1, srv2, srv3)

		err := app.shutdownServices(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), terminateErrorMessage+" 1")
		require.Contains(t, err.Error(), terminateErrorMessage+" 2")
		require.Equal(t, 2, app.state.ServicesLen()) // 2 services failed to terminate
	})

	t.Run("context", func(t *testing.T) {
		t.Run("already-canceled", func(t *testing.T) {
			app := rootMiscAStartedServicesApp(&mockService{name: "dep1"})
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			err := app.shutdownServices(ctx)
			require.Error(t, err)
			require.ErrorIs(t, err, context.Canceled)
			require.Contains(t, err.Error(), "service dep1 terminate")
			require.Equal(t, 1, app.state.ServicesLen())
		})

		t.Run("cancellation", func(t *testing.T) {
			// The service takes longer to terminate than the context lasts.
			app := rootMiscAStartedServicesApp(&mockService{name: "slow-dep", terminateDelay: 200 * time.Millisecond})
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			require.ErrorIs(t, app.shutdownServices(ctx), context.DeadlineExceeded)
			require.Equal(t, 1, app.state.ServicesLen())
		})
	})
}

func Test_LogServices(t *testing.T) {
	t.Parallel()
	runningService := &mockService{name: "running", started: true}
	errorService := &mockService{name: "error", stateError: errors.New("state error")}
	app := rootMiscAStartedServicesApp(runningService, errorService)

	var buf bytes.Buffer
	colors := Colors{Green: "\033[32m", Reset: "\033[0m", Blue: "\033[34m", Red: "\033[31m"}
	require.NoError(t, app.logServices(context.Background(), &buf, &colors))

	output := buf.String()

	for _, srv := range app.state.Services() {
		stateColor := colors.Blue
		state := "RUNNING"
		if _, err := srv.State(context.Background()); err != nil {
			stateColor = colors.Red
			state = "ERROR"
		}

		expected := fmt.Sprintf("%sINFO%s    🧩 %s[ %s ] %s%s\n", colors.Green, colors.Reset, stateColor, strings.ToUpper(state), srv.String(), colors.Reset)
		require.Contains(t, output, expected)
	}
}

func Test_NewConfiguredServicesNil(t *testing.T) {
	t.Parallel()
	require.PanicsWithError(t, "fiber: service at index 0 is nil", func() { New(Config{Services: []Service{nil}}) })
}

func Test_ServiceContextProviders(t *testing.T) {
	t.Run("no-provider", func(t *testing.T) {
		app := &App{configured: Config{}, state: newState()}
		require.Equal(t, context.Background(), app.servicesStartupCtx())
		require.Equal(t, context.Background(), app.servicesShutdownCtx())
	})

	t.Run("with-provider", func(t *testing.T) {
		ctx := context.TODO()
		app := &App{
			configured: Config{
				ServicesStartupContextProvider:  func() context.Context { return ctx },
				ServicesShutdownContextProvider: func() context.Context { return ctx },
			},
			state: newState(),
		}

		require.Equal(t, ctx, app.servicesStartupCtx())
		require.Equal(t, ctx, app.servicesShutdownCtx())
	})
}

// serviceBenchSets runs run as the sub-benchmarks no-services, single-service and
// multiple-services, and, given delayed, as multiple-services-with-delays on dep1
// to dep3 built by delayed with a delay of 1 to 3ms.
func serviceBenchSets(b *testing.B, run func(*testing.B, []Service), delayed func(string, time.Duration) Service) {
	b.Helper()
	b.Run("no-services", func(b *testing.B) { run(b, []Service{}) })
	b.Run("single-service", func(b *testing.B) { run(b, []Service{&mockService{name: "dep1"}}) })
	b.Run("multiple-services", func(b *testing.B) {
		run(b, []Service{&mockService{name: "dep1"}, &mockService{name: "dep2"}, &mockService{name: "dep3"}})
	})
	if delayed != nil {
		b.Run("multiple-services-with-delays", func(b *testing.B) {
			run(b, []Service{
				delayed("dep1", 1*time.Millisecond),
				delayed("dep2", 2*time.Millisecond),
				delayed("dep3", 3*time.Millisecond),
			})
		})
	}
}

func Benchmark_StartServices(b *testing.B) {
	benchmarkFn := func(b *testing.B, services []Service) {
		b.Helper()

		for b.Loop() {
			app := New(Config{Services: services})

			ctx := context.Background()
			if err := app.startServices(ctx); err != nil {
				b.Fatal("Expected no error but got", err)
			}
		}
	}

	serviceBenchSets(b, benchmarkFn, func(name string, d time.Duration) Service {
		return &mockService{name: name, startDelay: d}
	})
}

func Benchmark_ShutdownServices(b *testing.B) {
	benchmarkFn := func(b *testing.B, services []Service) {
		b.Helper()

		for b.Loop() {
			app := New(Config{Services: services})

			ctx := context.Background()
			if err := app.shutdownServices(ctx); err != nil {
				b.Fatal("Expected no error but got", err)
			}
		}
	}

	serviceBenchSets(b, benchmarkFn, func(name string, d time.Duration) Service {
		return &mockService{name: name, terminateDelay: d}
	})
}

func Benchmark_StartServices_withContextCancellation(b *testing.B) {
	benchmarkFn := func(b *testing.B, services []Service, timeout time.Duration) {
		b.Helper()

		for b.Loop() {
			app := New(Config{Services: services})

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
		benchmarkFn(b, []Service{&mockService{name: "dep1", startDelay: 100 * time.Millisecond}}, 10*time.Millisecond)
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

		for b.Loop() {
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

		for b.Loop() {
			app := New(Config{Services: services})

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

		for b.Loop() {
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

		b.ReportAllocs()

		var err error
		for b.Loop() {
			app := New(Config{Services: services})

			ctx := context.Background()
			err = app.startServices(ctx)
			if err != nil {
				continue
			}
			err = app.shutdownServices(ctx)
		}
		require.NoError(b, err)
	}

	serviceBenchSets(b, benchmarkFn, nil)
}

// orderedService records the order in which services are terminated.
type orderedService struct {
	terminated *[]string
	mockService
}

func (s *orderedService) Terminate(ctx context.Context) error {
	*s.terminated = append(*s.terminated, s.name)
	return s.mockService.Terminate(ctx)
}

func Test_ShutdownServices_ReverseStartOrder(t *testing.T) {
	t.Parallel()
	const count = 8
	var terminated []string
	services := make([]Service, 0, count)
	expected := make([]string, 0, count)
	for i := range count {
		name := fmt.Sprintf("dep%d", i)
		services = append(services, &orderedService{mockService: mockService{name: name}, terminated: &terminated})
		expected = append([]string{name}, expected...)
	}

	app := rootMiscAServicesApp(services...)

	require.NoError(t, app.startServices(context.Background()))
	require.Equal(t, count, app.state.ServicesLen())

	require.NoError(t, app.shutdownServices(context.Background()))
	require.Equal(t, expected, terminated)
	require.Zero(t, app.state.ServicesLen())
}

// uncomparableService is a Service stored by value whose slice field makes its
// dynamic type uncomparable; shutting one down must not panic.
type uncomparableService struct {
	name string
	tags []string
}

func (uncomparableService) Start(context.Context) error { return nil }

func (s uncomparableService) String() string { return s.name }

func (uncomparableService) State(context.Context) (string, error) { return "running", nil }

func (uncomparableService) Terminate(context.Context) error { return nil }

func Test_ShutdownServices_UncomparableService(t *testing.T) {
	t.Parallel()
	services := []Service{
		uncomparableService{name: "first", tags: []string{"a"}},
		uncomparableService{name: "second", tags: []string{"b"}},
	}
	app := rootMiscAServicesApp(services...)

	require.NoError(t, app.startServices(context.Background()))
	require.Equal(t, 2, app.state.ServicesLen())

	require.NotPanics(t, func() { require.NoError(t, app.shutdownServices(context.Background())) })
	require.Zero(t, app.state.ServicesLen())
}

func Test_ShutdownServices_StartedTwiceTerminatesOnce(t *testing.T) {
	t.Parallel()

	// App.init already starts configured services, so a second startServices
	// call registers each of them again, as the shutdown benchmarks do.
	var terminated []string
	services := []Service{
		&orderedService{mockService: mockService{name: "dep1"}, terminated: &terminated},
		&orderedService{mockService: mockService{name: "dep2"}, terminated: &terminated},
	}
	app := rootMiscAServicesApp(services...)

	require.NoError(t, app.startServices(context.Background()))
	require.NoError(t, app.startServices(context.Background()))
	require.Equal(t, 2, app.state.ServicesLen())

	require.NoError(t, app.shutdownServices(context.Background()))
	require.Equal(t, []string{"dep2", "dep1"}, terminated)
	require.Zero(t, app.state.ServicesLen())
}

func Test_StartServices_DuplicateNames(t *testing.T) {
	t.Parallel()
	t.Run("startServices fails before starting anything", func(t *testing.T) {
		t.Parallel()
		first := &mockService{name: "dup"}
		second := &mockService{name: "dup"}
		app := rootMiscAServicesApp(first, &mockService{name: "other"}, second)

		err := app.startServices(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), `duplicate service name "dup"`)
		require.False(t, first.started)
		require.False(t, second.started)
		require.Zero(t, app.state.ServicesLen())
	})

	t.Run("New panics", func(t *testing.T) {
		t.Parallel()
		require.PanicsWithError(t, `fiber: duplicate service name "dup": services at index 0 and 2 share it, but service names must be unique`, func() {
			New(Config{Services: []Service{&mockService{name: "dup"}, &mockService{name: "other"}, &mockService{name: "dup"}}})
		})
	})
}

// countingService records how many times it was terminated.
type countingService struct {
	terminations *atomic.Int32
	mockService
}

func (s *countingService) Terminate(ctx context.Context) error {
	s.terminations.Add(1)
	return s.mockService.Terminate(ctx)
}

func Test_ShutdownServices_ConcurrentCallersTerminateOnce(t *testing.T) {
	t.Parallel()
	const (
		count   = 3
		callers = 4
	)
	counters := make([]*atomic.Int32, count)
	services := make([]Service, 0, count)
	for i := range count {
		counters[i] = &atomic.Int32{}
		services = append(services, &countingService{
			mockService:  mockService{name: fmt.Sprintf("dep%d", i), terminateDelay: 10 * time.Millisecond},
			terminations: counters[i],
		})
	}

	app := rootMiscAServicesApp(services...)
	require.NoError(t, app.startServices(context.Background()))

	var wg sync.WaitGroup
	errs := make([]error, callers)
	for i := range callers {
		wg.Go(func() { errs[i] = app.shutdownServices(context.Background()) })
	}
	wg.Wait()

	for _, err := range errs {
		require.NoError(t, err)
	}
	for i, counter := range counters {
		require.Equal(t, int32(1), counter.Load(), "dep%d was terminated %d times", i, counter.Load())
	}
	require.Zero(t, app.state.ServicesLen())
}
