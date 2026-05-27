// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package lock provides a file-based release lock to prevent concurrent releases.
// See: https://github.com/SemRels/semrel/issues/46
package lock

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const lockFileName = ".semrel.lock"

// ErrLocked is returned when a release lock is already held.
var ErrLocked = errors.New("a release is already in progress")

// Info describes who holds the lock.
type Info struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Version   string    `json:"version,omitempty"`
}

// Lock represents a file-based release lock inside a repository.
type Lock struct {
	path string
}

// New creates a Lock for the given repository root directory.
func New(repoRoot string) *Lock {
	return &Lock{path: filepath.Join(repoRoot, lockFileName)}
}

// Acquire attempts to create the lock file.
// Returns ErrLocked if the lock file already exists (another process holds it).
func (l *Lock) Acquire(version string) error {
	// O_EXCL ensures the file is created atomically — fails if it already exists.
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			info, readErr := l.Info()
			if readErr == nil {
				return fmt.Errorf("%w: started at %s by PID %d (releasing %s)",
					ErrLocked, info.StartedAt.Format(time.RFC3339), info.PID, info.Version)
			}
			return ErrLocked
		}
		return fmt.Errorf("acquiring release lock: %w", err)
	}
	defer f.Close() //nolint:errcheck

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(Info{
		PID:       os.Getpid(),
		StartedAt: time.Now().UTC(),
		Version:   version,
	})
}

// Release removes the lock file, freeing the lock.
// Returns nil if the lock file doesn't exist (idempotent).
func (l *Lock) Release() error {
	err := os.Remove(l.path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("releasing release lock: %w", err)
	}
	return nil
}

// IsLocked returns true if a lock file currently exists.
func (l *Lock) IsLocked() bool {
	_, err := os.Stat(l.path)
	return err == nil
}

// Info returns the lock information from an existing lock file.
func (l *Lock) Info() (Info, error) {
	data, err := os.ReadFile(l.path)
	if err != nil {
		return Info{}, fmt.Errorf("reading lock file: %w", err)
	}
	var info Info
	if err := json.Unmarshal(data, &info); err != nil {
		return Info{}, fmt.Errorf("parsing lock file: %w", err)
	}
	return info, nil
}

// Path returns the absolute path to the lock file.
func (l *Lock) Path() string {
	return l.path
}
