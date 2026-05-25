// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type helperPlugin struct{}

func (helperPlugin) Name() string { return "helper-plugin" }

func (helperPlugin) Version() string { return "1.2.3" }

func (helperPlugin) PreRelease(_ context.Context, _ Config) error {
	if os.Getenv("SEMREL_SDK_HELPER_MODE") == "pre-error" {
		return errors.New("pre-release failed")
	}
	return nil
}

func (helperPlugin) PostRelease(_ context.Context, _ ReleaseEvent) error {
	if os.Getenv("SEMREL_SDK_HELPER_MODE") == "post-error" {
		return errors.New("post-release failed")
	}
	return nil
}

func TestRunHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_SDK_HELPER_PROCESS") != "1" {
		return
	}
	Run(helperPlugin{})
	os.Exit(0)
}

func TestRunPreReleaseSuccess(t *testing.T) {
	req := Request{Action: "pre-release", Config: Config{"token": "abc"}}
	stdout, stderr, exitCode := runSDKProcess(t, "", mustMarshalRequest(t, req))
	if exitCode != ExitSuccess {
		t.Fatalf("exit code = %d, want %d (stderr=%s)", exitCode, ExitSuccess, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	resp := decodeSDKResponse(t, stdout)
	if !resp.Success {
		t.Fatalf("response.Success = false, want true")
	}
	if !strings.Contains(resp.Message, "pre-release completed successfully") {
		t.Fatalf("response.Message = %q", resp.Message)
	}
}

func TestRunPostReleaseSuccess(t *testing.T) {
	req := Request{Action: "post-release", Event: ReleaseEvent{Version: "1.0.0", TagName: "v1.0.0"}}
	stdout, stderr, exitCode := runSDKProcess(t, "", mustMarshalRequest(t, req))
	if exitCode != ExitSuccess {
		t.Fatalf("exit code = %d, want %d (stderr=%s)", exitCode, ExitSuccess, stderr)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}

	resp := decodeSDKResponse(t, stdout)
	if !resp.Success {
		t.Fatalf("response.Success = false, want true")
	}
	if !strings.Contains(resp.Message, "post-release completed successfully") {
		t.Fatalf("response.Message = %q", resp.Message)
	}
}

func TestRunDecodeError(t *testing.T) {
	stdout, stderr, exitCode := runSDKProcess(t, "", []byte(`{"action":`))
	if exitCode != ExitError {
		t.Fatalf("exit code = %d, want %d", exitCode, ExitError)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}

	resp := decodeSDKResponse(t, stderr)
	if resp.Success {
		t.Fatalf("response.Success = true, want false")
	}
	if !strings.Contains(resp.Message, "decoding request") {
		t.Fatalf("response.Message = %q", resp.Message)
	}
}

func TestRunUnknownAction(t *testing.T) {
	req := Request{Action: "unknown"}
	stdout, stderr, exitCode := runSDKProcess(t, "", mustMarshalRequest(t, req))
	if exitCode != ExitError {
		t.Fatalf("exit code = %d, want %d", exitCode, ExitError)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}

	resp := decodeSDKResponse(t, stderr)
	if resp.Success {
		t.Fatalf("response.Success = true, want false")
	}
	if !strings.Contains(resp.Message, `unknown action: "unknown"`) {
		t.Fatalf("response.Message = %q", resp.Message)
	}
}

func TestRunPluginErrors(t *testing.T) {
	t.Run("pre-release", func(t *testing.T) {
		req := Request{Action: "pre-release", Config: Config{"enabled": true}}
		stdout, stderr, exitCode := runSDKProcess(t, "pre-error", mustMarshalRequest(t, req))
		if exitCode != ExitError {
			t.Fatalf("exit code = %d, want %d", exitCode, ExitError)
		}
		if stdout != "" {
			t.Fatalf("stdout = %q, want empty", stdout)
		}

		resp := decodeSDKResponse(t, stderr)
		if !strings.Contains(resp.Message, "pre-release failed") {
			t.Fatalf("response.Message = %q", resp.Message)
		}
	})

	t.Run("post-release", func(t *testing.T) {
		req := Request{Action: "post-release", Event: ReleaseEvent{Version: "1.0.0"}}
		stdout, stderr, exitCode := runSDKProcess(t, "post-error", mustMarshalRequest(t, req))
		if exitCode != ExitError {
			t.Fatalf("exit code = %d, want %d", exitCode, ExitError)
		}
		if stdout != "" {
			t.Fatalf("stdout = %q, want empty", stdout)
		}

		resp := decodeSDKResponse(t, stderr)
		if !strings.Contains(resp.Message, "post-release failed") {
			t.Fatalf("response.Message = %q", resp.Message)
		}
	})
}

func runSDKProcess(t *testing.T, mode string, input []byte) (string, string, int) {
	t.Helper()

	cmd := exec.Command(os.Args[0], "-test.run=^TestRunHelperProcess$")
	cmd.Env = append(os.Environ(),
		"GO_WANT_SDK_HELPER_PROCESS=1",
		"SEMREL_SDK_HELPER_MODE="+mode,
	)
	cmd.Stdin = bytes.NewReader(input)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), ExitSuccess
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("Run() error = %v", err)
	}
	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), exitErr.ExitCode()
}

func decodeSDKResponse(t *testing.T, raw string) Response {
	t.Helper()

	var resp Response
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", raw, err)
	}
	return resp
}

func mustMarshalRequest(t *testing.T, req Request) []byte {
	t.Helper()

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return data
}
