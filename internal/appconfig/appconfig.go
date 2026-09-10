// Package appconfig hands request hot paths the few Config bits they read on
// every call, without the copy App.Config returns by value.
//
// Config is over 600 bytes and Config() returns it whole, so a caller that
// wants one boolean out of it pays for all of it — measured at 25ns against
// 2ns for the App call itself, which is a third of a header extraction.
// Package fiber replaces Lookup at init with a read straight off the app it
// owns; the default reports !ok, so every caller keeps the by-value path as a
// fallback and this package needs no import of fiber, which would be a cycle.
package appconfig

// Hot is the subset of fiber.Config that request hot paths read.
type Hot struct {
	Immutable                bool
	DisableHeaderNormalizing bool
	UnescapePath             bool
}

// Lookup returns the Hot bits of app, which must be a *fiber.App. Package
// fiber replaces it at init; until then, or for anything else, ok is false.
//
// It is written once, before main, and only read afterwards.
var Lookup = func(any) (Hot, bool) { return Hot{}, false }
