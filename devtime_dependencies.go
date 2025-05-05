package fiber

import (
	"context"
	"errors"
	"fmt"
)

// DevTimeDependency is an interface that defines the methods for a development-time dependency.
type DevTimeDependency interface {
	// Start starts the dependency, returning an error if it fails.
	Start(ctx context.Context) error

	// String returns a string representation of the dependency.
	// It is used to print the dependency in the startup message.
	String() string

	// Terminate terminates the dependency, returning an error if it fails.
	Terminate(ctx context.Context) error
}

// hasDevTimeDependencies Checks if there are any dependency for the current application.
func (app *App) hasDevTimeDependencies() bool {
	return len(app.configured.DevTimeDependencies) > 0
}

// startDevTimeDependencies Handles the start process of dependencies for the current application.
// Iterates over all dependencies and tries to start them, returning an error if any error occurs.
func (app *App) startDevTimeDependencies(ctx context.Context) error {
	if app.hasDevTimeDependencies() {
		var errs []error
		for _, dep := range app.configured.DevTimeDependencies {
			if err := ctx.Err(); err != nil {
				// Context is canceled, return an error the soonest possible, so that
				// the user can see the context cancellation error and act on it.
				return fmt.Errorf("context canceled while starting dependencies: %w", err)
			}

			err := dep.Start(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("dependency %s start: %w", dep.String(), err)
				}
				errs = append(errs, fmt.Errorf("start dependency %s: %w", dep.String(), err))
			}
		}
		return errors.Join(errs...)
	}
	return nil
}

// shutdownDevTimeDependencies Handles the shutdown process of dependencies for the current application.
// Iterates over all dependencies and tries to terminate them, returning an error if any error occurs.
func (app *App) shutdownDevTimeDependencies(ctx context.Context) error {
	if app.hasDevTimeDependencies() {
		var errs []error
		for _, dep := range app.configured.DevTimeDependencies {
			if err := ctx.Err(); err != nil {
				// Context is canceled, do a best effort to terminate the dependencies.
				errs = append(errs, fmt.Errorf("context canceled: %w", err))
				continue
			}

			err := dep.Terminate(ctx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					return fmt.Errorf("dependency %s terminate: %w", dep.String(), err)
				}
				errs = append(errs, fmt.Errorf("terminate dependency %s: %w", dep.String(), err))
			}
		}
		return errors.Join(errs...)
	}
	return nil
}
