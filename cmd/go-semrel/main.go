// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The go-semrel Authors

// go-semrel is a Go-based semantic release system with a plugin architecture.
package main

import (
	"os"

	"github.com/GoSemantics/go-semrel/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
