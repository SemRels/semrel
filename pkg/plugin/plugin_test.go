// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package plugin_test

import (
	"context"
	"errors"
	"testing"

	"github.com/SemRels/semrel/pkg/plugin"
)

// testPlugin is a minimal Plugin implementation for testing.
type testPlugin struct {
	plugin.BasePlugin
	executeErr error
	skipped    bool
}

func newTestPlugin(name string) *testPlugin {
	return &testPlugin{BasePlugin: plugin.NewBasePlugin(name, "1.0.0")}
}

func (p *testPlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
	if p.executeErr != nil {
		return nil, p.executeErr
	}
	if p.skipped {
		return plugin.SkippedResult(p.Name(), "test skip"), nil
	}
	return plugin.SuccessResult(p.Name(), map[string]string{"version": rel.Version}), nil
}

func TestBasePlugin_NameVersion(t *testing.T) {
	p := plugin.NewBasePlugin("myplugin", "2.0.0")
	if p.Name() != "myplugin" {
		t.Errorf("expected name 'myplugin', got %q", p.Name())
	}
	if p.Version() != "2.0.0" {
		t.Errorf("expected version '2.0.0', got %q", p.Version())
	}
	if err := p.Validate(); err != nil {
		t.Errorf("default Validate should return nil, got %v", err)
	}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := plugin.NewRegistry()
	p := newTestPlugin("docker")

	if err := r.Register(p); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := r.Get("docker")
	if !ok {
		t.Fatal("expected to find registered plugin")
	}
	if got.Name() != "docker" {
		t.Errorf("expected name 'docker', got %q", got.Name())
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := plugin.NewRegistry()
	p := newTestPlugin("npm")
	r.Register(p)
	if err := r.Register(p); err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegistry_List(t *testing.T) {
	r := plugin.NewRegistry()
	r.Register(newTestPlugin("docker"))
	r.Register(newTestPlugin("npm"))
	r.Register(newTestPlugin("slack"))

	names := r.List()
	if len(names) != 3 {
		t.Errorf("expected 3 plugins, got %d", len(names))
	}
}

func TestRegistry_RunAll_Success(t *testing.T) {
	r := plugin.NewRegistry()
	r.Register(newTestPlugin("a"))
	r.Register(newTestPlugin("b"))

	rel := plugin.ReleaseContext{Version: "v1.0.0", Repository: "myorg/myrepo"}
	results, err := r.RunAll(context.Background(), rel)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestRegistry_RunAll_PluginError(t *testing.T) {
	r := plugin.NewRegistry()
	p := newTestPlugin("failing")
	p.executeErr = errors.New("publish failed")
	r.Register(p)

	_, err := r.RunAll(context.Background(), plugin.ReleaseContext{Version: "v1.0.0"})
	if err == nil {
		t.Error("expected error when plugin fails")
	}
}

func TestSuccessResult(t *testing.T) {
	res := plugin.SuccessResult("docker", map[string]string{"image": "myapp:v1.0.0"})
	if res.Skipped {
		t.Error("SuccessResult should not be skipped")
	}
	if res.Outputs["image"] != "myapp:v1.0.0" {
		t.Errorf("expected image output, got %v", res.Outputs)
	}
}

func TestSkippedResult(t *testing.T) {
	res := plugin.SkippedResult("gradle", "not a Java project")
	if !res.Skipped {
		t.Error("expected Skipped to be true")
	}
	if res.SkipReason == "" {
		t.Error("expected non-empty SkipReason")
	}
}

func TestErrInvalidConfig(t *testing.T) {
	err := plugin.ErrInvalidConfig{Plugin: "npm", Message: "token required"}
	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}
