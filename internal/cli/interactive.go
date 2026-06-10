// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// isTerminal returns true when stdin is connected to a TTY.
// This is used to detect non-interactive (CI) environments.
func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	// ModeCharDevice is set for character devices (terminals).
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// interactiveConfirm shows the proposed version and changelog preview, then asks
// the user to confirm (Y), abort (n), or edit the version manually (edit).
//
// It reads from r (usually os.Stdin) and writes to w (usually os.Stderr so
// that the prompt is not mixed with JSON stdout). Returns the confirmed version
// tag, or an error if the user aborts or if stdin is not a TTY.
//
// When dryRun is true the prompt still shows (so the user can inspect the
// proposed change) but a note is printed that no actual release will occur.
func interactiveConfirm(r io.Reader, w io.Writer, currentTag, nextTag, bump, changelogEntry string, dryRun bool) (string, error) {
	fmt.Fprintf(w, "\n")
	fmt.Fprintf(w, "  Current version : %s\n", currentTag)
	fmt.Fprintf(w, "  Proposed version: %s (%s bump)\n", nextTag, bump)
	if dryRun {
		fmt.Fprintf(w, "  (dry-run mode — no changes will be made)\n")
	}
	fmt.Fprintf(w, "\n  Changelog preview:\n")
	// Indent the changelog so it's visually distinct from the prompt.
	for _, line := range strings.Split(strings.TrimRight(changelogEntry, "\n"), "\n") {
		fmt.Fprintf(w, "    %s\n", line)
	}
	fmt.Fprintf(w, "\n")

	scanner := bufio.NewScanner(r)
	for {
		fmt.Fprintf(w, "  Proceed with %s? [Y/n/edit] ", nextTag)
		if !scanner.Scan() {
			return "", fmt.Errorf("interactive prompt: stdin closed unexpectedly")
		}
		answer := strings.TrimSpace(scanner.Text())
		switch strings.ToLower(answer) {
		case "", "y", "yes":
			return nextTag, nil
		case "n", "no":
			return "", fmt.Errorf("release aborted by user")
		case "edit":
			fmt.Fprintf(w, "  Enter the version tag to use (e.g. v2.0.0): ")
			if !scanner.Scan() {
				return "", fmt.Errorf("interactive prompt: stdin closed unexpectedly")
			}
			custom := strings.TrimSpace(scanner.Text())
			if custom == "" {
				fmt.Fprintf(w, "  Version cannot be empty — try again.\n")
				continue
			}
			return custom, nil
		default:
			fmt.Fprintf(w, "  Please enter Y, n, or edit.\n")
		}
	}
}
