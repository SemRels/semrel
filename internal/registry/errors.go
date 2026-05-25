// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The go-semrel Authors

// Package registry provides plugin registry discovery, caching and downloads.
package registry

import "fmt"

const (
	// ErrCodeNotFound indicates that requested registry data could not be found.
	ErrCodeNotFound = "not_found"
	// ErrCodeInvalidChecksum indicates checksum verification failure.
	ErrCodeInvalidChecksum = "invalid_checksum"
	// ErrCodeNetworkError indicates an HTTP transport or remote server failure.
	ErrCodeNetworkError = "network_error"
	// ErrCodeInvalidMetadata indicates malformed registry metadata.
	ErrCodeInvalidMetadata = "invalid_metadata"
	// ErrCodeCacheError indicates a cache read/write failure.
	ErrCodeCacheError = "cache_error"
)

// RegistryError represents a typed registry client error.
type RegistryError struct {
	Code    string
	Message string
	Err     error
}

// Error returns the formatted error message.
func (e *RegistryError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err == nil {
		return fmt.Sprintf("registry %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("registry %s: %s: %v", e.Code, e.Message, e.Err)
}

// Unwrap returns the wrapped error.
func (e *RegistryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func newRegistryError(code, message string, err error) error {
	return &RegistryError{Code: code, Message: message, Err: err}
}
