// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SemRels/semrel/internal/colors"
	"github.com/SemRels/semrel/pkg/changelog"
	"github.com/SemRels/semrel/pkg/cioutput"
	"github.com/SemRels/semrel/pkg/commits"
	"github.com/SemRels/semrel/pkg/config"
	"github.com/SemRels/semrel/pkg/editor"
	"github.com/SemRels/semrel/pkg/envfile"
	gitpkg "github.com/SemRels/semrel/pkg/git"
	"github.com/SemRels/semrel/pkg/lock"
	"github.com/SemRels/semrel/pkg/plugininstance"
	"github.com/SemRels/semrel/pkg/semver"
)

// ReleaseSummary holds the structured output of a release run.
// See: https://github.com/SemRels/semrel/issues/20
type ReleaseSummary struct {
	Released       bool     `json:"released"`
	DryRun         bool     `json:"dry_run"`
	CurrentVersion string   `json:"current_version"`
	NextVersion    string   `json:"next_version,omitempty"`
	Bump           string   `json:"bump,omitempty"`
	Commits        int      `json:"commits"`
	Breaking       bool     `json:"breaking"`
	Features       bool     `json:"features"`
	Fixes          bool     `json:"fixes"`
	ForcedPatch    bool     `json:"forced_patch,omitempty"`
	Changelog      string   `json:"changelog,omitempty"`
	CommitMessages []string `json:"commit_messages,omitempty"`
	Branch         string   `json:"branch"`
	TagPrefix      string   `json:"tag_prefix"`
	CeilingApplied bool     `json:"ceiling_applied,omitempty"`
	VersionCeiling string   `json:"version_ceiling,omitempty"`
}

// printSummary writes the summary to stdout in the selected format.
func printSummary(s ReleaseSummary, format string) error {
	if format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(s)
	}
	// default: text
	if !s.Released {
		return nil
	}
	fmt.Printf("Current version : %s\n", colors.Cyan(s.CurrentVersion))
	fmt.Printf("Next version    : %s\n", colors.Bold(colors.Green(s.NextVersion)))
	fmt.Printf("Bump type       : %s\n", colors.Yellow(s.Bump))
	fmt.Printf("Commits         : %d (breaking=%v feat=%v fix=%v)\n",
		s.Commits, s.Breaking, s.Features, s.Fixes)
	fmt.Printf("\n%s", s.Changelog)
	return nil
}

func writeCIOutputs(summary ReleaseSummary, githubOutput bool, gitlabDotenv string, outputFile string) error {
	meta := cioutput.ReleaseMeta{
		Released:        summary.Released,
		DryRun:          summary.DryRun,
		Bump:            summary.Bump,
		PreviousVersion: strings.TrimPrefix(summary.CurrentVersion, summary.TagPrefix),
		Changelog:       summary.Changelog,
		Branch:          summary.Branch,
		CeilingApplied:  summary.CeilingApplied,
	}
	if summary.Released {
		meta.Version = strings.TrimPrefix(summary.NextVersion, summary.TagPrefix)
		meta.Tag = summary.NextVersion
	}
	if githubOutput {
		if err := cioutput.WriteGitHubOutput(meta); err != nil {
			return err
		}
	}
	if gitlabDotenv != "" {
		if err := cioutput.WriteGitLabDotenv(meta, gitlabDotenv); err != nil {
			return err
		}
	}
	if outputFile != "" {
		if err := cioutput.WriteOutputFile(meta, outputFile); err != nil {
			return err
		}
	}
	return nil
}

func releaseBumpLabel(current, next *semver.Version) string {
	switch {
	case next == nil || current == nil:
		return ""
	case next.Major != current.Major:
		return "major"
	case next.Minor != current.Minor:
		return "minor"
	default:
		return "patch"
	}
}

// version is set via ldflags at build time, or read from the embedded module info.
var version = func() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	return "dev"
}()

