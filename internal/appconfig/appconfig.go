// Package appconfig hands request hot paths the few Config bits they read on
// every call, without the copy App.Config returns by value.
//
// Config is over 600 bytes, so reading one boolean out of it cost 25ns against
// 2ns for the App call itself. Package fiber installs a reader that takes the
// bits off the app directly; it cannot be done from here, because config is
// unexported and importing fiber would be a cycle.
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
	// read answers Of. Every caller holds a fiber.Ctx and so has imported
	// fiber, whose init is ordered before them, and none can see this one.
	read = func(any) Hot { return Hot{} }

	// readOnce makes "installed once, before main" true of the code rather
	// than of a comment: without it any package could retarget what Immutable
	// means, and could do it while requests are being served.
	readOnce sync.Once
)

// SetReader installs the function Of answers with. The first call wins, later
// calls and a nil reader are ignored.
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
