// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package registry

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFirstPartyCanonicalIdentityAllCategories(t *testing.T) {
	tests := []struct {
		category string
		short    string
	}{
		{"provider", "github"},
		{"condition", "github-actions"},
		{"analyzer", "conventional"},
		{"generator", "changelog-md"},
		{"updater", "go"},
		{"hook", "teams"},
		{"packager", "nfpm"},
		{"publisher", "oci"},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			meta := PluginMeta{Namespace: "semrel", Name: tt.short, Category: tt.category}
			reg := PluginRegistry{Plugins: []PluginMeta{meta}}
			canonical := "@semrel/" + tt.category + "-" + tt.short
			if got := meta.CanonicalRef(); got != canonical {
				t.Fatalf("CanonicalRef() = %q, want %q", got, canonical)
			}
			for _, ref := range []string{canonical, tt.category + "-" + tt.short, tt.short, "@semrel/" + tt.short} {
				got, err := reg.FindPlugin(ref)
				if err != nil {
					t.Fatalf("FindPlugin(%q) error = %v", ref, err)
				}
				if got.CanonicalRef() != canonical {
					t.Fatalf("FindPlugin(%q) = %q, want %q", ref, got.CanonicalRef(), canonical)
				}
			}
		})
	}
}

func TestPackageNameMetadataKeepsStableResolverID(t *testing.T) {
	var meta PluginMeta
	err := json.Unmarshal([]byte(`{
		"id":"teams",
		"packageName":"@semrel/hook-teams",
		"name":"teams",
		"category":"hook",
		"aliases":[
			{"value":"teams","type":"legacy-id","pluginType":"hook","deprecated":true},
			{"ref":"hook-teams","deprecated":true}
		]
	}`), &meta)
	if err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if meta.CanonicalRef() != "@semrel/hook-teams" {
		t.Fatalf("CanonicalRef() = %q", meta.CanonicalRef())
	}
	if meta.ExecutableName() != "teams" {
		t.Fatalf("ExecutableName() = %q", meta.ExecutableName())
	}
	reg := PluginRegistry{Plugins: []PluginMeta{meta}}
	for _, alias := range []string{"teams", "hook-teams", "@semrel/hook-teams"} {
		if _, err := reg.FindPlugin(alias); err != nil {
			t.Fatalf("FindPlugin(%q) error = %v", alias, err)
		}
	}
}

func TestHistoricalAliasCannotBeClaimedByCollidingPublisher(t *testing.T) {
	reg := PluginRegistry{Plugins: []PluginMeta{{
		Namespace: "semrel",
		Name:      "publisher-npm",
		Category:  "publisher",
		Aliases:   []string{"npm", "@semrel/npm"},
	}}}
	if _, err := reg.FindPlugin("npm"); err == nil || !strings.Contains(err.Error(), "@semrel/updater-npm") {
		t.Fatalf("historical npm alias error = %v", err)
	}
	got, err := reg.FindPlugin("publisher-npm")
	if err != nil || got.CanonicalRef() != "@semrel/publisher-npm" {
		t.Fatalf("canonical publisher lookup = %v, %v", got, err)
	}
}

func TestFindPluginUsesMetadataAliasesAndRejectsAmbiguity(t *testing.T) {
	reg := PluginRegistry{Plugins: []PluginMeta{
		{Namespace: "semrel", Name: "custom", Category: "hook", Aliases: []string{"legacy-custom"}},
		{Namespace: "semrel", Name: "other", Category: "hook", Aliases: []string{"shared"}},
		{Namespace: "semrel", Name: "third", Category: "hook", LegacyAliases: []string{"shared"}},
	}}

	got, err := reg.FindPlugin("legacy-custom")
	if err != nil || got.CanonicalRef() != "@semrel/hook-custom" {
		t.Fatalf("metadata alias lookup = %v, %v", got, err)
	}
	if _, err := reg.FindPlugin("shared"); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous alias error = %v", err)
	}
}

func TestHistoricalNPMAliasNeverDependsOnRegistryOrder(t *testing.T) {
	publisher := PluginMeta{Namespace: "semrel", Name: "npm", Category: "publisher"}
	updater := PluginMeta{Namespace: "semrel", Name: "npm", Category: "updater"}

	for _, plugins := range [][]PluginMeta{{publisher, updater}, {updater, publisher}} {
		reg := PluginRegistry{Plugins: plugins}
		for _, legacy := range []string{"npm", "@semrel/npm", "updater-npm"} {
			got, err := reg.FindPlugin(legacy)
			if err != nil {
				t.Fatalf("FindPlugin(%q) error = %v", legacy, err)
			}
			if got.CanonicalRef() != "@semrel/updater-npm" {
				t.Fatalf("FindPlugin(%q) = %q", legacy, got.CanonicalRef())
			}
		}
		got, err := reg.FindPlugin("publisher-npm")
		if err != nil || got.CanonicalRef() != "@semrel/publisher-npm" {
			t.Fatalf("typed publisher lookup = %v, %v", got, err)
		}
	}
}

func TestExecutableNameRemainsCompatibleAcrossIdentityMigration(t *testing.T) {
	meta := PluginMeta{Namespace: "semrel", Name: "provider-github", Category: "provider"}
	if got := meta.CanonicalRef(); got != "@semrel/provider-github" {
		t.Fatalf("CanonicalRef() = %q", got)
	}
	if got := meta.ExecutableName(); got != "github" {
		t.Fatalf("ExecutableName() = %q, want github", got)
	}
	meta.ArtifactName = "provider-github-v2"
	if got := meta.ExecutableName(); got != "provider-github-v2" {
		t.Fatalf("explicit ExecutableName() = %q", got)
	}

	publisher := PluginMeta{Namespace: "semrel", Name: "publisher-npm", Category: "publisher"}
	if got := publisher.ExecutableName(); got != "publisher-npm" {
		t.Fatalf("new colliding publisher executable = %q", got)
	}
	updater := PluginMeta{Namespace: "semrel", Name: "updater-npm", Category: "updater"}
	if got := updater.ExecutableName(); got != "npm" {
		t.Fatalf("historical updater executable = %q", got)
	}
}