// NewRootCommand returns the root cobra command for semrel.
func NewRootCommand() *cobra.Command {
	var dryRun bool
	var configFile string
	var outputFormat string
	var noColor bool
	var envFile string

	root := &cobra.Command{
		Use:   "semrel",
		Short: "A Go-based semantic release system with plugin architecture",
		Long: `semrel automates software releases by analysing Conventional Commits,
determining the next SemVer version, generating changelogs and invoking
configurable release plugins (git tags, GitHub/GitLab Releases, npm, Docker, Helm, ...).

Configuration: auto-detected from .semrel.yaml, .semrel.yml, .semrel.toml, .semrel.json (override with --config)
Env file:      .env          (override with --env-file, set to "" to disable)
Documentation: https://github.com/SemRels/semrel

Quick start:
  semrel config init        — create .semrel.yaml interactively
  semrel doctor             — verify your setup before the first release
  semrel release --dry-run  — preview the next release without making changes
  semrel release            — run the full release pipeline`,
		Version:      version,
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if noColor {
				colors.Disable()
			}
			if envFile != "" {
				if err := envfile.Load(envFile); err != nil {
					fmt.Fprintf(os.Stderr, "warning: could not load env file %q: %v\n", envFile, err)
				}
			}
		},
	}

	root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Simulate the release without making any changes")
	root.PersistentFlags().StringVar(&configFile, "config", "", "Path to configuration file (auto-detected if not set: .semrel.yaml, .semrel.yml, .semrel.toml, .semrel.json)")
	root.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text",
		"Output format: text or json")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable coloured terminal output")
	root.PersistentFlags().StringVar(&envFile, "env-file", ".env",
		"Path to .env file to load before release (set to \"\" to disable)")

	root.AddCommand(newReleaseCommand(&dryRun, &configFile, &outputFormat))
	root.AddCommand(newLintCommand(&configFile, &outputFormat))
	root.AddCommand(newCommitlintCommand(&outputFormat))
	root.AddCommand(newPluginCommand())
	root.AddCommand(newDoctorCommand(&configFile, &outputFormat))
	root.AddCommand(newChangelogCommand(&configFile, &outputFormat))
	root.AddCommand(newConfigCommand(&configFile, &outputFormat))
	root.AddCommand(newMigrateCommand(&configFile))

	return root
}

