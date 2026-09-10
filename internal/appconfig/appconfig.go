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

// Hot is the subset of fiber.Config that request hot paths read.
type Hot struct {
	Immutable                bool
	DisableHeaderNormalizing bool
	UnescapePath             bool
}

// Of returns the Hot bits of app, which must be a *fiber.App.
//
// Package fiber replaces this in its init, and every caller holds a fiber.Ctx
// and so has imported fiber, which makes that init ordered before any call
// here. The value below is only what something that managed to run first would
// see, and it is the zero Config. It is written once, before main, and read
// afterwards.
var Of = func(any) Hot { return Hot{} }
