// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/GoSemantics/semrel/pkg/changelog"
	"github.com/GoSemantics/semrel/pkg/commits"
	"github.com/GoSemantics/semrel/pkg/config"
	gitpkg "github.com/GoSemantics/semrel/pkg/git"
	"github.com/GoSemantics/semrel/pkg/semver"
	"github.com/spf13/cobra"
)

// version is set via ldflags at build time.
var version = "dev"

// NewRootCommand returns the root cobra command for semrel.
func NewRootCommand() *cobra.Command {
	var dryRun bool
	var configFile string

	root := &cobra.Command{
		Use:   "semrel",
		Short: "A Go-based semantic release system with plugin architecture",
		Long: `semrel automates software releases by analysing Conventional Commits,
determining the next SemVer version, generating changelogs and invoking
configurable release plugins (git tags, GitHub/GitLab Releases, npm, Docker, Helm, ...).`,
		Version:      version,
		SilenceUsage: true,
	}

	root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Simulate the release without making any changes")
	root.PersistentFlags().StringVar(&configFile, "config", ".semrel.yaml", "Path to configuration file")

	root.AddCommand(newReleaseCommand(&dryRun, &configFile))
	root.AddCommand(newLintCommand(&configFile))

	return root
}

func newReleaseCommand(dryRun *bool, configFile *string) *cobra.Command {
	var forcePatch bool
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Run the full release pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRelease(cmd.Context(), *dryRun, *configFile, forcePatch)
		},
	}
	cmd.Flags().BoolVar(&forcePatch, "force-bump-patch-version", false,
		"Force a patch release even when no releasable commits are found")
	return cmd
}

func runRelease(ctx context.Context, dryRun bool, configFile string, forcePatch bool) error {
	if dryRun {
		fmt.Fprintln(os.Stdout, "╔══════════════════════════════════════╗")
		fmt.Fprintln(os.Stdout, "║          DRY RUN — no changes        ║")
		fmt.Fprintln(os.Stdout, "╚══════════════════════════════════════╝")
	}

	// 1. Load config
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// 2. Open repository
	repo, err := gitpkg.OpenRepository(".")
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}

	// 3. Get current branch
	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}

	// Check if the current branch is configured for release
	if !isBranchConfigured(branch, cfg.Branches) {
		fmt.Printf("Branch %q is not configured for release — skipping.\n", branch)
		return nil
	}

	// 4. Get last tag and commits since it
	lastTag, err := repo.LastTag(ctx)
	if err != nil {
		return fmt.Errorf("getting last tag: %w", err)
	}

	rawMessages, err := repo.CommitsSince(ctx, lastTag)
	if err != nil {
		return fmt.Errorf("getting commits: %w", err)
	}

	if len(rawMessages) == 0 && !forcePatch {
		fmt.Println("No commits since last release — nothing to release.")
		return nil
	}

	// 5. Parse commits
	parser := commits.NewParser()
	parsed := parser.ParseAll(rawMessages)

	// 6. Build rule map and analyse commits
	ruleMap := make(map[string]string)
	for _, r := range cfg.Rules {
		ruleMap[r.Type] = r.Bump
	}

	var (
		hasFeat, hasFix, hasBreaking bool
		commitTypes                  []string
	)
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
	if bump == "" && !forcePatch {
		fmt.Println("No releasable commits found — nothing to release.")
		return nil
	}
	if bump == "" && forcePatch {
		bump = "patch"
	}

	// 7. Calculate next version
	currentVersion, err := semver.ParseVersion(strings.TrimPrefix(lastTag, cfg.TagPrefix))
	if err != nil {
		currentVersion = &semver.Version{}
	}
	calc := semver.NewCalculator()
	nextVer := calc.NextVersion(currentVersion, hasFeat, hasFix, hasBreaking)
	if nextVer == nil {
		if forcePatch {
			nextVer = calc.ForcePatch(currentVersion)
			fmt.Println("[force-bump-patch-version] No releasable commits — forcing patch bump.")
		} else {
			fmt.Println("No releasable commits found — nothing to release.")
			return nil
		}
	}

	nextTag := cfg.TagPrefix + nextVer.String()

	// 8. Generate changelog
	gen := changelog.NewGenerator()
	changelogEntry := gen.Generate(nextTag, parsed)

	// 9. Output results
	fmt.Printf("Current version : %s%s\n", cfg.TagPrefix, currentVersion.String())
	fmt.Printf("Next version    : %s\n", nextTag)
	fmt.Printf("Bump type       : %s\n", bump)
	fmt.Printf("Commits         : %d (breaking=%v feat=%v fix=%v)\n",
		len(parsed), hasBreaking, hasFeat, hasFix)
	fmt.Printf("\n%s", changelogEntry)

	if dryRun {
		fmt.Println("\n[dry-run] Would perform:")
		fmt.Printf("  • git tag %s\n", nextTag)
		fmt.Println("  • prepend entry to CHANGELOG.md")
		fmt.Println("\n[dry-run] No changes made.")
		return nil
	}

	// 10. Create git tag
	if err := repo.CreateTag(ctx, nextTag, "Release "+nextTag); err != nil {
		return fmt.Errorf("creating tag: %w", err)
	}
	fmt.Printf("✓ Created tag %s\n", nextTag)

	// 11. Write CHANGELOG.md (prepend)
	if err := prependChangelog("CHANGELOG.md", changelogEntry); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update CHANGELOG.md: %v\n", err)
	} else {
		fmt.Println("✓ Updated CHANGELOG.md")
	}

	return nil
}

func newLintCommand(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Validate commit messages since the last release",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLint(cmd.Context(), *configFile)
		},
	}
}

func runLint(ctx context.Context, configFile string) error {
	repo, err := gitpkg.OpenRepository(".")
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}

	lastTag, err := repo.LastTag(ctx)
	if err != nil {
		return fmt.Errorf("getting last tag: %w", err)
	}

	rawMessages, err := repo.CommitsSince(ctx, lastTag)
	if err != nil {
		return fmt.Errorf("getting commits: %w", err)
	}

	parser := commits.NewParser()
	var invalid []string
	for _, msg := range rawMessages {
		c, err := parser.Parse(msg)
		if err != nil || c.Type == "" {
			// Trim to first line for display
			firstLine := strings.SplitN(msg, "\n", 2)[0]
			invalid = append(invalid, firstLine)
		}
	}

	if len(invalid) > 0 {
		fmt.Fprintf(os.Stderr, "Found %d non-conventional commit(s):\n", len(invalid))
		for _, m := range invalid {
			fmt.Fprintf(os.Stderr, "  - %s\n", m)
		}
		return fmt.Errorf("lint failed: %d non-conventional commit(s)", len(invalid))
	}

	fmt.Printf("✓ All %d commit(s) follow Conventional Commits.\n", len(rawMessages))
	return nil
}

// isBranchConfigured checks whether the given branch name is in the release branches list.
// Supports exact matches.
func isBranchConfigured(branch string, branches []config.BranchConfig) bool {
	if len(branches) == 0 {
		return true // no restriction configured
	}
	for _, b := range branches {
		if b.Name == branch {
			return true
		}
	}
	return false
}

// prependChangelog prepends content to a CHANGELOG.md file.
func prependChangelog(path, entry string) error {
	existing := ""
	data, err := os.ReadFile(path)
	if err == nil {
		existing = string(data)
	}
	return os.WriteFile(path, []byte(entry+existing), 0o644)
}
