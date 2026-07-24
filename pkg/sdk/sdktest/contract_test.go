//go:build integration

// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package sdktest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestContractServerMatchesAndRecordsStatefulRequests(t *testing.T) {
	server := NewContractServer(t,
		ExpectedRequest{
			Name:   "create release",
			Method: http.MethodPost,
			Path:   "/releases?draft=true",
			Headers: http.Header{
				"Authorization": {"Bearer test-token"},
				"Content-Type":  {"application/json"},
			},
			Body: []byte(`{"tag":"v1.2.3"}`),
			Response: Response{
				Status:  http.StatusCreated,
				Headers: http.Header{"X-Request-ID": {"request-1"}},
				Body:    []byte(`{"id":42}`),
			},
		},
		ExpectedRequest{
			Name:   "read release",
			Method: http.MethodGet,
			Path:   "/releases/42",
			Response: Response{
				Status: http.StatusOK,
				Body:   []byte(`{"id":42,"tag":"v1.2.3"}`),
			},
		},
	)

	request, err := http.NewRequest(http.MethodPost, server.URL+"/releases?draft=true", strings.NewReader(`{"tag":"v1.2.3"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer test-token")
	request.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	if response.Header.Get("X-Request-ID") != "request-1" {
		t.Fatalf("X-Request-ID = %q", response.Header.Get("X-Request-ID"))
	}
	if string(body) != `{"id":42}` {
		t.Fatalf("body = %q", body)
	}

	response, err = server.Client().Get(server.URL + "/releases/42")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()

	requests := server.Requests()
	if len(requests) != 2 {
		t.Fatalf("recorded requests = %d, want 2", len(requests))
	}
	if requests[0].Method != http.MethodPost || requests[0].Path != "/releases?draft=true" {
		t.Fatalf("first recorded request = %#v", requests[0])
	}
	if !bytes.Equal(requests[0].Body, []byte(`{"tag":"v1.2.3"}`)) {
		t.Fatalf("first recorded body = %q", requests[0].Body)
	}

	requests[0].Body[0] = '!'
	if bytes.Equal(server.Requests()[0].Body, requests[0].Body) {
		t.Fatal("Requests returned mutable server state")
	}
}

func TestContractServerReportsMismatchesAndUnexpectedRequests(t *testing.T) {
	recorder := &recordingTestingT{}
	server := NewContractServer(recorder, ExpectedRequest{
		Method: http.MethodPost,
		Path:   "/expected",
		Headers: http.Header{
			"Authorization": {"Bearer expected"},
		},
		Body: []byte("expected"),
	})

	request, err := http.NewRequest(http.MethodGet, server.URL+"/wrong", strings.NewReader("actual"))
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("mismatch status = %d, want %d", response.StatusCode, http.StatusInternalServerError)
	}

	response, err = server.Client().Get(server.URL + "/extra")
	if err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	recorder.runCleanups()

	failures := strings.Join(recorder.errors, "\n")
	if !strings.Contains(failures, `method = "GET", want "POST"`) {
		t.Fatalf("missing method mismatch in %q", failures)
	}
	if !strings.Contains(failures, "unexpected GET /extra") {
		t.Fatalf("missing unexpected request in %q", failures)
	}
}

func TestContractServerWaitTimeoutAndCleanup(t *testing.T) {
	server := NewContractServer(t, ExpectedRequest{
		Method: http.MethodGet,
		Path:   "/slow",
		Response: Response{
			Delay: 5 * time.Second,
		},
	})

	requestContext, cancelRequest := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancelRequest()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, server.URL+"/slow", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = server.Client().Do(request); err == nil {
		t.Fatal("slow request unexpectedly succeeded")
	}

	waitContext, cancelWait := context.WithTimeout(context.Background(), time.Second)
	defer cancelWait()
	if err := server.WaitForRequests(waitContext, 1); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	server.Close()
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("Close took %s; delayed handler was not cleaned up", elapsed)
	}

	timeoutContext, cancelTimeout := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelTimeout()
	if err := server.WaitForRequests(timeoutContext, 2); err == nil || !strings.Contains(err.Error(), "received 1") {
		t.Fatalf("WaitForRequests timeout = %v", err)
	}
}

type recordingTestingT struct {
	cleanups []func()
	errors   []string
}

func (t *recordingTestingT) Helper() {}

func (t *recordingTestingT) Errorf(format string, args ...any) {
	t.errors = append(t.errors, strings.TrimSpace(fmt.Sprintf(format, args...)))
}

func (t *recordingTestingT) Cleanup(cleanup func()) {
	t.cleanups = append(t.cleanups, cleanup)
}

func (t *recordingTestingT) runCleanups() {
	for i := len(t.cleanups) - 1; i >= 0; i-- {
		t.cleanups[i]()
	}
}
