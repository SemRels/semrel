// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPluginCommandRegistered(t *testing.T) {
	root := NewRootCommand()
	var found bool
	for _, cmd := range root.Commands() {
		if cmd.Use == "plugin" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'plugin' subcommand to be registered")
	}
}

func TestPluginSubcommandsExist(t *testing.T) {
	root := NewRootCommand()
	var pluginCmd *bytes.Buffer
	_ = pluginCmd

	var pc interface{ Commands() []*interface{} }
	_ = pc

	for _, cmd := range root.Commands() {
		if cmd.Use != "plugin" {
			continue
		}
		names := make([]string, 0, len(cmd.Commands()))
		for _, sub := range cmd.Commands() {
			names = append(names, sub.Use)
		}
		for _, want := range []string{"list", "search <query>", "install <name[@version]>"} {
			found := false
			for _, n := range names {
				if strings.HasPrefix(n, strings.Fields(want)[0]) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected subcommand %q, got %v", want, names)
			}
		}
	}
}

func TestSplitNameVersion(t *testing.T) {
	cases := []struct{ input, name, ver string }{
		{"npm", "npm", ""},
		{"npm@1.2.0", "npm", "1.2.0"},
		{"github@v2.0.0", "github", "v2.0.0"},
		{"my-plugin@0.1.0-beta", "my-plugin", "0.1.0-beta"},
	}
	for _, c := range cases {
		n, v := splitNameVersion(c.input)
		if n != c.name || v != c.ver {
			t.Errorf("splitNameVersion(%q): got (%q,%q) want (%q,%q)", c.input, n, v, c.name, c.ver)
		}
	}
}

func TestContainsTag(t *testing.T) {
	tags := []string{"npm", "nodejs", "javascript"}
	if !containsTag(tags, "node") {
		t.Error("expected 'node' to match tag 'nodejs'")
	}
	if containsTag(tags, "python") {
		t.Error("expected 'python' not to match any tag")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello world", 5); got != "hell…" {
		t.Errorf("truncate: got %q", got)
	}
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate short: got %q", got)
	}
}
