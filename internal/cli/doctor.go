// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/SemRels/semrel/internal/colors"
	"github.com/SemRels/semrel/pkg/config"
	gitpkg "github.com/SemRels/semrel/pkg/git"
)

// DoctorCheck represents a single pre-flight check performed by `semrel doctor`.
type DoctorCheck struct {
	// Name is the short machine-readable identifier for the check.
	Name string `json:"name"`
	// Status is "ok", "warn", or "fail".
	Status string `json:"status"`
	// Message is the human-readable result.
	Message string `json:"message"`
	// Fix is an optional suggestion for how to resolve the issue.
	Fix string `json:"fix,omitempty"`
}

// DoctorResult is the top-level output of `semrel doctor`.
type DoctorResult struct {
	Healthy bool          `json:"healthy"`
	Checks  []DoctorCheck `json:"checks"`
}

func newDoctorCommand(configFile *string, outputFormat *string) *cobra.Command {
	var online bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run pre-flight checks on the semrel configuration and environment",
		Long: `doctor validates the local environment before a release is attempted.

It checks:
  • .semrel.yaml exists and is valid
  • each configured plugin binary is discoverable (~/.semrel/plugins/ or $PATH)
  • required environment variables are set (where detectable)
  • the current branch is a configured release branch
  • a git repository is present and at least one tag exists

Exit code 0 = all checks pass (no failures). Non-zero = one or more checks failed.
Warnings do not affect the exit code.

Use --online to also ping the semrel registry for plugin availability.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(cmd.Context(), *configFile, *outputFormat, online)
		},
	}
	cmd.Flags().BoolVar(&online, "online", false,
		"Ping the semrel registry to verify plugin versions (requires network access)")
	return cmd
}

func runDoctor(ctx context.Context, configFile string, outputFormat string, online bool) error {
	var checks []DoctorCheck

	// 1. Resolve and validate config file.
	cfgPath, cfg, cfgCheck := checkConfig(configFile)
	checks = append(checks, cfgCheck)

	// Remaining checks only make sense when the config was loaded.
	if cfg != nil {
		// 2. Verify each plugin binary is findable.
		checks = append(checks, checkPlugins(cfg)...)

		// 3. Check current branch matches a release branch.
		checks = append(checks, checkBranch(ctx, cfg)...)
	}

	// 4. Check git repository.
	checks = append(checks, checkGit(ctx, cfgPath)...)

	// 5. Check common env vars.
	checks = append(checks, checkEnvVars(cfg)...)

	// 6. (optional) Online registry ping — currently a stub.
	if online {
		checks = append(checks, DoctorCheck{
			Name:    "registry-online",
			Status:  "warn",
			Message: "--online mode is not yet implemented; skipping registry check",
		})
	}

	healthy := true
	for _, c := range checks {
		if c.Status == "fail" {
			healthy = false
			break
		}
	}

	result := DoctorResult{Healthy: healthy, Checks: checks}

	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	printDoctorResult(result)

	if !healthy {
		return fmt.Errorf("one or more checks failed")
	}
	return nil
}

// checkConfig resolves, loads and validates the .semrel.yaml.
func checkConfig(configFile string) (string, *config.Config, DoctorCheck) {
	check := DoctorCheck{Name: "config-file"}

	cfgPath := configFile
	if cfgPath == "" {
		found, err := config.FindConfigFile(".")
		if err != nil {
			check.Status = "fail"
			check.Message = "no config file found (.semrel.yaml / .semrel.yml / .semrel.toml / .semrel.json)"
			check.Fix = "run `semrel config init` or create .semrel.yaml manually"
			return "", nil, check
		}
		cfgPath = found
	}

	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		check.Status = "fail"
		check.Message = fmt.Sprintf("%s found but failed to load: %v", cfgPath, err)
		check.Fix = "fix the syntax or schema errors listed above"
		return cfgPath, nil, check
	}

	if err := cfg.Validate(); err != nil {
		check.Status = "fail"
		check.Message = fmt.Sprintf("%s is invalid: %v", cfgPath, err)
		check.Fix = "correct the validation errors in your config file"
		return cfgPath, cfg, check
	}

	check.Status = "ok"
	check.Message = fmt.Sprintf("%s found and valid", cfgPath)
	return cfgPath, cfg, check
}

// checkPlugins verifies that every plugin binary is discoverable.
func checkPlugins(cfg *config.Config) []DoctorCheck {
	var checks []DoctorCheck

	specs := pluginSpecsFromConfig(cfg.Plugins)
	seen := map[string]bool{}

	for _, spec := range specs {
		label := spec.Uses
		if label == "" {
			label = spec.Path
		}
		name := "plugin:" + label
		if seen[name] {
			continue
		}
		seen[name] = true

		foundPath, err := resolvePluginBinary(spec)
		if err != nil {
			fix := fmt.Sprintf("run `semrel plugin install %s`", label)
			checks = append(checks, DoctorCheck{
				Name:    name,
				Status:  "fail",
				Message: fmt.Sprintf("%s NOT FOUND — expected at ~/.semrel/plugins/ or $PATH", pluginBinaryName(spec.Uses)),
				Fix:     fix,
			})
			continue
		}
		// For path-based specs, resolvePluginBinary returns the path without checking existence.
		if info, statErr := os.Stat(foundPath); statErr != nil || info.IsDir() {
			checks = append(checks, DoctorCheck{
				Name:    name,
				Status:  "fail",
				Message: fmt.Sprintf("plugin binary not found at path: %s", foundPath),
				Fix:     fmt.Sprintf("verify the path or run `semrel plugin install %s`", label),
			})
		} else {
			checks = append(checks, DoctorCheck{
				Name:    name,
				Status:  "ok",
				Message: fmt.Sprintf("found at %s", foundPath),
			})
		}
	}

	return checks
}

// checkBranch verifies the current branch is a configured release branch.
func checkBranch(ctx context.Context, cfg *config.Config) []DoctorCheck {
	repo, err := gitpkg.OpenRepository(".")
	if err != nil {
		// git check will catch this separately.
		return nil
	}

	branch, err := repo.CurrentBranch(ctx)
	if err != nil {
		return []DoctorCheck{{
			Name:    "release-branch",
			Status:  "warn",
			Message: "could not determine current branch",
		}}
	}

	for _, b := range cfg.Branches {
		if b.Name == branch {
			return []DoctorCheck{{
				Name:    "release-branch",
				Status:  "ok",
				Message: fmt.Sprintf("current branch %q is a configured release branch", branch),
			}}
		}
	}

	return []DoctorCheck{{
		Name:    "release-branch",
		Status:  "warn",
		Message: fmt.Sprintf("current branch %q is not in the configured release branches", branch),
		Fix:     "switch to a configured release branch or add it to `branches` in .semrel.yaml",
	}}
}

// checkGit verifies the git repository is reachable and has at least one tag.
func checkGit(ctx context.Context, configFile string) []DoctorCheck {
	var checks []DoctorCheck

	// Check git is on PATH.
	gitPath, err := exec.LookPath("git")
	if err != nil {
		checks = append(checks, DoctorCheck{
			Name:    "git-binary",
			Status:  "fail",
			Message: "git binary not found on $PATH",
			Fix:     "install git (https://git-scm.com/downloads)",
		})
		return checks
	}
	checks = append(checks, DoctorCheck{
		Name:    "git-binary",
		Status:  "ok",
		Message: fmt.Sprintf("git found at %s", gitPath),
	})

	// Check we're inside a git repository.
	repoDir := "."
	if configFile != "" {
		repoDir = filepath.Dir(configFile)
	}
	repo, err := gitpkg.OpenRepository(repoDir)
	if err != nil {
		checks = append(checks, DoctorCheck{
			Name:    "git-repository",
			Status:  "fail",
			Message: "not inside a git repository (or git repo could not be opened)",
			Fix:     "run `git init` or navigate to a git repository root",
		})
		return checks
	}
	checks = append(checks, DoctorCheck{
		Name:    "git-repository",
		Status:  "ok",
		Message: fmt.Sprintf("git repository detected (path: %s)", repo.Path),
	})

	// Check at least one tag exists (semrel needs a baseline version).
	lastTag, err := repo.LastTag(ctx)
	if err != nil {
		checks = append(checks, DoctorCheck{
			Name:    "git-tags",
			Status:  "warn",
			Message: "could not read git tags",
		})
	} else if lastTag == "" {
		checks = append(checks, DoctorCheck{
			Name:    "git-tags",
			Status:  "warn",
			Message: "no git tags found — semrel will start from v0.0.0 on first release",
			Fix:     "create an initial tag with `git tag v0.0.0` if you want to control the starting version",
		})
	} else {
		checks = append(checks, DoctorCheck{
			Name:    "git-tags",
			Status:  "ok",
			Message: fmt.Sprintf("last tag: %s", lastTag),
		})
	}

	return checks
}

// checkEnvVars checks for commonly required environment variables.
// This is heuristic: we detect known provider patterns from plugin names.
func checkEnvVars(cfg *config.Config) []DoctorCheck {
	if cfg == nil {
		return nil
	}

	type envCheck struct {
		plugin string
		envVar string
	}

	var needed []envCheck
	for _, p := range cfg.Plugins {
		name := strings.ToLower(p.Uses + p.Path)
		switch {
		case strings.Contains(name, "provider-github") || strings.Contains(name, "github"):
			needed = append(needed, envCheck{"provider-github", "GITHUB_TOKEN"})
		case strings.Contains(name, "provider-gitlab") || strings.Contains(name, "gitlab"):
			needed = append(needed, envCheck{"provider-gitlab", "GITLAB_TOKEN"})
		case strings.Contains(name, "provider-gitea") || strings.Contains(name, "gitea"):
			needed = append(needed, envCheck{"provider-gitea", "GITEA_TOKEN"})
		}
	}

	var checks []DoctorCheck
	seen := map[string]bool{}
	for _, n := range needed {
		if seen[n.envVar] {
			continue
		}
		seen[n.envVar] = true

		if os.Getenv(n.envVar) != "" {
			checks = append(checks, DoctorCheck{
				Name:    "env:" + strings.ToLower(n.envVar),
				Status:  "ok",
				Message: fmt.Sprintf("%s is set (required by %s)", n.envVar, n.plugin),
			})
		} else {
			checks = append(checks, DoctorCheck{
				Name:    "env:" + strings.ToLower(n.envVar),
				Status:  "warn",
				Message: fmt.Sprintf("%s is not set — %s will fail at runtime", n.envVar, n.plugin),
				Fix:     fmt.Sprintf("export %s=<token>", n.envVar),
			})
		}
	}

	return checks
}

// printDoctorResult writes a human-readable summary to stdout.
func printDoctorResult(result DoctorResult) {
	for _, c := range result.Checks {
		var sym, symColored, msgColored string
		switch c.Status {
		case "ok":
			sym = "✔"
			symColored = colors.Green(sym)
			msgColored = c.Message
		case "warn":
			sym = "⚠"
			symColored = colors.Yellow(sym)
			msgColored = colors.Yellow(c.Message)
		case "fail":
			sym = "✗"
			symColored = colors.Red(sym)
			msgColored = colors.Red(c.Message)
		default:
			sym = "?"
			symColored = sym
			msgColored = c.Message
		}

		fmt.Printf("%s  %-30s %s\n", symColored, c.Name, msgColored)
		if c.Fix != "" {
			fmt.Printf("   %-30s hint: %s\n", "", c.Fix)
		}
	}

	fmt.Println()
	if result.Healthy {
		fmt.Printf("%s  all checks passed\n", colors.Green("✔"))
	} else {
		fmt.Printf("%s  one or more checks failed — fix the issues above before releasing\n", colors.Red("✗"))
	}
}
