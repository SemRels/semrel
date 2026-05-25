// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Inter-package dependency graph and topological release ordering for monorepos.
//
// In a monorepo, Package A may depend on Package B. When B is released, A
// might need to be released immediately after (or simultaneously). The
// dependency graph determines the correct release order so that upstream
// packages are released before their consumers.
//
// See: https://github.com/SemRels/semrel/issues/43

package monorepo

import (
	"fmt"
	"strings"
)

// PackageDep extends Package with explicit dependency declarations.
type PackageDep struct {
	Package
	// DependsOn lists the names of packages this package depends on.
	DependsOn []string
}

// DependencyGraph holds the inter-package dependency relationships.
type DependencyGraph struct {
	packages map[string]*PackageDep
}

// NewDependencyGraph creates a graph from a list of PackageDep entries.
// Returns an error if a dependency references an unknown package name.
func NewDependencyGraph(packages []PackageDep) (*DependencyGraph, error) {
	g := &DependencyGraph{
		packages: make(map[string]*PackageDep, len(packages)),
	}
	for i := range packages {
		p := &packages[i]
		g.packages[p.Name] = p
	}
	for _, p := range packages {
		for _, dep := range p.DependsOn {
			if _, ok := g.packages[dep]; !ok {
				return nil, fmt.Errorf("package %q depends on unknown package %q", p.Name, dep)
			}
		}
	}
	return g, nil
}

// TopologicalOrder returns packages in dependency order using Kahn's algorithm:
// dependencies are released before the packages that depend on them.
// Returns an error if a circular dependency is detected.
func (g *DependencyGraph) TopologicalOrder() ([]PackageDep, error) {
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for name := range g.packages {
		if _, ok := inDegree[name]; !ok {
			inDegree[name] = 0
		}
	}
	for _, p := range g.packages {
		for _, dep := range p.DependsOn {
			inDegree[p.Name]++
			dependents[dep] = append(dependents[dep], p.Name)
		}
	}

	var queue []string
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}
	depgraphSort(queue)

	var result []PackageDep
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		result = append(result, *g.packages[name])

		deps := dependents[name]
		depgraphSort(deps)
		for _, dependent := range deps {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
				depgraphSort(queue)
			}
		}
	}

	if len(result) != len(g.packages) {
		var cycle []string
		for name, deg := range inDegree {
			if deg > 0 {
				cycle = append(cycle, name)
			}
		}
		depgraphSort(cycle)
		return nil, fmt.Errorf("circular dependency detected among: %s", strings.Join(cycle, ", "))
	}

	return result, nil
}

// Dependents returns all packages that directly or indirectly depend on
// the package with the given name.
func (g *DependencyGraph) Dependents(name string) []string {
	visited := make(map[string]bool)
	var result []string
	g.collectDependents(name, visited, &result)
	depgraphSort(result)
	return result
}

func (g *DependencyGraph) collectDependents(name string, visited map[string]bool, result *[]string) {
	for _, p := range g.packages {
		if visited[p.Name] {
			continue
		}
		for _, dep := range p.DependsOn {
			if dep == name {
				visited[p.Name] = true
				*result = append(*result, p.Name)
				g.collectDependents(p.Name, visited, result)
				break
			}
		}
	}
}

// depgraphSort sorts a string slice in-place (insertion sort for small slices).
func depgraphSort(s []string) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}
