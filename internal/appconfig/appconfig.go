// Package appconfig hands request hot paths the few Config bits they read on
// every call, without the copy App.Config returns by value.
//
// Config is over 600 bytes and Config() returns it whole, so a caller that
// wants one boolean out of it pays for all of it — measured at 25ns against
// 2ns for the App call itself, which is a third of a header extraction.
// Package fiber installs a reader that takes the bits straight off the app it
// owns. It cannot be done the other way around: config is unexported, and this
// package importing fiber would be a cycle.
package appconfig

import (
	"sync"
)

// Hot is the subset of fiber.Config that request hot paths read.
type Hot struct {
	Immutable                bool
	DisableHeaderNormalizing bool
	UnescapePath             bool
}

var (
	// read answers Of. Until package fiber installs the real one it reports
	// the zero Config, which no caller here can reach: each holds a fiber.Ctx
	// and so has imported fiber, whose init is ordered before them.
	read = func(any) Hot { return Hot{} }

	// readOnce makes "installed once, before main" a property of the code
	// rather than a promise in a comment. Without it any package in the
	// module could retarget what Immutable means for the whole process, and
	// could do it while requests are being served.
	readOnce sync.Once
)

// SetReader installs the function Of answers with. The first call wins and
// later calls are ignored, so the reader cannot be replaced once requests are
// running; a nil reader is refused outright.
//
// Package fiber calls this from its init. Nothing else should call it.
func SetReader(fn func(any) Hot) {
	if fn == nil {
		return
	}
	readOnce.Do(func() { read = fn })
}

// Of returns the Hot bits of app, which must be a *fiber.App.
func Of(app any) Hot {
	return read(app)
}
