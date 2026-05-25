// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package plugininstance

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// PluginSpec.InstanceID()
// ---------------------------------------------------------------------------

func TestPluginSpec_InstanceID_Name(t *testing.T) {
	s := PluginSpec{Uses: "plugin-a", Name: "my-instance"}
	if s.InstanceID() != "my-instance" {
		t.Errorf("expected 'my-instance', got %q", s.InstanceID())
	}
}

func TestPluginSpec_InstanceID_Uses(t *testing.T) {
	s := PluginSpec{Uses: "plugin-a"}
	if s.InstanceID() != "plugin-a" {
		t.Errorf("expected 'plugin-a', got %q", s.InstanceID())
	}
}

func TestPluginSpec_InstanceID_Path(t *testing.T) {
	s := PluginSpec{Path: "/usr/local/bin/myplugin"}
	if s.InstanceID() != "/usr/local/bin/myplugin" {
		t.Errorf("expected path as ID, got %q", s.InstanceID())
	}
}

// ---------------------------------------------------------------------------
// Orchestrator.Run()
// ---------------------------------------------------------------------------

func TestOrchestrator_Run_Empty(t *testing.T) {
	o := NewOrchestrator(func(_ context.Context, _ PluginSpec) error {
		t.Error("runner should not be called for empty spec list")
		return nil
	})
	if err := o.Run(context.Background(), nil); err != nil {
		t.Errorf("expected no error for empty specs, got: %v", err)
	}
}

func TestOrchestrator_Run_AllSuccess(t *testing.T) {
	var called []string
	runner := func(_ context.Context, spec PluginSpec) error {
		called = append(called, spec.InstanceID())
		return nil
	}
	o := NewOrchestrator(runner)
	specs := []PluginSpec{
		{Uses: "plugin-a", Name: "instance-1"},
		{Uses: "plugin-a", Name: "instance-2"},
		{Uses: "plugin-b"},
	}
	if err := o.Run(context.Background(), specs); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
	if len(called) != 3 {
		t.Errorf("expected 3 runner calls, got %d: %v", len(called), called)
	}
	if called[0] != "instance-1" || called[1] != "instance-2" || called[2] != "plugin-b" {
		t.Errorf("unexpected call order: %v", called)
	}
}

func TestOrchestrator_Run_SamePluginMultipleInstances(t *testing.T) {
	var configs []map[string]interface{}
	runner := func(_ context.Context, spec PluginSpec) error {
		configs = append(configs, spec.Config)
		return nil
	}
	o := NewOrchestrator(runner)
	specs := []PluginSpec{
		{Uses: "semrel-plugin-publish", Name: "staging", Config: map[string]interface{}{"registry": "staging.example.com"}},
		{Uses: "semrel-plugin-publish", Name: "prod", Config: map[string]interface{}{"registry": "prod.example.com"}},
	}
	if err := o.Run(context.Background(), specs); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
	if configs[0]["registry"] != "staging.example.com" {
		t.Errorf("expected staging registry, got %v", configs[0])
	}
	if configs[1]["registry"] != "prod.example.com" {
		t.Errorf("expected prod registry, got %v", configs[1])
	}
}

func TestOrchestrator_Run_PartialFailure(t *testing.T) {
	runner := func(_ context.Context, spec PluginSpec) error {
		if spec.Name == "failing" {
			return errors.New("something went wrong")
		}
		return nil
	}
	o := NewOrchestrator(runner)
	specs := []PluginSpec{
		{Uses: "plugin-a", Name: "ok"},
		{Uses: "plugin-b", Name: "failing"},
		{Uses: "plugin-c", Name: "also-ok"},
	}
	err := o.Run(context.Background(), specs)
	if err == nil {
		t.Fatal("expected error for failing instance")
	}
	if !strings.Contains(err.Error(), "failing") {
		t.Errorf("expected error to mention 'failing', got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "something went wrong") {
		t.Errorf("expected original error in message, got %q", err.Error())
	}
}

func TestOrchestrator_Run_AllFail_CombinedError(t *testing.T) {
	runner := func(_ context.Context, spec PluginSpec) error {
		return fmt.Errorf("%s failed", spec.InstanceID())
	}
	o := NewOrchestrator(runner)
	specs := []PluginSpec{
		{Name: "a"},
		{Name: "b"},
	}
	err := o.Run(context.Background(), specs)
	if err == nil {
		t.Fatal("expected combined error")
	}
	if !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "b") {
		t.Errorf("expected both errors, got %q", err.Error())
	}
}

func TestOrchestrator_Run_ContinuesAfterFailure(t *testing.T) {
	called := 0
	runner := func(_ context.Context, spec PluginSpec) error {
		called++
		return errors.New("always fail")
	}
	o := NewOrchestrator(runner)
	specs := []PluginSpec{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	o.Run(context.Background(), specs)
	if called != 3 {
		t.Errorf("expected all 3 instances to be attempted, got %d", called)
	}
}
