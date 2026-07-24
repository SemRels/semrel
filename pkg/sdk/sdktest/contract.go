// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package sdktest provides reusable contract fixtures for semrel plugins and
// clients. It is intended for test code only.
package sdktest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"
)

// TestingT is the subset of testing.T used by ContractServer.
type TestingT interface {
	Helper()
	Errorf(format string, args ...any)
	Cleanup(func())
}

// Response describes the response returned for an expected request.
type Response struct {
	Status  int
	Headers http.Header
	Body    []byte
	Delay   time.Duration
}

// ExpectedRequest describes one request in a stateful HTTP exchange. Path is
// matched against RequestURI, so it may include an exact query string.
// Headers are matched as a subset and Body is matched byte-for-byte.
type ExpectedRequest struct {
	Name     string
	Method   string
	Path     string
	Headers  http.Header
	Body     []byte
	Response Response
}

// RecordedRequest is an immutable snapshot of a request received by the server.
type RecordedRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

// ContractServer is a stateful, ordered HTTP contract server.
type ContractServer struct {
	URL string

	server *httptest.Server
	once   sync.Once
	notify chan struct{}

	mu           sync.Mutex
	expectations []ExpectedRequest
	requests     []RecordedRequest
	failures     []string
}

// NewContractServer starts a server on a random loopback port. It verifies all
// expectations and closes active connections during test cleanup.
func NewContractServer(t TestingT, expectations ...ExpectedRequest) *ContractServer {
	t.Helper()

	server := &ContractServer{
		expectations: append([]ExpectedRequest(nil), expectations...),
		notify:       make(chan struct{}, 1),
	}
	server.server = httptest.NewServer(http.HandlerFunc(server.serveHTTP))
	server.URL = server.server.URL
	t.Cleanup(func() {
		server.Close()
		server.AssertExpectations(t)
	})
	return server
}

// Close stops the server and all active client connections. It is idempotent.
func (s *ContractServer) Close() {
	s.once.Do(func() {
		s.server.CloseClientConnections()
		s.server.Close()
	})
}

// Client returns an HTTP client configured for this server.
func (s *ContractServer) Client() *http.Client {
	return s.server.Client()
}

// Requests returns snapshots of all requests received so far.
func (s *ContractServer) Requests() []RecordedRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	requests := make([]RecordedRequest, len(s.requests))
	for i, request := range s.requests {
		requests[i] = RecordedRequest{
			Method:  request.Method,
			Path:    request.Path,
			Headers: request.Headers.Clone(),
			Body:    append([]byte(nil), request.Body...),
		}
	}
	return requests
}

// WaitForRequests waits until at least count requests have been recorded.
func (s *ContractServer) WaitForRequests(ctx context.Context, count int) error {
	for {
		s.mu.Lock()
		got := len(s.requests)
		s.mu.Unlock()
		if got >= count {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("waiting for %d requests (received %d): %w", count, got, ctx.Err())
		case <-s.notify:
		}
	}
}

// AssertExpectations reports mismatches and unconsumed expectations. It
// returns true when the complete contract was satisfied.
func (s *ContractServer) AssertExpectations(t interface {
	Helper()
	Errorf(format string, args ...any)
}) bool {
	t.Helper()

	s.mu.Lock()
	failures := append([]string(nil), s.failures...)
	remaining := append([]ExpectedRequest(nil), s.expectations...)
	s.mu.Unlock()

	for _, failure := range failures {
		t.Errorf("HTTP contract: %s", failure)
	}
	for _, expectation := range remaining {
		t.Errorf("HTTP contract: expected request was not received: %s", expectationLabel(expectation))
	}
	return len(failures) == 0 && len(remaining) == 0
}

func (s *ContractServer) serveHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read request", http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	recorded := RecordedRequest{
		Method:  r.Method,
		Path:    r.URL.RequestURI(),
		Headers: r.Header.Clone(),
		Body:    append([]byte(nil), body...),
	}

	s.mu.Lock()
	s.requests = append(s.requests, recorded)
	var expectation ExpectedRequest
	if len(s.expectations) == 0 {
		s.failures = append(s.failures, fmt.Sprintf("unexpected %s %s", r.Method, r.URL.RequestURI()))
		s.mu.Unlock()
		s.signalRequest()
		http.Error(w, "unexpected request", http.StatusInternalServerError)
		return
	}
	expectation = s.expectations[0]
	s.expectations = s.expectations[1:]
	failure := matchRequest(expectation, recorded)
	if failure != "" {
		s.failures = append(s.failures, failure)
	}
	s.mu.Unlock()
	s.signalRequest()

	if failure != "" {
		http.Error(w, "request did not match contract", http.StatusInternalServerError)
		return
	}
	if expectation.Response.Delay > 0 {
		timer := time.NewTimer(expectation.Response.Delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-r.Context().Done():
			return
		}
	}
	for name, values := range expectation.Response.Headers {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	status := expectation.Response.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(expectation.Response.Body)
}

func (s *ContractServer) signalRequest() {
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

func matchRequest(expectation ExpectedRequest, request RecordedRequest) string {
	label := expectationLabel(expectation)
	if expectation.Method != "" && request.Method != expectation.Method {
		return fmt.Sprintf("%s: method = %q, want %q", label, request.Method, expectation.Method)
	}
	if expectation.Path != "" && request.Path != expectation.Path {
		return fmt.Sprintf("%s: path = %q, want %q", label, request.Path, expectation.Path)
	}
	for name, expected := range expectation.Headers {
		actual := request.Headers.Values(name)
		if !equalStrings(actual, expected) {
			return fmt.Sprintf("%s: header %q = %q, want %q", label, name, actual, expected)
		}
	}
	if expectation.Body != nil && !bytes.Equal(request.Body, expectation.Body) {
		return fmt.Sprintf("%s: body = %q, want %q", label, request.Body, expectation.Body)
	}
	return ""
}

func expectationLabel(expectation ExpectedRequest) string {
	if expectation.Name != "" {
		return expectation.Name
	}
	return strings.TrimSpace(expectation.Method + " " + expectation.Path)
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
