// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package monorepo

import (
	"testing"
)

func TestNewDependencyGraph_Basic(t *testing.T) {
	pkgs := []PackageDep{
		{Package: Package{Name: "core", Path: "/repo/core"}},
		{Package: Package{Name: "api", Path: "/repo/api"}, DependsOn: []string{"core"}},
	}
	g, err := NewDependencyGraph(pkgs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g == nil {
		t.Fatal("expected non-nil graph")
	}
}

func TestNewDependencyGraph_UnknownDep(t *testing.T) {
	pkgs := []PackageDep{
		{Package: Package{Name: "api", Path: "/repo/api"}, DependsOn: []string{"nonexistent"}},
	}
	_, err := NewDependencyGraph(pkgs)
	if err == nil {
		t.Fatal("expected error for unknown dependency")
	}
}

func TestTopologicalOrder_NoDeps(t *testing.T) {
	pkgs := []PackageDep{
		{Package: Package{Name: "b", Path: "/repo/b"}},
		{Package: Package{Name: "a", Path: "/repo/a"}},
	}
	g, _ := NewDependencyGraph(pkgs)
	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("expected 2 packages, got %d", len(order))
	}
}

func TestTopologicalOrder_LinearChain(t *testing.T) {
	pkgs := []PackageDep{
		{Package: Package{Name: "cli", Path: "/repo/cli"}, DependsOn: []string{"api"}},
		{Package: Package{Name: "api", Path: "/repo/api"}, DependsOn: []string{"core"}},
		{Package: Package{Name: "core", Path: "/repo/core"}},
	}
	g, _ := NewDependencyGraph(pkgs)
	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 {
		t.Fatalf("expected 3 packages, got %d", len(order))
	}
	pos := make(map[string]int)
	for i, p := range order {
		pos[p.Name] = i
	}
	if pos["core"] > pos["api"] {
		t.Errorf("core (pos %d) should come before api (pos %d)", pos["core"], pos["api"])
	}
	if pos["api"] > pos["cli"] {
		t.Errorf("api (pos %d) should come before cli (pos %d)", pos["api"], pos["cli"])
	}
}

func TestTopologicalOrder_DiamondDep(t *testing.T) {
	pkgs := []PackageDep{
		{Package: Package{Name: "core"}},
		{Package: Package{Name: "api"}, DependsOn: []string{"core"}},
		{Package: Package{Name: "sdk"}, DependsOn: []string{"core"}},
		{Package: Package{Name: "cli"}, DependsOn: []string{"api", "sdk"}},
	}
	g, _ := NewDependencyGraph(pkgs)
	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 4 {
		t.Fatalf("expected 4, got %d", len(order))
	}
	pos := make(map[string]int)
	for i, p := range order {
		pos[p.Name] = i
	}
	if pos["core"] > pos["api"] || pos["core"] > pos["sdk"] {
		t.Error("core must precede api and sdk")
	}
	if pos["api"] > pos["cli"] || pos["sdk"] > pos["cli"] {
		t.Error("api and sdk must precede cli")
	}
}

func TestTopologicalOrder_Cycle(t *testing.T) {
	pkgs := []PackageDep{
		{Package: Package{Name: "a"}, DependsOn: []string{"b"}},
		{Package: Package{Name: "b"}, DependsOn: []string{"a"}},
	}
	g, _ := NewDependencyGraph(pkgs)
	_, err := g.TopologicalOrder()
	if err == nil {
		t.Fatal("expected circular dependency error")
	}
}

func TestDependents_Transitive(t *testing.T) {
	pkgs := []PackageDep{
		{Package: Package{Name: "core"}},
		{Package: Package{Name: "api"}, DependsOn: []string{"core"}},
		{Package: Package{Name: "web"}, DependsOn: []string{"api"}},
	}
	g, _ := NewDependencyGraph(pkgs)
	deps := g.Dependents("core")
	if len(deps) != 2 {
		t.Fatalf("expected 2 dependents of core, got %d: %v", len(deps), deps)
	}
}

func TestDependents_None(t *testing.T) {
	pkgs := []PackageDep{
		{Package: Package{Name: "core"}},
		{Package: Package{Name: "api"}, DependsOn: []string{"core"}},
	}
	g, _ := NewDependencyGraph(pkgs)
	deps := g.Dependents("api")
	if len(deps) != 0 {
		t.Fatalf("expected 0 dependents of api, got %d", len(deps))
	}
}

func TestNewDependencyGraph_Empty(t *testing.T) {
	g, err := NewDependencyGraph(nil)
	if err != nil {
		t.Fatalf("unexpected error for empty graph: %v", err)
	}
	order, err := g.TopologicalOrder()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 0 {
		t.Fatalf("expected 0 packages, got %d", len(order))
	}
}
