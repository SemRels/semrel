// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package colors provides terminal colour helpers that degrade gracefully
// when stdout is not a TTY (CI environments, pipes, --no-color flag).
// See: https://github.com/SemRels/semrel/issues/22
package colors

import (
	"os"
)

// ANSI escape codes.
const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	white  = "\033[37m"
)

// enabled controls whether colour codes are emitted.
// It is automatically detected from the environment but can be overridden.
var enabled = isTerminal()

// isTerminal returns true if stdout appears to be an interactive terminal.
// We use a simple file-descriptor stat check; no external dependencies.
func isTerminal() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	// ModeCharDevice is set for real terminals but not pipes / redirects.
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// Enable forces colour output on (useful for testing or --color flag).
func Enable() { enabled = true }

// Disable forces colour output off (useful for --no-color flag or CI).
func Disable() { enabled = false }

// IsEnabled reports whether colour output is currently active.
func IsEnabled() bool { return enabled }

func wrap(code, s string) string {
	if !enabled {
		return s
	}
	return code + s + reset
}

// Bold returns s in bold.
func Bold(s string) string { return wrap(bold, s) }

// Green returns s in green.
func Green(s string) string { return wrap(green, s) }

// Red returns s in red.
func Red(s string) string { return wrap(red, s) }

// Yellow returns s in yellow.
func Yellow(s string) string { return wrap(yellow, s) }

// Cyan returns s in cyan.
func Cyan(s string) string { return wrap(cyan, s) }

// White returns s in white.
func White(s string) string { return wrap(white, s) }

// Success formats a ✓ success message in green.
func Success(msg string) string { return Green("✓ " + msg) }

// Warning formats a ⚠ warning message in yellow.
func Warning(msg string) string { return Yellow("⚠ " + msg) }

// Error formats a ✗ error message in red.
func Error(msg string) string { return Red("✗ " + msg) }

// Version formats a version string in cyan bold.
func Version(v string) string { return wrap(bold+cyan[2:], v) }
