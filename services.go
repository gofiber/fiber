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

// hasServices Checks if there are any services for the current application.
func (app *App) hasServices() bool {
	return len(app.configured.Services) > 0
}

// servicesStartupCtx Returns the context for the services startup.
// If the ServicesStartupCtx is not set, it returns a new background context.
func (app *App) servicesStartupCtx() context.Context {
	if app.config.ServicesStartupCtx != nil {
		return app.config.ServicesStartupCtx
	}

	return context.Background()
}

// servicesShutdownCtx Returns the context for the services shutdown.
// If the ServicesShutdownCtx is not set, it returns a new background context.
func (app *App) servicesShutdownCtx() context.Context {
	if app.config.ServicesShutdownCtx != nil {
		return app.config.ServicesShutdownCtx
	}

	return context.Background()
}

// startServices Handles the start process of services for the current application.
// Iterates over all services and tries to start them, returning an error if any error occurs.
func (app *App) startServices(ctx context.Context) error {
	if app.hasServices() {
		var errs []error
		for _, dep := range app.configured.Services {
			if err := ctx.Err(); err != nil {
				// Context is canceled, return an error the soonest possible, so that
				// the user can see the context cancellation error and act on it.
				return fmt.Errorf("context canceled while starting service %s: %w", dep.String(), err)
			}

			err := dep.Start(ctx)
			if err == nil {
				// mark the service as started
				app.startedServices = append(app.startedServices, dep)
				continue
			}

			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return fmt.Errorf("service %s start: %w", dep.String(), err)
			}

			errs = append(errs, fmt.Errorf("service %s start: %w", dep.String(), err))
		}
		return errors.Join(errs...)
	}
	return nil
}

// shutdownServices Handles the shutdown process of services for the current application.
// Iterates over all services and tries to terminate them, returning an error if any error occurs.
func (app *App) shutdownServices(ctx context.Context) error {
	if len(app.startedServices) > 0 {
		var errs []error
		for _, dep := range app.startedServices {
			if err := ctx.Err(); err != nil {
				// Context is canceled, do a best effort to terminate the services.
				errs = append(errs, fmt.Errorf("service %s terminate: %w", dep.String(), err))
				continue
			}

			err := dep.Terminate(ctx)
			if err != nil {
				// Best effort to terminate the services.
				errs = append(errs, fmt.Errorf("service %s terminate: %w", dep.String(), err))
			}
		}
		return errors.Join(errs...)
	}
	return nil
}

// logServices logs information about services
func (app *App) logServices(ctx context.Context, out io.Writer, colors Colors) {
	if app.hasServices() {
		fmt.Fprintf(out,
			"%sINFO%s Services: \t%s%d%s\n",
			colors.Green, colors.Reset, colors.Blue, len(app.configured.Services), colors.Reset)
		for _, dep := range app.configured.Services {
			var state string
			var stateColor string
			state, err := dep.State(ctx)
			if err != nil {
				state = "ERROR"
				stateColor = colors.Red
			} else {
				stateColor = colors.Blue
				state = strings.ToUpper(state)
			}
			fmt.Fprintf(out, "%sINFO%s    🥡 %s[ %s ] %s%s\n", colors.Green, colors.Reset, stateColor, strings.ToUpper(state), dep.String(), colors.Reset)
		}
	}
}
