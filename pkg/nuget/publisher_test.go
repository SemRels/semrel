// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package nuget_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoSemantics/semrel/pkg/nuget"
)

func writeFakeNupkg(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("PK\x03\x04fake nupkg content"), 0o644); err != nil {
		t.Fatalf("write fake nupkg: %v", err)
	}
	return path
}

func TestPush_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		if r.Header.Get("X-NuGet-ApiKey") != "test-key" {
			t.Errorf("expected API key header")
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := writeFakeNupkg(t, dir, "mylib.1.0.0.nupkg")

	p := nuget.NewPublisher(nuget.Config{
		APIKey: "test-key",
		Source: srv.URL + "/v3/index.json",
	})
	if err := p.Push(path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPush_Conflict_SkipDuplicate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := writeFakeNupkg(t, dir, "mylib.1.0.0.nupkg")

	p := nuget.NewPublisher(nuget.Config{
		APIKey:        "test-key",
		Source:        srv.URL + "/v3/index.json",
		SkipDuplicate: true,
	})
	if err := p.Push(path); err != nil {
		t.Fatalf("expected no error with SkipDuplicate, got: %v", err)
	}
}

func TestPush_Conflict_NoSkip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := writeFakeNupkg(t, dir, "mylib.1.0.0.nupkg")

	p := nuget.NewPublisher(nuget.Config{APIKey: "key", Source: srv.URL + "/v3/index.json"})
	if err := p.Push(path); err == nil {
		t.Fatal("expected error for 409 without SkipDuplicate")
	}
}

func TestPush_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	path := writeFakeNupkg(t, dir, "mylib.1.0.0.nupkg")

	p := nuget.NewPublisher(nuget.Config{APIKey: "key", Source: srv.URL + "/v3/index.json"})
	if err := p.Push(path); err == nil {
		t.Fatal("expected error for 500")
	}
}

func TestPush_FileNotFound(t *testing.T) {
	p := nuget.NewPublisher(nuget.Config{APIKey: "key"})
	if err := p.Push("/nonexistent/path.nupkg"); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestPushGlob_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeFakeNupkg(t, dir, "lib-a.1.0.0.nupkg")
	writeFakeNupkg(t, dir, "lib-b.2.0.0.nupkg")

	p := nuget.NewPublisher(nuget.Config{APIKey: "key", Source: srv.URL + "/v3/index.json"})
	if err := p.PushGlob(filepath.Join(dir, "*.nupkg")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPushGlob_NoMatches(t *testing.T) {
	p := nuget.NewPublisher(nuget.Config{APIKey: "key"})
	if err := p.PushGlob("/nonexistent/*.nupkg"); err == nil {
		t.Fatal("expected error for no matching files")
	}
}

func TestNewPublisher_Defaults(t *testing.T) {
	p := nuget.NewPublisher(nuget.Config{APIKey: "key"})
	if p == nil {
		t.Fatal("expected non-nil publisher")
	}
	// Source defaults to nuget.org; just verify it doesn't panic
}
