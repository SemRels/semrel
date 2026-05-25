// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package lock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireAndRelease(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)

	if err := l.Acquire("v1.2.3"); err != nil {
		t.Fatalf("Acquire: unexpected error: %v", err)
	}
	defer l.Release() //nolint:errcheck

	if !l.IsLocked() {
		t.Error("expected IsLocked() == true after Acquire")
	}

	if err := l.Release(); err != nil {
		t.Fatalf("Release: unexpected error: %v", err)
	}

	if l.IsLocked() {
		t.Error("expected IsLocked() == false after Release")
	}
}

func TestAcquire_AlreadyLocked(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)

	if err := l.Acquire("v1.0.0"); err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	defer l.Release() //nolint:errcheck

	err := l.Acquire("v1.1.0")
	if err == nil {
		t.Fatal("expected error on second Acquire, got nil")
	}
	if !errors.Is(err, ErrLocked) {
		t.Errorf("expected ErrLocked, got: %v", err)
	}
}

func TestInfo(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)

	before := time.Now().UTC().Add(-time.Second)
	if err := l.Acquire("v2.0.0"); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer l.Release() //nolint:errcheck

	info, err := l.Info()
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Version != "v2.0.0" {
		t.Errorf("expected version v2.0.0, got %q", info.Version)
	}
	if info.PID <= 0 {
		t.Errorf("expected positive PID, got %d", info.PID)
	}
	if info.StartedAt.Before(before) {
		t.Errorf("StartedAt %v is before test start %v", info.StartedAt, before)
	}
}

func TestRelease_NotLocked(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)
	// Release without acquiring should be idempotent
	if err := l.Release(); err != nil {
		t.Errorf("Release on non-existent lock: %v", err)
	}
}

func TestIsLocked_NoFile(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)
	if l.IsLocked() {
		t.Error("expected IsLocked() == false for fresh directory")
	}
}

func TestPath(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)
	expected := filepath.Join(dir, ".semrel.lock")
	if l.Path() != expected {
		t.Errorf("expected %q, got %q", expected, l.Path())
	}
}

func TestAcquire_UnwritableDirectory(t *testing.T) {
	// Try to acquire lock in a directory that doesn't exist
	l := New(filepath.Join(t.TempDir(), "nonexistent", "subdir"))
	err := l.Acquire("v1.0.0")
	if err == nil {
		t.Error("expected error for unwritable/nonexistent directory")
	}
}

func TestInfo_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	l := New(dir)

	// Write corrupt JSON to lock file
	if err := os.WriteFile(l.Path(), []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("writing corrupt lock file: %v", err)
	}

	_, err := l.Info()
	if err == nil {
		t.Error("expected error reading corrupt lock file")
	}
}
