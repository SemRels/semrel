// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)

const (
	// LockFileName is the path of the plugin lock file relative to the working directory.
	// It should be committed to version control alongside .semrel.yaml.
	LockFileName = ".semrel.lock"

	lockFileVersion = 1
)

// PluginLockFile is the root of .semrel.lock.
//
// It records the exact version and checksums of every installed plugin so that
// all contributors and CI pipelines use identical binaries.  The file is meant
// to be committed to the repository and updated via `semrel plugin install`.
type PluginLockFile struct {
	LockVersion int               `json:"semrelLockVersion"`
	UpdatedAt   string            `json:"updatedAt"`
	Plugins     []PluginLockEntry `json:"plugins"`
}

// PluginLockEntry pins a single plugin to a specific version.
type PluginLockEntry struct {
	// BinaryName is the stable lookup key derived from the plugin's short name,
	// e.g. "semrel-plugin-github".  It is produced by pluginBinaryName() and is
	// consistent across naming variants ("github", "provider-github", "@SemRels/github").
	BinaryName string `json:"binaryName"`

	// Ref is the canonical registry reference, e.g. "@SemRels/github" or "github"
	// for namespace-less plugins.
	Ref string `json:"ref"`

	// Version is the pinned version, e.g. "1.2.0".
	Version string `json:"version"`

	// Checksums maps platform keys ("linux_amd64", "darwin_arm64", …) to their
	// SHA-256 hex digest.  All platforms are stored so the lock file is usable
	// on any machine regardless of OS/architecture.
	Checksums map[string]string `json:"checksums"`
}

// ReadLockFile reads .semrel.lock from the current working directory.
// If the file does not exist, an empty lock file is returned without error.
func ReadLockFile() (*PluginLockFile, error) {
	data, err := os.ReadFile(LockFileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &PluginLockFile{LockVersion: lockFileVersion}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", LockFileName, err)
	}
	var lf PluginLockFile
	if err := json.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", LockFileName, err)
	}
	return &lf, nil
}

// Write serialises the lock file and writes it to the current working directory.
// Plugins are sorted by BinaryName for deterministic diffs.
func (lf *PluginLockFile) Write() error {
	lf.LockVersion = lockFileVersion
	lf.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	sort.Slice(lf.Plugins, func(i, j int) bool {
		return lf.Plugins[i].BinaryName < lf.Plugins[j].BinaryName
	})
	data, err := json.MarshalIndent(lf, "", "  ")
	if err != nil {
		return fmt.Errorf("serialising %s: %w", LockFileName, err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(LockFileName, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", LockFileName, err)
	}
	return nil
}

// Upsert adds or replaces the lock entry identified by entry.BinaryName.
func (lf *PluginLockFile) Upsert(entry PluginLockEntry) {
	for i := range lf.Plugins {
		if lf.Plugins[i].BinaryName == entry.BinaryName {
			lf.Plugins[i] = entry
			return
		}
	}
	lf.Plugins = append(lf.Plugins, entry)
}

// FindByBinaryName returns the lock entry for the given binary name, or nil if
// no entry exists.  binaryName should be the value produced by pluginBinaryName().
func (lf *PluginLockFile) FindByBinaryName(binaryName string) *PluginLockEntry {
	for i := range lf.Plugins {
		if lf.Plugins[i].BinaryName == binaryName {
			return &lf.Plugins[i]
		}
	}
	return nil
}