func newReleaseCommand(dryRun *bool, configFile *string, outputFormat *string) *cobra.Command {
	var forcePatch bool
	var editNotes bool
	var interactive bool
	var githubOutput bool
	var gitlabDotenv string
	var outputFile string
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Run the full release pipeline",
		Long: `Run the full semrel release pipeline.

Pipeline steps:
  1. Load and validate .semrel.yaml
  2. Run condition-phase plugins (gates — abort the release on failure)
  3. Check the current branch is configured for release
  4. Find the last git tag and collect commits since then
  5. Parse commits against Conventional Commits rules
  6. Calculate the next SemVer version bump (major / minor / patch)
  7. Generate changelog / release notes
  8. Run pre-tag plugins (e.g. version file updaters)
  9. Commit CHANGELOG.md (unless commit_changelog: false)
 10. Create and push the git tag
 11. Run release-phase plugins (providers, hooks, package publishers)

Examples:
  # Dry-run — preview the release without making any changes
  semrel release --dry-run

  # Review/edit release notes in $EDITOR before tagging
  semrel release --edit

  # Pause for confirmation before tagging (not for CI)
  semrel release --interactive

  # Emit metadata to $GITHUB_OUTPUT (GitHub Actions)
  semrel release --github-output

Exit codes:
  0 — release created, or nothing to release on a non-release branch
  1 — error (config invalid, git error, plugin failure, …)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRelease(cmd.Context(), *dryRun, *configFile, forcePatch, editNotes, interactive, *outputFormat, githubOutput, gitlabDotenv, outputFile)
		},
	}
	cmd.Flags().BoolVar(&forcePatch, "force-bump-patch-version", false,
		"Force a patch release even when no releasable commits are found")
	cmd.Flags().BoolVar(&editNotes, "edit", false,
		"Open the generated release notes in $EDITOR before finalising the release")
	cmd.Flags().BoolVar(&interactive, "interactive", false,
		"Pause before tagging and prompt for confirmation (requires a TTY; not for CI)")
	cmd.Flags().BoolVar(&githubOutput, "github-output", false, "Write release metadata to $GITHUB_OUTPUT (GitHub Actions)")
	cmd.Flags().StringVar(&gitlabDotenv, "gitlab-dotenv", "", "Write release metadata as dotenv artifact (GitLab CI)")
	cmd.Flags().StringVar(&outputFile, "output-file", "", "Write release metadata to file (.json or .env)")
	return cmd
}

func runRelease(ctx context.Context, dryRun bool, configFile string, forcePatch bool, editNotes bool, interactive bool, outputFormat string, githubOutput bool, gitlabDotenv string, outputFile string) error {
	if dryRun {
		_, _ = fmt.Fprintln(os.Stdout, "╔══════════════════════════════════════╗")
		_, _ = fmt.Fprintln(os.Stdout, "║          DRY RUN — no changes        ║")
		_, _ = fmt.Fprintln(os.Stdout, "╚══════════════════════════════════════╝")
	}

	configFile, err := resolveConfigFile(configFile)
	if err != nil {
		return err
	}

	// 1. Load config
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// 1a. Warn if config schema is outdated.
	if !config.IsUpToDate(cfg) && outputFormat != "json" {
		fmt.Fprintf(os.Stderr, "warning: .semrel.yaml is at schema version %d; current version is %d — run `semrel migrate` to upgrade\n",
			cfg.SchemaVersion, config.CurrentSchemaVersion)
	}

	// 2. Open repository
	repo, err := gitpkg.OpenRepository(".")
	if err != nil {
		return fmt.Errorf("opening repository: %w", err)
	}

	// Acquire release lock to prevent concurrent releases (skip in dry-run)
	// See: https://github.com/SemRels/semrel/issues/46
	if !dryRun {
		rl := lock.New(repo.Path)
		if err := rl.Acquire(""); err != nil {
			return fmt.Errorf("acquiring release lock: %w", err)
		}
		defer func() {
			if releaseErr := rl.Release(); releaseErr != nil {
				fmt.Fprintf(os.Stderr, "warning: could not release lock: %v\n", releaseErr)
			}
		}()
	}

	// 2a. Run condition-phase plugins — these gate the release before any git work.
	// A failing condition plugin aborts the release immediately with a clear error.
	if condSpecs := pluginSpecsForPhase(cfg.Plugins, "condition"); len(condSpecs) > 0 {
		if outputFormat != "json" {
			fmt.Println("Running condition checks…")
		}
		orch := plugininstance.NewOrchestrator(makePluginRunner(dryRun, ReleaseSummary{}))
		if err := orch.Run(ctx, condSpecs); err != nil {
			return fmt.Errorf("condition check failed: %w", err)
		}
		if outputFormat != "json" {
			fmt.Println(colors.Success("All conditions passed."))
		}
	}

	// 3. Get current branch
	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}

	// Check if the current branch is configured for release
	if !isBranchConfigured(branch, cfg.Branches) {
		summary := ReleaseSummary{Released: false, DryRun: dryRun, Branch: branch, TagPrefix: cfg.TagPrefix}
		if err := writeCIOutputs(summary, githubOutput, gitlabDotenv, outputFile); err != nil {
			return err
		}
		msg := fmt.Sprintf("Branch %q is not configured for release — skipping.", branch)
		if outputFormat == "json" {
			return printSummary(summary, outputFormat)
		}
		fmt.Println(msg)
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

	currentVersion, err := semver.ParseVersion(strings.TrimPrefix(lastTag, cfg.TagPrefix))
	if err != nil {
		currentVersion = &semver.Version{}
	}
	currentTag := cfg.TagPrefix + currentVersion.String()

	if len(rawMessages) == 0 && !forcePatch {
		summary := ReleaseSummary{
			Released:       false,
			DryRun:         dryRun,
			CurrentVersion: currentTag,
			Branch:         branch,
			TagPrefix:      cfg.TagPrefix,
		}
		if err := writeCIOutputs(summary, githubOutput, gitlabDotenv, outputFile); err != nil {
			return err
		}
		if outputFormat == "json" {
			return printSummary(summary, outputFormat)
		}
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
		summary := ReleaseSummary{
			Released:       false,
			DryRun:         dryRun,
			CurrentVersion: currentTag,
			Commits:        len(parsed),
			Branch:         branch,
			TagPrefix:      cfg.TagPrefix,
		}
		if err := writeCIOutputs(summary, githubOutput, gitlabDotenv, outputFile); err != nil {
			return err
		}
		if outputFormat == "json" {
			return printSummary(summary, outputFormat)
		}
		fmt.Println("No releasable commits found — nothing to release.")
		return nil
	}
	if bump == "" && forcePatch {
		bump = "patch"
	}

	// 7. Calculate next version
	calc := semver.NewCalculator()
	nextVer := calc.NextVersion(currentVersion, hasFeat, hasFix, hasBreaking)
	forcedPatch := false
	if nextVer == nil {
		if forcePatch {
			nextVer = calc.ForcePatch(currentVersion)
			forcedPatch = true
			if outputFormat != "json" {
				fmt.Println("[force-bump-patch-version] No releasable commits — forcing patch bump.")
			}
		} else {
			summary := ReleaseSummary{
				Released:       false,
				DryRun:         dryRun,
				CurrentVersion: currentTag,
				Commits:        len(parsed),
				Branch:         branch,
				TagPrefix:      cfg.TagPrefix,
			}
			if err := writeCIOutputs(summary, githubOutput, gitlabDotenv, outputFile); err != nil {
				return err
			}
			if outputFormat == "json" {
				return printSummary(summary, outputFormat)
			}
			fmt.Println("No releasable commits found — nothing to release.")
			return nil
		}
	}

	nextTag := cfg.TagPrefix + nextVer.String()
	ceilingApplied := false

	// 7b. Apply version ceiling if configured
	if cfg.VersionCeiling != "" {
		ceiling, err := semver.ParseVersion(cfg.VersionCeiling)
		if err != nil {
			return fmt.Errorf("invalid version_ceiling: %w", err)
		}
		strategy := cfg.CeilingStrategy
		if strategy == "" {
			strategy = "clamp"
		}
		clamped, skipped, err := semver.ApplyCeiling(currentVersion, nextVer, ceiling, strategy)
		if err != nil {
			return err
		}
		if skipped {
			summary := ReleaseSummary{
				Released:       false,
				DryRun:         dryRun,
				CurrentVersion: currentTag,
				Commits:        len(parsed),
				Branch:         branch,
				TagPrefix:      cfg.TagPrefix,
				VersionCeiling: cfg.VersionCeiling,
			}
			if err := writeCIOutputs(summary, githubOutput, gitlabDotenv, outputFile); err != nil {
				return err
			}
			msg := fmt.Sprintf("Version ceiling %s reached — skipping release (calculated: %s).", cfg.VersionCeiling, nextTag)
			if outputFormat == "json" {
				return printSummary(summary, outputFormat)
			}
			fmt.Println(msg)
			return nil
		}
		if clamped != nil && clamped.String() != nextVer.String() {
			fmt.Fprintf(os.Stderr, "warning: version ceiling %s applied — bump clamped from %s to %s\n", cfg.VersionCeiling, nextVer.String(), clamped.String())
			ceilingApplied = true
			nextVer = clamped
			nextTag = cfg.TagPrefix + nextVer.String()
			bump = releaseBumpLabel(currentVersion, nextVer)
		}
	}

	// 8. Generate changelog
	gen := changelog.NewGenerator()
	changelogEntry := gen.Generate(nextTag, parsed)

	// 8a. Optionally open the changelog in an editor for manual review
	// See: https://github.com/SemRels/semrel/issues/48
	if editNotes {
		ed := editor.New()
		edited, err := ed.Edit(changelogEntry)
		if err != nil {
			return fmt.Errorf("editing release notes: %w", err)
		}
		changelogEntry = edited
	}

	// 8b. Interactive confirmation prompt before tagging.
	// See: https://github.com/SemRels/semrel/issues/193
	if interactive {
		if !isTerminal() {
			return fmt.Errorf("--interactive requires a TTY; stdin is not a terminal (CI detected)")
		}
		confirmed, err := interactiveConfirm(os.Stdin, os.Stderr, currentTag, nextTag, bump, changelogEntry, dryRun)
		if err != nil {
			return err
		}
		// If the user edited the tag, update nextTag only (nextVer not used after this point).
		if confirmed != nextTag {
			nextTag = confirmed
		}
	}

	summary := ReleaseSummary{
		Released:       true,
		DryRun:         dryRun,
		CurrentVersion: currentTag,
		NextVersion:    nextTag,
		Bump:           bump,
		Commits:        len(parsed),
		Breaking:       hasBreaking,
		Features:       hasFeat,
		Fixes:          hasFix,
		ForcedPatch:    forcedPatch,
		Changelog:      changelogEntry,
		CommitMessages: rawMessages,
		Branch:         branch,
		TagPrefix:      cfg.TagPrefix,
		CeilingApplied: ceilingApplied,
		VersionCeiling: cfg.VersionCeiling,
	}

	// 9. Output results
	if outputFormat != "json" {
		if err := printSummary(summary, outputFormat); err != nil {
			return err
		}
	}

	if dryRun {
		if outputFormat != "json" {
			fmt.Println("\n[dry-run] Would perform:")
			fmt.Printf("  • prepend entry to CHANGELOG.md\n")
			if cfg.ShouldCommitChangelog() {
				fmt.Println("  • git commit CHANGELOG.md")
			}
			fmt.Printf("  • git tag %s\n", nextTag)
		}
		// Still run plugins in dry-run so they can report what they would do.
		// Each plugin receives SEMREL_DRY_RUN=true and is expected to exit 0.
		if len(cfg.Plugins) > 0 {
			orchestrator := plugininstance.NewOrchestrator(makePluginRunner(dryRun, summary))
			if err := orchestrator.Run(ctx, pluginSpecsFromConfig(cfg.Plugins)); err != nil {
				return err
			}
		}
		if outputFormat != "json" {
			fmt.Println("\n[dry-run] No changes made.")
		}
		if err := writeCIOutputs(summary, githubOutput, gitlabDotenv, outputFile); err != nil {
			return err
		}
		if outputFormat == "json" {
			return printSummary(summary, outputFormat)
		}
		return nil
	}

	// 10. Check if the tag already exists (idempotency / repair mode)
	tagExists, err := repo.TagExists(ctx, nextTag)
	if err != nil {
		return fmt.Errorf("checking tag: %w", err)
	}
	if tagExists {
		strategy := cfg.ResolvedTagExistsStrategy()
		switch strategy {
		case "error":
			return fmt.Errorf("tag %s already exists — aborting (tag_exists_strategy=error)", nextTag)
		case "skip":
			if outputFormat != "json" {
				fmt.Printf("Tag %s already exists — skipping release (tag_exists_strategy=skip).\n", nextTag)
			}
			summary.Released = false
			return writeCIOutputs(summary, githubOutput, gitlabDotenv, outputFile)
		default: // "update-changelog"
			if outputFormat != "json" {
				fmt.Printf("Tag %s already exists — updating CHANGELOG.md only (tag_exists_strategy=update-changelog).\n", nextTag)
			}
			if err := prependChangelog("CHANGELOG.md", changelogEntry); err != nil {
				_, _ = fmt.Fprintln(os.Stderr, colors.Warning(fmt.Sprintf("could not update CHANGELOG.md: %v", err)))
			} else {
				if cfg.ShouldCommitChangelog() {
					msg := fmt.Sprintf("chore(changelog): update for %s [skip ci]", nextTag)
					if err := repo.CommitFiles(ctx, []string{"CHANGELOG.md"}, msg); err != nil {
						_, _ = fmt.Fprintln(os.Stderr, colors.Warning(fmt.Sprintf("could not commit CHANGELOG.md: %v", err)))
					} else if outputFormat != "json" {
						fmt.Println(colors.Success("Committed CHANGELOG.md (tag already existed)"))
					}
				} else if outputFormat != "json" {
					fmt.Println(colors.Success("Updated CHANGELOG.md"))
				}
			}
			return writeCIOutputs(summary, githubOutput, gitlabDotenv, outputFile)
		}
	}

	// 11. Write CHANGELOG.md (prepend) and optionally commit it before tagging
	if err := prependChangelog("CHANGELOG.md", changelogEntry); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, colors.Warning(fmt.Sprintf("could not update CHANGELOG.md: %v", err)))
	} else {
		if cfg.ShouldCommitChangelog() {
			msg := fmt.Sprintf("chore(changelog): update for %s [skip ci]", nextTag)
			if err := repo.CommitFiles(ctx, []string{"CHANGELOG.md"}, msg); err != nil {
				_, _ = fmt.Fprintln(os.Stderr, colors.Warning(fmt.Sprintf("could not commit CHANGELOG.md: %v", err)))
			} else if outputFormat != "json" {
				fmt.Println(colors.Success("Committed CHANGELOG.md"))
			}
		} else if outputFormat != "json" {
			fmt.Println(colors.Success("Updated CHANGELOG.md"))
		}
	}

	// 11a. Run pre-tag plugins (e.g. updater-go) — these run AFTER version
	//      calculation but BEFORE the git tag is created, so the tagged commit can include
	//      any version-file changes committed by the plugins.
	if preTagSpecs := pluginSpecsForPhase(cfg.Plugins, "pre-tag"); len(preTagSpecs) > 0 {
		if !dryRun {
			orch := plugininstance.NewOrchestrator(makePluginRunner(dryRun, summary))
			if err := orch.Run(ctx, preTagSpecs); err != nil {
				return err
			}
			// Auto-commit any tracked files modified by pre-tag plugins (e.g. version.go).
			// The tag will then point to this commit so `go install @vX.Y.Z` embeds the version.
			msg := fmt.Sprintf("chore(release): set version to %s [skip ci]", summary.NextVersion)
			if committed, err := repo.CommitModifiedTrackedFiles(ctx, msg); err != nil {
				_, _ = fmt.Fprintln(os.Stderr, colors.Warning(fmt.Sprintf("could not commit pre-tag changes: %v", err)))
			} else if committed && outputFormat != "json" {
				fmt.Println(colors.Success(fmt.Sprintf("Committed pre-tag version files (%s)", summary.NextVersion)))
			}
		} else if outputFormat != "json" {
			fmt.Println(colors.Warning("[dry-run] skipping pre-tag plugins"))
		}
	}

	// 12. Create git tag (on the CHANGELOG/version-bump commit)
	if err := repo.CreateTag(ctx, nextTag, "Release "+nextTag); err != nil {
		return fmt.Errorf("creating tag: %w", err)
	}
	if outputFormat != "json" {
		fmt.Println(colors.Success(fmt.Sprintf("Created tag %s", colors.Bold(nextTag))))
	}

	// 13. Run configured release-phase plugins
	if len(cfg.Plugins) > 0 {
		orchestrator := plugininstance.NewOrchestrator(makePluginRunner(dryRun, summary))
		if err := orchestrator.Run(ctx, pluginSpecsFromConfig(cfg.Plugins)); err != nil {
			return err
		}
	}

	if err := writeCIOutputs(summary, githubOutput, gitlabDotenv, outputFile); err != nil {
		return err
	}
	if outputFormat == "json" {
		return printSummary(summary, outputFormat)
	}
	return nil
}

func pluginSpecsFromConfig(plugins []config.PluginConfig) []plugininstance.PluginSpec {
	return pluginSpecsForPhase(plugins, "release")
}

// pluginSpecsForPhase returns specs for plugins matching the given phase.
// Phase "release" also includes plugins with no phase set (the default).
func pluginSpecsForPhase(plugins []config.PluginConfig, phase string) []plugininstance.PluginSpec {
	specs := make([]plugininstance.PluginSpec, 0, len(plugins))
	for _, p := range plugins {
		effective := p.Phase
		if effective == "" {
			effective = "release"
		}
		if effective != phase {
			continue
		}
		specs = append(specs, plugininstance.PluginSpec{
			Uses:   p.Uses,
			Path:   p.Path,
			Name:   p.Name,
			Config: p.Args,
		})
	}
	return specs
}

func makePluginRunner(dryRun bool, rel ReleaseSummary) plugininstance.Runner {
	return func(ctx context.Context, spec plugininstance.PluginSpec) error {
		binPath, err := resolvePluginBinary(spec)
		if err != nil {
			fmt.Printf("⚠  plugin %q not installed (run: semrel plugin install %s)\n", spec.Uses, spec.Uses)
			return nil
		}

		cmd := exec.CommandContext(ctx, binPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		env := os.Environ()
		// Release context — available to all plugin binaries as SEMREL_* env vars.
		// Variable names match the documented plugin contract in docs/plugin-development.md.
		dryRunStr := "false"
		if dryRun {
			dryRunStr = "true"
		}
		env = append(env,
			"SEMREL_CURRENT_VERSION="+rel.CurrentVersion,
			"SEMREL_VERSION="+rel.NextVersion,
			"SEMREL_TAG_NAME="+rel.NextVersion,
			"SEMREL_NEXT_VERSION="+rel.NextVersion,
			"SEMREL_BUMP="+rel.Bump,
			"SEMREL_TAG_PREFIX="+rel.TagPrefix,
			"SEMREL_CHANGELOG="+rel.Changelog,
			"SEMREL_BRANCH="+rel.Branch,
			"SEMREL_DRY_RUN="+dryRunStr,
		)
		if len(rel.CommitMessages) > 0 {
			if commitsJSON, err := json.Marshal(rel.CommitMessages); err == nil {
				env = append(env, "SEMREL_COMMITS="+string(commitsJSON))
			}
		}
		// Plugin-specific config from .semrel.yaml args.
		for k, v := range spec.Config {
			env = append(env, fmt.Sprintf("SEMREL_PLUGIN_%s=%v", pluginEnvKey(k), v))
		}
		cmd.Env = env
		return cmd.Run()
	}
}

func resolvePluginBinary(spec plugininstance.PluginSpec) (string, error) {
	if spec.Path != "" {
		return spec.Path, nil
	}

	binaryName := pluginBinaryName(spec.Uses)
	if binaryName == "" {
		return "", fmt.Errorf("plugin binary name is empty")
	}

	home, err := os.UserHomeDir()
	if err == nil {
		local := filepath.Join(home, ".semrel", "plugins", binaryName)
		for _, candidate := range localBinaryCandidates(local) {
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				return candidate, nil
			}
		}
	}

	return exec.LookPath(binaryName)
}

func pluginBinaryName(uses string) string {
	uses = strings.TrimSpace(uses)
	uses = strings.TrimPrefix(uses, "semrel-plugin-")
	if idx := strings.LastIndex(uses, "/"); idx >= 0 {
		uses = uses[idx+1:]
	}
	if idx := strings.Index(uses, "@"); idx >= 0 {
		uses = uses[:idx]
	}
	if uses == "" {
		return ""
	}
	return "semrel-plugin-" + uses
}

func localBinaryCandidates(base string) []string {
	candidates := []string{base}
	for _, ext := range strings.Split(strings.ToLower(os.Getenv("PATHEXT")), ";") {
		ext = strings.TrimSpace(ext)
		if ext == "" {
			continue
		}
		candidates = append(candidates, base+strings.ToLower(ext))
		candidates = append(candidates, base+ext)
	}
	return candidates
}

func pluginEnvKey(key string) string {
	key = strings.ToUpper(key)
	replacer := strings.NewReplacer("-", "_", ".", "_", " ", "_")
	return replacer.Replace(key)
}

func resolveConfigFile(configFile string) (string, error) {
	if configFile != "" {
		return configFile, nil
	}

	found, err := config.FindConfigFile(".")
	if err != nil {
		return "", fmt.Errorf("no config file found: %w", err)
	}
	return found, nil
}

func newLintCommand(configFile *string, outputFormat *string) *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Validate commit messages since the last release",
		Long: `Validate all commit messages since the last release tag against the
Conventional Commits specification (https://www.conventionalcommits.org/).

Reads every commit reachable since the last semver tag and reports any that
do not conform. Useful as a pre-merge CI gate to catch bad commit messages
before they block a release.

Examples:
  semrel lint
  semrel lint --output json

Exit codes:
  0 — all commits are valid
  1 — one or more commits are invalid`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLint(cmd.Context(), *configFile, *outputFormat)
		},
	}
}

// LintSummary holds the structured output of a lint run.
type LintSummary struct {
	Valid   bool     `json:"valid"`
	Total   int      `json:"total"`
	Invalid []string `json:"invalid,omitempty"`
}

func runLint(ctx context.Context, configFile string, outputFormat string) error {
	configFile, err := resolveConfigFile(configFile)
	if err != nil {
		return err
	}
	_ = configFile

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
		if outputFormat == "json" {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(LintSummary{Valid: false, Total: len(rawMessages), Invalid: invalid})
		} else {
			fmt.Fprintf(os.Stderr, "%s\n", colors.Error(fmt.Sprintf("Found %d non-conventional commit(s):", len(invalid))))
			for _, m := range invalid {
				fmt.Fprintf(os.Stderr, "  %s %s\n", colors.Red("•"), m)
			}
		}
		return fmt.Errorf("lint failed: %d non-conventional commit(s)", len(invalid))
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(LintSummary{Valid: true, Total: len(rawMessages)})
	}
	fmt.Println(colors.Success(fmt.Sprintf("All %d commit(s) follow Conventional Commits.", len(rawMessages))))
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

// CommitlintResult holds the result of linting a single commit message.
type CommitlintResult struct {
	Message string `json:"message"`
	Valid   bool   `json:"valid"`
	Error   string `json:"error,omitempty"`
}

// CommitlintSummary is the structured output for the commitlint command.
// See: https://github.com/SemRels/semrel/issues/47
type CommitlintSummary struct {
	Valid   bool               `json:"valid"`
	Total   int                `json:"total"`
	Passed  int                `json:"passed"`
	Failed  int                `json:"failed"`
	Results []CommitlintResult `json:"results,omitempty"`
}

// newCommitlintCommand returns the commitlint sub-command.
// It validates one or more commit messages against the Conventional Commits spec.
//
// Usage:
//
//	semrel commitlint "feat: add feature"
//	echo "fix: typo" | semrel commitlint --stdin
//	semrel commitlint --from HEAD~5 --to HEAD
func newCommitlintCommand(outputFormat *string) *cobra.Command {
	var fromRef, toRef string
	var stdin bool

	cmd := &cobra.Command{
		Use:   "commitlint [message...]",
		Short: "Validate commit messages against Conventional Commits",
		Long: `commitlint validates commit messages against the Conventional Commits
specification (https://www.conventionalcommits.org/).

Without arguments, lints all commits since the last release tag (same scope as semrel lint).

Examples:
  # Lint commits since the last release tag (default)
  semrel commitlint

  # Lint a single message passed as an argument
  semrel commitlint "feat(auth): add OAuth2 support"

  # Lint a commit range (HEAD~5 through HEAD)
  semrel commitlint --from HEAD~5 --to HEAD

  # Lint the PR head range in CI (GitHub Actions)
  semrel commitlint --from ${{ github.event.pull_request.base.sha }} --to ${{ github.event.pull_request.head.sha }}

  # Pipe a message from stdin
  echo "fix: typo" | semrel commitlint --stdin

Exit code 0 means all messages are valid; non-zero means at least one is invalid.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCommitlint(cmd.Context(), args, fromRef, toRef, stdin, *outputFormat)
		},
	}

	cmd.Flags().StringVar(&fromRef, "from", "", "Start ref (exclusive) for commit range")
	cmd.Flags().StringVar(&toRef, "to", "HEAD", "End ref (inclusive) for commit range (default: HEAD)")
	cmd.Flags().BoolVar(&stdin, "stdin", false, "Read a single commit message from stdin")

	return cmd
}

