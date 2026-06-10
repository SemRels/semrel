// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestChangelogCommand_RegisteredInRoot(t *testing.T) {
	root := NewRootCommand()
	for _, sub := range root.Commands() {
		if sub.Use == "changelog" {
			return
		}
	}
	t.Error("expected 'changelog' command registered in root")
}

func TestRunChangelog_NoConfigFile(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	err = runChangelog(context.Background(), "", "text", false, "")
	if err == nil {
		t.Error("expected error when no config file exists")
	}
}

func TestRunChangelog_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
branches:
  - name: main
`)
	// runChangelog changes dir internally through OpenRepository, but since it
	// passes "." we need to chdir.
	orig, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	err := runChangelog(context.Background(), cfgPath, "text", false, "")
	if err == nil {
		t.Error("expected error when not in a git repo")
	}
	if !strings.Contains(err.Error(), "opening repository") && !strings.Contains(err.Error(), "git") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestChangelogFilePath_Default(t *testing.T) {
	t.Setenv("SEMREL_PLUGIN_CHANGELOG_FILE", "")
	path := changelogFilePath(nil)
	if path != "CHANGELOG.md" {
		t.Errorf("expected CHANGELOG.md, got %q", path)
	}
}

func TestChangelogFilePath_EnvOverride(t *testing.T) {
	t.Setenv("SEMREL_PLUGIN_CHANGELOG_FILE", "docs/CHANGES.md")
	path := changelogFilePath(nil)
	if path != "docs/CHANGES.md" {
		t.Errorf("expected docs/CHANGES.md, got %q", path)
	}
}

func TestWriteChangelogEntry_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SEMREL_PLUGIN_CHANGELOG_FILE", dir+"/CHANGELOG.md")

	entry := "## v1.1.0\n\n### Features\n- add stuff\n"
	if err := writeChangelogEntry(entry, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(dir + "/CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "## v1.1.0") {
		t.Errorf("expected entry in file, got: %s", string(data))
	}
}

func TestWriteChangelogEntry_PrependsToExisting(t *testing.T) {
	dir := t.TempDir()
	file := dir + "/CHANGELOG.md"
	t.Setenv("SEMREL_PLUGIN_CHANGELOG_FILE", file)

	existing := "## v1.0.0\n\n### Bug Fixes\n- initial\n"
	if err := os.WriteFile(file, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	entry := "## v1.1.0\n\n### Features\n- new thing\n"
	if err := writeChangelogEntry(entry, nil); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.HasPrefix(content, "## v1.1.0") {
		t.Errorf("expected new entry first, got: %s", content)
	}
	if !strings.Contains(content, "## v1.0.0") {
		t.Errorf("expected old entry preserved, got: %s", content)
	}
}

func TestEmitNoChangelog_JSONOutput(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := emitNoChangelog("json", "v1.2.3", 5, 2)

	w.Close()
	os.Stdout = old

	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if err.Error() != "no releasable commits" {
		t.Errorf("unexpected error message: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)

	var summary ChangelogSummary
	if jsonErr := json.Unmarshal(buf.Bytes(), &summary); jsonErr != nil {
		t.Fatalf("bad JSON: %v\nraw: %s", jsonErr, buf.String())
	}
	if summary.HasUnreleased {
		t.Error("expected HasUnreleased=false")
	}
	if summary.CurrentVersion != "v1.2.3" {
		t.Errorf("expected v1.2.3, got %q", summary.CurrentVersion)
	}
	if summary.Commits != 5 {
		t.Errorf("expected 5 commits, got %d", summary.Commits)
	}
}

func TestEmitNoChangelog_TextOutput(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := emitNoChangelog("text", "v1.0.0", 0, 2)

	w.Close()
	os.Stdout = old

	if err == nil {
		t.Fatal("expected non-nil error")
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	if !strings.Contains(buf.String(), "No releasable commits") {
		t.Errorf("expected 'No releasable commits' in output, got: %s", buf.String())
	}
}

func TestExitCodeError(t *testing.T) {
	e := &exitCodeError{code: 2, msg: "test"}
	if e.Error() != "test" {
		t.Errorf("unexpected message: %s", e.Error())
	}
	if e.ExitCode() != 2 {
		t.Errorf("expected code 2, got %d", e.ExitCode())
	}
}
