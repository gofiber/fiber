//go:build !dev

package fiber

import (
	"context"
	"io"
)

// DevTimeDependency is an interface that defines the methods for a development-time dependency.
type DevTimeDependency interface {
	Start(ctx context.Context) error
	String() string
	Terminate(ctx context.Context) error
}

// hasDevTimeDependencies always returns false in production builds
func (*App) hasDevTimeDependencies() bool {
	return false
}

// startDevTimeDependencies is a no-op in production builds
func (*App) startDevTimeDependencies(_ context.Context) error {
	return nil
}

// shutdownDevTimeDependencies is a no-op in production builds
func (*App) shutdownDevTimeDependencies(_ context.Context) error {
	return nil
}

// logDevTimeDependencies is a no-op in production builds
func (*App) logDevTimeDependencies(_ io.Writer, _ Colors) {}
