// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package builtins_test

import (
	"testing"

	"github.com/GoSemantics/semrel/pkg/builtins"
)

func TestDefaultRegistryReturnsNonNilRegistry(t *testing.T) {
	reg := builtins.DefaultRegistry()
	if reg == nil {
		t.Fatal("DefaultRegistry() returned nil")
	}
}

func TestDefaultRegistryIsEmpty(t *testing.T) {
	reg := builtins.DefaultRegistry()
	if got := len(reg.List()); got != 0 {
		t.Fatalf("DefaultRegistry() returned %d plugins, want 0", got)
	}
}
