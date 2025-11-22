package fiber

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
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
// Iterates over all the started services in reverse order and tries to terminate them,
// returning an error if any error occurs.
func (app *App) shutdownServices(ctx context.Context) error {
	if app.state.ServicesLen() == 0 {
		return nil
	}

	var errs []error
	for _, srv := range app.state.Services() {
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

		// Remove the service from the State
		app.state.deleteService(srv)
	}
	return errors.Join(errs...)
}

// logServices logs information about services
func (app *App) logServices(ctx context.Context, out io.Writer, colors *Colors) {
	if !app.hasConfiguredServices() {
		return
	}

	scheme := colors
	if scheme == nil {
		scheme = &DefaultColors
	}

	fmt.Fprintf(out,
		"%sINFO%s Services: \t%s%d%s\n",
		scheme.Green, scheme.Reset, scheme.Blue, app.state.ServicesLen(), scheme.Reset)
	for _, srv := range app.state.Services() {
		var state string
		var stateColor string
		state, err := srv.State(ctx)
		if err != nil {
			state = errString
			stateColor = scheme.Red
		} else {
			stateColor = scheme.Blue
			state = strings.ToUpper(state)
		}
		fmt.Fprintf(out, "%sINFO%s    🥡 %s[ %s ] %s%s\n", scheme.Green, scheme.Reset, stateColor, strings.ToUpper(state), srv.String(), scheme.Reset)
	}
}
