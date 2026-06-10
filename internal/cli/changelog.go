// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/GoSemantics/semrel/pkg/changelog"
	"github.com/GoSemantics/semrel/pkg/commits"
	"github.com/GoSemantics/semrel/pkg/config"
	gitpkg "github.com/GoSemantics/semrel/pkg/git"
	"github.com/GoSemantics/semrel/pkg/semver"
)

// ChangelogSummary is the structured output of `semrel changelog`.
type ChangelogSummary struct {
	// HasUnreleased is true when there are releasable commits since the last tag.
	HasUnreleased bool `json:"has_unreleased"`
	// CurrentVersion is the version of the latest tag (e.g. "v1.2.3").
	CurrentVersion string `json:"current_version"`
	// NextVersion is the projected next version (e.g. "v1.3.0").
	NextVersion string `json:"next_version,omitempty"`
	// Bump is the semver bump type: "major", "minor", or "patch".
	Bump string `json:"bump,omitempty"`
	// Commits is the number of unreleased commits analysed.
	Commits int `json:"commits"`
	// Changelog is the generated changelog entry (Markdown or configured format).
	Changelog string `json:"changelog,omitempty"`
}

func newChangelogCommand(configFile *string, outputFormat *string) *cobra.Command {
	var write bool
	var since string
	cmd := &cobra.Command{
		Use:   "changelog",
		Short: "Generate changelog from unreleased commits without performing a release",
		Long: `changelog analyses unreleased commits and generates release notes without
creating a tag, pushing anything, or invoking provider/hook plugins.

Examples:
  # Preview unreleased changes on stdout
  semrel changelog

  # Write/prepend the new entry to CHANGELOG.md
  semrel changelog --write

  # Start from a specific tag instead of the detected last tag
  semrel changelog --since v1.0.0

  # Machine-readable JSON output
  semrel changelog --output json

Exit codes:
  0 — unreleased releasable commits found (changelog generated)
  2 — nothing to release (no releasable commits)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChangelog(cmd.Context(), *configFile, *outputFormat, write, since)
		},
	}
	cmd.Flags().BoolVar(&write, "write", false,
		"Prepend the generated entry to CHANGELOG.md (or the file specified by SEMREL_PLUGIN_CHANGELOG_FILE)")
	cmd.Flags().StringVar(&since, "since", "",
		"Generate changelog since this tag/ref instead of auto-detecting the last release tag")
	return cmd
}

func runChangelog(ctx context.Context, configFile string, outputFormat string, write bool, since string) error {
	cfgPath, err := resolveConfigFile(configFile)
	if err != nil {
		return err
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	repo, err := gitpkg.OpenRepository(".")
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}

	// Resolve the "since" tag.
	var lastTag string
	if since != "" {
		lastTag = since
	} else {
		lt, err := repo.LastTag(ctx)
		if err != nil {
			return fmt.Errorf("getting last tag: %w", err)
		}
		lastTag = lt
	}

	rawMessages, err := repo.CommitsSince(ctx, lastTag)
	if err != nil {
		return fmt.Errorf("getting commits: %w", err)
	}

	currentVersion, err := semver.ParseVersion(strings.TrimPrefix(lastTag, cfg.TagPrefix))
	if err != nil {
		currentVersion = &semver.Version{}
	}
	currentTag := cfg.TagPrefix + currentVersion.String()

	if len(rawMessages) == 0 {
		return emitNoChangelog(outputFormat, currentTag, 0, 2)
	}

	// Parse and analyse commits.
	parser := commits.NewParser()
	parsed := parser.ParseAll(rawMessages)

	ruleMap := make(map[string]string)
	for _, r := range cfg.Rules {
		ruleMap[r.Type] = r.Bump
	}

	var hasFeat, hasFix, hasBreaking bool
	var commitTypes []string
	for _, c := range parsed {
		if c.IsBreakingChange {
			hasBreaking = true
		}
		if c.Type != "" {
			commitTypes = append(commitTypes, c.Type)
		}
		switch c.Type {
		case "feat":
			hasFeat = true
		case "fix", "perf", "revert":
			hasFix = true
		}
	}

	bump := semver.BumpFromRules(commitTypes, ruleMap, hasBreaking)
	if bump == "" {
		return emitNoChangelog(outputFormat, currentTag, len(parsed), 2)
	}

	calc := semver.NewCalculator()
	nextVer := calc.NextVersion(currentVersion, hasFeat, hasFix, hasBreaking)
	if nextVer == nil {
		return emitNoChangelog(outputFormat, currentTag, len(parsed), 2)
	}
	nextTag := cfg.TagPrefix + nextVer.String()

	gen := changelog.NewGenerator()
	entry := gen.Generate(nextTag, parsed)

	if write {
		if err := writeChangelogEntry(entry, cfg); err != nil {
			return fmt.Errorf("writing changelog: %w", err)
		}
	}

	summary := ChangelogSummary{
		HasUnreleased:  true,
		CurrentVersion: currentTag,
		NextVersion:    nextTag,
		Bump:           bump,
		Commits:        len(parsed),
		Changelog:      entry,
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(summary)
	}

	// Human-readable output.
	fmt.Printf("Current version : %s\n", currentTag)
	fmt.Printf("Next version    : %s (%s bump)\n", nextTag, bump)
	fmt.Printf("Commits         : %d (breaking=%v feat=%v fix=%v)\n",
		len(parsed), hasBreaking, hasFeat, hasFix)
	if write {
		changelogFile := changelogFilePath(cfg)
		fmt.Printf("Written to      : %s\n", changelogFile)
	}
	fmt.Printf("\n%s", entry)
	return nil
}

// emitNoChangelog prints an appropriate "nothing to release" message and returns an
// error with the supplied exit code embedded via exitCodeError.
func emitNoChangelog(outputFormat string, currentTag string, numCommits int, code int) error {
	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(ChangelogSummary{
			HasUnreleased:  false,
			CurrentVersion: currentTag,
			Commits:        numCommits,
		})
	} else {
		fmt.Println("No releasable commits found — nothing to changelog.")
	}
	return &exitCodeError{code: code, msg: "no releasable commits"}
}

// exitCodeError is an error that carries an explicit process exit code.
// Cobra translates non-nil RunE errors to exit 1 by default; we use this
// wrapper so that `semrel changelog` can exit 2 for "nothing to release",
// matching the behaviour of `semrel release --dry-run`.
type exitCodeError struct {
	code int
	msg  string
}

func (e *exitCodeError) Error() string { return e.msg }
func (e *exitCodeError) ExitCode() int { return e.code }

// changelogFilePath returns the target changelog file, honouring SEMREL_PLUGIN_CHANGELOG_FILE.
func changelogFilePath(cfg *config.Config) string {
	if v := os.Getenv("SEMREL_PLUGIN_CHANGELOG_FILE"); v != "" {
		return v
	}
	return "CHANGELOG.md"
}

// writeChangelogEntry prepends the new entry to the changelog file.
func writeChangelogEntry(entry string, cfg *config.Config) error {
	file := changelogFilePath(cfg)
	return prependChangelog(file, entry)
}
