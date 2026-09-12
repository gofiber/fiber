package fiber

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"

	utilsstrings "github.com/gofiber/utils/v2/strings"
)

// Service is an interface that defines the methods for a service.
type Service interface {
	// Start starts the service, returning an error if it fails.
	Start(ctx context.Context) error

	// String returns a string representation of the service.
	// It is used to print a human-readable name of the service in the startup message.
	String() string

	// State returns the current state of the service.
	State(ctx context.Context) (string, error)

	// Terminate terminates the service, returning an error if it fails.
	Terminate(ctx context.Context) error
}

// hasConfiguredServices Checks if there are any services for the current application.
func (app *App) hasConfiguredServices() bool {
	return len(app.configured.Services) > 0
}

func (app *App) validateConfiguredServices() error {
	return validateServicesSlice(app.configured.Services)
}

func validateServicesSlice(services []Service) error {
	seen := make(map[string]int, len(services))
	for idx, srv := range services {
		if srv == nil {
			return fmt.Errorf("fiber: service at index %d is nil", idx)
		}
		// Services are keyed by name in the State, so two sharing one would overwrite each other.
		name := srv.String()
		if first, dup := seen[name]; dup {
			return fmt.Errorf("fiber: duplicate service name %q: services at index %d and %d share it, but service names must be unique", name, first, idx)
		}
		seen[name] = idx
	}
	return nil
}

// initServices If the app is configured to use services, this function registers
// a post shutdown hook to shutdown them after the server is closed.
// This function panics if there is an error starting the services.
func (app *App) initServices() {
	if !app.hasConfiguredServices() {
		return
	}

	if err := app.startServices(app.servicesStartupCtx()); err != nil {
		panic(err)
	}
}

// servicesStartupCtx Returns the context for the services startup.
// If the ServicesStartupContextProvider is not set, it returns a new background context.
func (app *App) servicesStartupCtx() context.Context {
	if app.configured.ServicesStartupContextProvider != nil {
		return app.configured.ServicesStartupContextProvider()
	}

	return context.Background()
}

// servicesShutdownCtx Returns the context for the services shutdown.
// If the ServicesShutdownContextProvider is not set, it returns a new background context.
func (app *App) servicesShutdownCtx() context.Context {
	if app.configured.ServicesShutdownContextProvider != nil {
		return app.configured.ServicesShutdownContextProvider()
	}

	return context.Background()
}

// startServices Handles the start process of services for the current application.
// Iterates over all configured services and tries to start them, returning an error if any error occurs.
func (app *App) startServices(ctx context.Context) error {
	if !app.hasConfiguredServices() {
		return nil
	}

	// Validate the whole slice first, so nothing is started when an entry is invalid.
	if err := validateServicesSlice(app.configured.Services); err != nil {
		return err
	}

	var errs []error
	for _, srv := range app.configured.Services {
		if err := ctx.Err(); err != nil {
			// Context is canceled, return an error the soonest possible, so that
			// the user can see the context cancellation error and act on it.
			return fmt.Errorf("context canceled while starting service %s: %w", srv.String(), err)
		}

		err := srv.Start(ctx)
		if err == nil {
			// mark the service as started
			app.state.setService(srv)
			continue
		}

		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("service %s start: %w", srv.String(), err)
		}

		errs = append(errs, fmt.Errorf("service %s start: %w", srv.String(), err))
	}
	return errors.Join(errs...)
}

// shutdownServices Handles the shutdown process of services for the current application.
// Iterates over all the started services in reverse start order and tries to terminate them,
// returning an error if any error occurs.
func (app *App) shutdownServices(ctx context.Context) error {
	services := app.state.takeStartedServices()
	if len(services) == 0 {
		return nil
	}

	var errs []error
	terminated := make([]bool, len(services))
	for i, srv := range slices.Backward(services) {
		if err := ctx.Err(); err != nil {
			// Context is canceled, do a best effort to terminate the services.
			errs = append(errs, fmt.Errorf("service %s terminate: %w", srv.String(), err))
			continue
		}

		err := srv.Terminate(ctx)
		if err != nil {
			// Best effort to terminate the services.
			errs = append(errs, fmt.Errorf("service %s terminate: %w", srv.String(), err))
			continue
		}

		terminated[i] = true
		// Remove the service from the State
		app.state.deleteService(srv)
	}

	// One that would not terminate goes back, so a later shutdown retries it.
	var remaining []Service
	for i, srv := range services {
		if !terminated[i] {
			remaining = append(remaining, srv)
		}
	}
	app.state.restoreStartedServices(remaining)

	return errors.Join(errs...)
}

// logServices logs information about services and returns an error
// if any configured service is nil.
func (app *App) logServices(ctx context.Context, out io.Writer, colors *Colors) error {
	if !app.hasConfiguredServices() {
		return nil
	}

	scheme := colors
	if scheme == nil {
		scheme = &DefaultColors
	}

	fmt.Fprintf(out,
		"%sINFO%s Services: \t%s%d%s\n",
		scheme.Green, scheme.Reset, scheme.Blue, app.state.ServicesLen(), scheme.Reset)
	for key, srv := range app.state.Services() {
		if srv == nil {
			return fmt.Errorf("fiber: service %q is nil", key)
		}
		var state string
		var stateColor string
		state, err := srv.State(ctx)
		if err != nil {
			state = errString
			stateColor = scheme.Red
		} else {
			stateColor = scheme.Blue
		}
		fmt.Fprintf(out, "%sINFO%s    🧩 %s[ %s ] %s%s\n", scheme.Green, scheme.Reset, stateColor, utilsstrings.ToUpper(state), srv.String(), scheme.Reset)
	}
	return nil
}
