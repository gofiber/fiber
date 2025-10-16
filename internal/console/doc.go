// Package console provides helpers for interacting with terminal-aware writers.
//
// It centralizes the cross-platform logic required by Fiber's startup and logging
// subsystems, including ANSI color translation on Windows and terminal detection
// helpers that mirror the behavior of the historical go-colorable and go-isatty
// dependencies.
package console