func runCommitlint(ctx context.Context, args []string, fromRef, toRef string, stdinMode bool, outputFormat string) error {
	parser := commits.NewParser()

	var messages []string

	switch {
	case stdinMode:
		// Read from stdin
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		msg := strings.TrimSpace(string(data))
		if msg != "" {
			messages = append(messages, msg)
		}
	case fromRef != "":
		// Read from git log range
		repo, err := gitpkg.OpenRepository(".")
		if err != nil {
			return fmt.Errorf("opening repository: %w", err)
		}
		msgs, err := repo.CommitsSince(ctx, fromRef)
		if err != nil {
			return fmt.Errorf("getting commits: %w", err)
		}
		messages = msgs
	case len(args) > 0:
		messages = args
	default:
		// No arguments: lint all commits since the last release tag.
		repo, err := gitpkg.OpenRepository(".")
		if err != nil {
			return fmt.Errorf("opening repository: %w", err)
		}
		lastTag, err := repo.LastTag(ctx)
		if err != nil {
			return fmt.Errorf("getting last tag: %w", err)
		}
		msgs, err := repo.CommitsSince(ctx, lastTag)
		if err != nil {
			return fmt.Errorf("getting commits: %w", err)
		}
		messages = msgs
	}

	if len(messages) == 0 {
		fmt.Println("No commit messages to lint.")
		return nil
	}

	var results []CommitlintResult
	for _, msg := range messages {
		firstLine := strings.SplitN(msg, "\n", 2)[0]
		c, err := parser.Parse(msg)
		r := CommitlintResult{Message: firstLine, Valid: true}
		if err != nil {
			r.Valid = false
			r.Error = err.Error()
		} else if c.Type == "" {
			r.Valid = false
			r.Error = "not a Conventional Commit (missing type)"
		}
		results = append(results, r)
	}

	passed := 0
	failed := 0
	for _, r := range results {
		if r.Valid {
			passed++
		} else {
			failed++
		}
	}

	summary := CommitlintSummary{
		Valid:   failed == 0,
		Total:   len(results),
		Passed:  passed,
		Failed:  failed,
		Results: results,
	}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(summary); err != nil {
			return fmt.Errorf("encoding JSON output: %w", err)
		}
	} else {
		for _, r := range results {
			if r.Valid {
				fmt.Println(colors.Success(fmt.Sprintf("  ✓ %s", r.Message)))
			} else {
				fmt.Println(colors.Error(fmt.Sprintf("  ✗ %s", r.Message)))
				if r.Error != "" {
					fmt.Printf("      %s\n", colors.Red(r.Error))
				}
			}
		}
		fmt.Println()
		if summary.Valid {
			fmt.Println(colors.Success(fmt.Sprintf("All %d commit(s) are valid.", summary.Total)))
		} else {
			fmt.Fprintf(os.Stderr, "%s\n",
				colors.Error(fmt.Sprintf("%d of %d commit(s) failed validation.", summary.Failed, summary.Total)))
		}
	}

	if !summary.Valid {
		return fmt.Errorf("commitlint failed: %d invalid commit(s)", summary.Failed)
	}
	return nil
}
