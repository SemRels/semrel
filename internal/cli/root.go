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
	"strings"

	"github.com/GoSemantics/semrel/internal/colors"
	"github.com/GoSemantics/semrel/pkg/changelog"
	"github.com/GoSemantics/semrel/pkg/commits"
	"github.com/GoSemantics/semrel/pkg/config"
	"github.com/GoSemantics/semrel/pkg/editor"
	gitpkg "github.com/GoSemantics/semrel/pkg/git"
	"github.com/GoSemantics/semrel/pkg/lock"
	"github.com/GoSemantics/semrel/pkg/plugininstance"
	"github.com/GoSemantics/semrel/pkg/semver"
	"github.com/spf13/cobra"
)

// ReleaseSummary holds the structured output of a release run.
// See: https://github.com/SemRels/semrel/issues/20
type ReleaseSummary struct {
	Released       bool   `json:"released"`
	DryRun         bool   `json:"dry_run"`
	CurrentVersion string `json:"current_version"`
	NextVersion    string `json:"next_version,omitempty"`
	Bump           string `json:"bump,omitempty"`
	Commits        int    `json:"commits"`
	Breaking       bool   `json:"breaking"`
	Features       bool   `json:"features"`
	Fixes          bool   `json:"fixes"`
	ForcedPatch    bool   `json:"forced_patch,omitempty"`
	Changelog      string `json:"changelog,omitempty"`
	Branch         string `json:"branch"`
	TagPrefix      string `json:"tag_prefix"`
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

// version is set via ldflags at build time.
var version = "dev"

// NewRootCommand returns the root cobra command for semrel.
func NewRootCommand() *cobra.Command {
	var dryRun bool
	var configFile string
	var outputFormat string
	var noColor bool

	root := &cobra.Command{
		Use:   "semrel",
		Short: "A Go-based semantic release system with plugin architecture",
		Long: `semrel automates software releases by analysing Conventional Commits,
determining the next SemVer version, generating changelogs and invoking
configurable release plugins (git tags, GitHub/GitLab Releases, npm, Docker, Helm, ...).

Configuration: .semrel.yaml  (override with --config)
Documentation: https://github.com/SemRels/semrel`,
		Version:      version,
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if noColor {
				colors.Disable()
			}
		},
	}

	root.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Simulate the release without making any changes")
	root.PersistentFlags().StringVar(&configFile, "config", ".semrel.yaml", "Path to configuration file")
	root.PersistentFlags().StringVarP(&outputFormat, "output", "o", "text",
		"Output format: text or json")
	root.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable coloured terminal output")

	root.AddCommand(newReleaseCommand(&dryRun, &configFile, &outputFormat))
	root.AddCommand(newLintCommand(&configFile, &outputFormat))
	root.AddCommand(newCommitlintCommand(&outputFormat))

	return root
}

func newReleaseCommand(dryRun *bool, configFile *string, outputFormat *string) *cobra.Command {
	var forcePatch bool
	var editNotes bool
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Run the full release pipeline",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRelease(cmd.Context(), *dryRun, *configFile, forcePatch, editNotes, *outputFormat)
		},
	}
	cmd.Flags().BoolVar(&forcePatch, "force-bump-patch-version", false,
		"Force a patch release even when no releasable commits are found")
	cmd.Flags().BoolVar(&editNotes, "edit", false,
		"Open the generated release notes in $EDITOR before finalising the release")
	return cmd
}

func runRelease(ctx context.Context, dryRun bool, configFile string, forcePatch bool, editNotes bool, outputFormat string) error {
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

	// 3. Get current branch
	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}

	// Check if the current branch is configured for release
	if !isBranchConfigured(branch, cfg.Branches) {
		msg := fmt.Sprintf("Branch %q is not configured for release — skipping.", branch)
		if outputFormat == "json" {
			return printSummary(ReleaseSummary{Released: false, Branch: branch, TagPrefix: cfg.TagPrefix}, outputFormat)
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
		if outputFormat == "json" {
			return printSummary(ReleaseSummary{
				Released:       false,
				DryRun:         dryRun,
				CurrentVersion: currentTag,
				Branch:         branch,
				TagPrefix:      cfg.TagPrefix,
			}, outputFormat)
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
		if outputFormat == "json" {
			return printSummary(ReleaseSummary{
				Released:       false,
				DryRun:         dryRun,
				CurrentVersion: currentTag,
				Commits:        len(parsed),
				Branch:         branch,
				TagPrefix:      cfg.TagPrefix,
			}, outputFormat)
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
			fmt.Println("No releasable commits found — nothing to release.")
			return nil
		}
	}

	nextTag := cfg.TagPrefix + nextVer.String()

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
		Branch:         branch,
		TagPrefix:      cfg.TagPrefix,
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
			fmt.Printf("  • git tag %s\n", nextTag)
			fmt.Println("  • prepend entry to CHANGELOG.md")
			fmt.Println("\n[dry-run] No changes made.")
		} else {
			return printSummary(summary, outputFormat)
		}
		return nil
	}

	// 10. Create git tag
	if err := repo.CreateTag(ctx, nextTag, "Release "+nextTag); err != nil {
		return fmt.Errorf("creating tag: %w", err)
	}
	if outputFormat != "json" {
		fmt.Println(colors.Success(fmt.Sprintf("Created tag %s", colors.Bold(nextTag))))
	}

	// 11. Write CHANGELOG.md (prepend)
	if err := prependChangelog("CHANGELOG.md", changelogEntry); err != nil {
		fmt.Fprintln(os.Stderr, colors.Warning(fmt.Sprintf("could not update CHANGELOG.md: %v", err)))
	} else if outputFormat != "json" {
		fmt.Println(colors.Success("Updated CHANGELOG.md"))
	}

	// 12. Run configured plugins
	if len(cfg.Plugins) > 0 {
		orchestrator := plugininstance.NewOrchestrator(makePluginRunner(dryRun))
		if err := orchestrator.Run(ctx, pluginSpecsFromConfig(cfg.Plugins)); err != nil {
			return err
		}
	}

	if outputFormat == "json" {
		return printSummary(summary, outputFormat)
	}
	return nil
}

func pluginSpecsFromConfig(plugins []config.PluginConfig) []plugininstance.PluginSpec {
	specs := make([]plugininstance.PluginSpec, 0, len(plugins))
	for _, p := range plugins {
		specs = append(specs, plugininstance.PluginSpec{
			Uses:   p.Uses,
			Path:   p.Path,
			Config: p.Args,
		})
	}
	return specs
}

func makePluginRunner(dryRun bool) plugininstance.Runner {
	return func(ctx context.Context, spec plugininstance.PluginSpec) error {
		binPath, err := resolvePluginBinary(spec)
		if err != nil {
			fmt.Printf("⚠  plugin %q not installed (run: semrel plugin install %s)\n", spec.Uses, spec.Uses)
			return nil
		}

		if dryRun {
			fmt.Printf("[dry-run] would execute plugin: %s\n", binPath)
			return nil
		}

		cmd := exec.CommandContext(ctx, binPath)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		env := os.Environ()
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

func newLintCommand(configFile *string, outputFormat *string) *cobra.Command {
	return &cobra.Command{
		Use:   "lint",
		Short: "Validate commit messages since the last release",
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

Examples:
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
		return fmt.Errorf("provide commit messages as arguments, use --from/--to for a git range, or use --stdin")
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
		enc.Encode(summary)
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
