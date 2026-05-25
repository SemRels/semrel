// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package signing

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type execCapture struct {
	Name           string   `json:"name"`
	Args           []string `json:"args"`
	CosignPassword string   `json:"cosignPassword"`
}

func TestSignerExecHelper(t *testing.T) {
	if os.Getenv("GO_WANT_SIGNING_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for i := range os.Args {
		if os.Args[i] == "--" {
			args = os.Args[i+1:]
			break
		}
	}
	if len(args) == 0 {
		os.Exit(2)
	}

	capture := execCapture{
		Name:           args[0],
		Args:           args[1:],
		CosignPassword: os.Getenv("COSIGN_PASSWORD"),
	}
	if path := os.Getenv("SIGNING_CAPTURE_FILE"); path != "" {
		data, _ := json.Marshal(capture)
		_ = os.WriteFile(path, data, 0o644)
	}

	if os.Getenv("SIGNING_HELPER_MODE") == "fail" {
		fmt.Fprint(os.Stderr, "helper failure")
		os.Exit(1)
	}
	os.Exit(0)
}

func TestCosignSignUsesExpectedArgsAndPassword(t *testing.T) {
	captureFile := filepath.Join(t.TempDir(), "capture.json")
	t.Setenv("GO_WANT_SIGNING_HELPER_PROCESS", "1")
	t.Setenv("SIGNING_HELPER_MODE", "success")
	t.Setenv("SIGNING_CAPTURE_FILE", captureFile)

	signer := NewSigner(Config{
		Backend:     BackendCosign,
		KeyPath:     "cosign.key",
		KeyPassword: "super-secret",
		Annotations: map[string]string{"repo": "semrel"},
	})
	signer.execCmd = helperExecCommand()

	if err := signer.SignArtifact("artifact.tgz"); err != nil {
		t.Fatalf("SignArtifact() error = %v", err)
	}

	capture := readExecCapture(t, captureFile)
	if capture.Name != "cosign" {
		t.Fatalf("command = %q, want cosign", capture.Name)
	}
	wantArgs := []string{"sign-blob", "--yes", "--key", "cosign.key", "--annotations", "repo=semrel", "--bundle", "artifact.tgz.bundle", "artifact.tgz"}
	if strings.Join(capture.Args, "|") != strings.Join(wantArgs, "|") {
		t.Fatalf("args = %#v, want %#v", capture.Args, wantArgs)
	}
	if capture.CosignPassword != "super-secret" {
		t.Fatalf("COSIGN_PASSWORD = %q, want super-secret", capture.CosignPassword)
	}
}

func TestCosignVerifyUsesExpectedArgs(t *testing.T) {
	captureFile := filepath.Join(t.TempDir(), "capture.json")
	t.Setenv("GO_WANT_SIGNING_HELPER_PROCESS", "1")
	t.Setenv("SIGNING_HELPER_MODE", "success")
	t.Setenv("SIGNING_CAPTURE_FILE", captureFile)

	signer := NewSigner(Config{Backend: BackendCosign, KeyPath: "cosign.pub"})
	signer.execCmd = helperExecCommand()

	if err := signer.VerifyArtifact("artifact.tgz"); err != nil {
		t.Fatalf("VerifyArtifact() error = %v", err)
	}

	capture := readExecCapture(t, captureFile)
	wantArgs := []string{"verify-blob", "--key", "cosign.pub", "--bundle", "artifact.tgz.bundle", "artifact.tgz"}
	if strings.Join(capture.Args, "|") != strings.Join(wantArgs, "|") {
		t.Fatalf("args = %#v, want %#v", capture.Args, wantArgs)
	}
}

func TestGPGSignUsesExpectedArgs(t *testing.T) {
	captureFile := filepath.Join(t.TempDir(), "capture.json")
	t.Setenv("GO_WANT_SIGNING_HELPER_PROCESS", "1")
	t.Setenv("SIGNING_HELPER_MODE", "success")
	t.Setenv("SIGNING_CAPTURE_FILE", captureFile)

	signer := NewSigner(Config{Backend: BackendGPG, KeyPath: "ABC123"})
	signer.execCmd = helperExecCommand()

	if err := signer.SignArtifact("artifact.tgz"); err != nil {
		t.Fatalf("SignArtifact() error = %v", err)
	}

	capture := readExecCapture(t, captureFile)
	if capture.Name != "gpg" {
		t.Fatalf("command = %q, want gpg", capture.Name)
	}
	wantArgs := []string{"--batch", "--yes", "--armor", "--detach-sign", "--local-user", "ABC123", "artifact.tgz"}
	if strings.Join(capture.Args, "|") != strings.Join(wantArgs, "|") {
		t.Fatalf("args = %#v, want %#v", capture.Args, wantArgs)
	}
}

func TestSigningCommandErrorsIncludeToolOutput(t *testing.T) {
	captureFile := filepath.Join(t.TempDir(), "capture.json")
	t.Setenv("GO_WANT_SIGNING_HELPER_PROCESS", "1")
	t.Setenv("SIGNING_HELPER_MODE", "fail")
	t.Setenv("SIGNING_CAPTURE_FILE", captureFile)

	signer := NewSigner(Config{Backend: BackendCosign})
	signer.execCmd = helperExecCommand()

	err := signer.SignArtifact("artifact.tgz")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "cosign sign-blob") || !strings.Contains(err.Error(), "helper failure") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAvailabilityHelpersRespectPATH(t *testing.T) {
	binDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(binDir, "cosign.cmd"), []byte("@echo off\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(cosign) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "gpg.cmd"), []byte("@echo off\r\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(gpg) error = %v", err)
	}

	t.Setenv("PATH", binDir)
	t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")

	if !IsCosignAvailable() {
		t.Fatal("expected cosign to be available")
	}
	if !IsGPGAvailable() {
		t.Fatal("expected gpg to be available")
	}

	backends := SupportedBackends()
	if len(backends) != 2 {
		t.Fatalf("SupportedBackends() len = %d, want 2", len(backends))
	}
}

func TestAvailabilityHelpersReturnFalseWhenMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("PATHEXT", ".COM;.EXE;.BAT;.CMD")

	if IsCosignAvailable() {
		t.Fatal("expected cosign to be unavailable")
	}
	if IsGPGAvailable() {
		t.Fatal("expected gpg to be unavailable")
	}
	if len(SupportedBackends()) != 0 {
		t.Fatalf("SupportedBackends() = %#v, want empty", SupportedBackends())
	}
}

func helperExecCommand() func(string, ...string) *exec.Cmd {
	return func(name string, args ...string) *exec.Cmd {
		cmdArgs := []string{"-test.run=^TestSignerExecHelper$", "--", name}
		cmdArgs = append(cmdArgs, args...)
		return exec.Command(os.Args[0], cmdArgs...)
	}
}

func readExecCapture(t *testing.T, path string) execCapture {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var capture execCapture
	if err := json.Unmarshal(data, &capture); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return capture
}
