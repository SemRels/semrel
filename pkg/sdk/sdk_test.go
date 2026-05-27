// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package sdk

import (
	"context"
	"testing"
)

// ---------------------------------------------------------------------------
// Config helpers
// ---------------------------------------------------------------------------

func TestConfig_GetString(t *testing.T) {
	c := Config{"key": "value", "num": 42}
	if got := c.GetString("key"); got != "value" {
		t.Errorf("expected 'value', got %q", got)
	}
	if got := c.GetString("num"); got != "" {
		t.Errorf("non-string should return '', got %q", got)
	}
	if got := c.GetString("missing"); got != "" {
		t.Errorf("missing key should return '', got %q", got)
	}
}

func TestConfig_GetBool(t *testing.T) {
	c := Config{"yes": true, "no": false, "str": "true"}
	if !c.GetBool("yes") {
		t.Error("expected true for 'yes'")
	}
	if c.GetBool("no") {
		t.Error("expected false for 'no'")
	}
	if c.GetBool("str") {
		t.Error("string 'true' should not be coerced to bool")
	}
	if c.GetBool("missing") {
		t.Error("missing key should return false")
	}
}

func TestConfig_Get(t *testing.T) {
	c := Config{"key": 42}
	v, ok := c.Get("key")
	if !ok {
		t.Error("expected ok=true for existing key")
	}
	if v != 42 {
		t.Errorf("expected 42, got %v", v)
	}
	_, ok = c.Get("missing")
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

// ---------------------------------------------------------------------------
// Plugin interface compliance
// ---------------------------------------------------------------------------

// mockPlugin implements the Plugin interface for testing.
type mockPlugin struct {
	preErr  error
	postErr error
}

func (m *mockPlugin) Name() string                                        { return "mock-plugin" }
func (m *mockPlugin) Version() string                                     { return "0.1.0" }
func (m *mockPlugin) PreRelease(_ context.Context, _ Config) error        { return m.preErr }
func (m *mockPlugin) PostRelease(_ context.Context, _ ReleaseEvent) error { return m.postErr }

func TestPlugin_Interface(t *testing.T) {
	var _ Plugin = &mockPlugin{}
}

func TestPlugin_Name(t *testing.T) {
	p := &mockPlugin{}
	if p.Name() != "mock-plugin" {
		t.Errorf("unexpected Name: %q", p.Name())
	}
}

func TestPlugin_Version(t *testing.T) {
	p := &mockPlugin{}
	if p.Version() != "0.1.0" {
		t.Errorf("unexpected Version: %q", p.Version())
	}
}

func TestPlugin_PreRelease_Success(t *testing.T) {
	p := &mockPlugin{}
	if err := p.PreRelease(context.Background(), nil); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPlugin_PostRelease_Success(t *testing.T) {
	p := &mockPlugin{}
	event := ReleaseEvent{Version: "1.0.0", TagName: "v1.0.0"}
	if err := p.PostRelease(context.Background(), event); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ReleaseEvent
// ---------------------------------------------------------------------------

func TestReleaseEvent_Fields(t *testing.T) {
	e := ReleaseEvent{
		Version:         "1.2.3",
		PreviousVersion: "1.2.2",
		TagName:         "v1.2.3",
		Changelog:       "## v1.2.3\n- feat: something",
		Repository:      "https://github.com/org/repo.git",
		Branch:          "main",
		DryRun:          false,
	}
	if e.Version != "1.2.3" {
		t.Errorf("expected 1.2.3, got %q", e.Version)
	}
	if e.TagName != "v1.2.3" {
		t.Errorf("expected v1.2.3, got %q", e.TagName)
	}
}
