// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/GoSemantics/semrel/pkg/config"
)

func newConfigCommand(configFile *string, outputFormat *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage the semrel configuration",
		Long: `config provides subcommands for creating, inspecting, and validating .semrel.yaml.

Subcommands:
  init      — interactive wizard to create .semrel.yaml from scratch
  validate  — validate the current config file
  show      — print the resolved config
  set       — set a specific top-level key`,
	}
	cmd.AddCommand(newConfigInitCommand(configFile))
	cmd.AddCommand(newConfigValidateCommand(configFile))
	cmd.AddCommand(newConfigShowCommand(configFile, outputFormat))
	cmd.AddCommand(newConfigSetCommand(configFile))
	return cmd
}

// ---------------------------------------------------------------------------
// semrel config init
// ---------------------------------------------------------------------------

func newConfigInitCommand(configFile *string) *cobra.Command {
	var noInteractive bool
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive wizard to create .semrel.yaml",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigInit(*configFile, noInteractive, force)
		},
	}
	cmd.Flags().BoolVar(&noInteractive, "no-interactive", false,
		"Skip prompts and create a minimal default config (for CI)")
	cmd.Flags().BoolVar(&force, "force", false,
		"Overwrite an existing config file")
	return cmd
}

func runConfigInit(configFile string, noInteractive bool, force bool) error {
	target := configFile
	if target == "" {
		target = ".semrel.yaml"
	}

	// Check if file already exists.
	if _, err := os.Stat(target); err == nil && !force {
		return fmt.Errorf("%s already exists — use --force to overwrite", target)
	}

	cfg := defaultConfig()

	if !noInteractive {
		if !isTerminal() {
			fmt.Fprintln(os.Stderr, "warning: stdin is not a TTY — falling back to default config (use --no-interactive to suppress this warning)")
		} else {
			var err error
			cfg, err = runConfigWizard(cfg)
			if err != nil {
				return err
			}
		}
	}

	data, err := marshalConfigYAML(cfg)
	if err != nil {
		return fmt.Errorf("marshalling config: %w", err)
	}

	if err := os.WriteFile(target, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", target, err)
	}

	fmt.Printf("✔  %s written\n", target)
	return nil
}

// defaultConfig returns a sensible starting config.
func defaultConfig() *config.Config {
	return &config.Config{
		Branches:  []config.BranchConfig{{Name: "main"}},
		TagPrefix: "v",
		Plugins:   []config.PluginConfig{},
	}
}

// runConfigWizard walks the user through the interactive wizard.
func runConfigWizard(cfg *config.Config) (*config.Config, error) {
	scanner := bufio.NewScanner(os.Stdin)

	prompt := func(label, def string) string {
		if def != "" {
			fmt.Fprintf(os.Stderr, "? %s [%s]: ", label, def)
		} else {
			fmt.Fprintf(os.Stderr, "? %s: ", label)
		}
		if !scanner.Scan() {
			return def
		}
		v := strings.TrimSpace(scanner.Text())
		if v == "" {
			return def
		}
		return v
	}

	// Release branch.
	branchesInput := prompt("Release branch(es) (comma-separated)", "main")
	var branches []config.BranchConfig
	for _, b := range strings.Split(branchesInput, ",") {
		b = strings.TrimSpace(b)
		if b != "" {
			branches = append(branches, config.BranchConfig{Name: b})
		}
	}
	if len(branches) == 0 {
		branches = []config.BranchConfig{{Name: "main"}}
	}
	cfg.Branches = branches

	// Tag prefix.
	cfg.TagPrefix = prompt("Tag prefix", "v")

	// Provider plugin.
	fmt.Fprintf(os.Stderr, "? Git forge / provider plugin [github/gitlab/gitea/git/skip]: ")
	if scanner.Scan() {
		provider := strings.TrimSpace(strings.ToLower(scanner.Text()))
		switch provider {
		case "github", "provider-github":
			cfg.Plugins = append(cfg.Plugins, config.PluginConfig{
				Uses:  "provider-github",
				Phase: "release",
			})
		case "gitlab", "provider-gitlab":
			cfg.Plugins = append(cfg.Plugins, config.PluginConfig{
				Uses:  "provider-gitlab",
				Phase: "release",
			})
		case "gitea", "provider-gitea":
			cfg.Plugins = append(cfg.Plugins, config.PluginConfig{
				Uses:  "provider-gitea",
				Phase: "release",
			})
		case "git", "provider-git":
			cfg.Plugins = append(cfg.Plugins, config.PluginConfig{
				Uses:  "provider-git",
				Phase: "release",
			})
		}
	}

	// Changelog generator.
	fmt.Fprintf(os.Stderr, "? Add changelog generator? [changelog-md/skip]: ")
	if scanner.Scan() {
		gen := strings.TrimSpace(strings.ToLower(scanner.Text()))
		if gen == "changelog-md" || gen == "generator-changelog-md" {
			cfg.Plugins = append(cfg.Plugins, config.PluginConfig{
				Uses: "generator-changelog-md",
			})
		}
	}

	// Notification hook.
	fmt.Fprintf(os.Stderr, "? Add notification hook? [slack/teams/matrix/email/skip]: ")
	if scanner.Scan() {
		hook := strings.TrimSpace(strings.ToLower(scanner.Text()))
		switch hook {
		case "slack":
			cfg.Plugins = append(cfg.Plugins, config.PluginConfig{Uses: "hook-slack", Phase: "release"})
		case "teams":
			cfg.Plugins = append(cfg.Plugins, config.PluginConfig{Uses: "hook-teams", Phase: "release"})
		case "matrix":
			cfg.Plugins = append(cfg.Plugins, config.PluginConfig{Uses: "hook-matrix", Phase: "release"})
		case "email":
			cfg.Plugins = append(cfg.Plugins, config.PluginConfig{Uses: "hook-email", Phase: "release"})
		}
	}

	return cfg, nil
}

// marshalConfigYAML serialises the config to YAML with explanatory comments.
func marshalConfigYAML(cfg *config.Config) ([]byte, error) {
	var sb strings.Builder
	// yaml-language-server directive enables schema validation and auto-complete
	// in VS Code (YAML extension), JetBrains IDEs, and any LSP-aware editor.
	sb.WriteString("# yaml-language-server: $schema=https://semrel.io/schema/v1.json\n")
	sb.WriteString("# semrel configuration — https://semrel.io\n")
	sb.WriteString("#\n")
	sb.WriteString("# Full reference: https://semrel.io/guide/configuration/\n\n")

	// Marshal the struct to YAML without comments first, then prepend the header.
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	sb.Write(raw)
	return []byte(sb.String()), nil
}

// ---------------------------------------------------------------------------
// semrel config validate
// ---------------------------------------------------------------------------

func newConfigValidateCommand(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the current .semrel.yaml configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigValidate(*configFile)
		},
	}
}

func runConfigValidate(configFile string) error {
	cfgPath, err := resolveConfigFile(configFile)
	if err != nil {
		return err
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	fmt.Printf("✔  %s is valid\n", cfgPath)
	return nil
}

// ---------------------------------------------------------------------------
// semrel config show
// ---------------------------------------------------------------------------

func newConfigShowCommand(configFile *string, outputFormat *string) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the resolved configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigShow(*configFile, *outputFormat)
		},
	}
}

func runConfigShow(configFile string, outputFormat string) error {
	cfgPath, err := resolveConfigFile(configFile)
	if err != nil {
		return err
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}
	if outputFormat == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cfg)
	}
	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write(raw); err != nil {
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// semrel config set
// ---------------------------------------------------------------------------

func newConfigSetCommand(configFile *string) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a top-level or nested config key",
		Long: `set updates a single key in .semrel.yaml.

Supported keys:
  tagPrefix             — tag prefix string (e.g. "v")
  branches.0.name       — name of the first release branch
  version_ceiling       — version ceiling constraint
  ceiling_strategy      — how to handle the ceiling: "clamp" or "skip"
  commit_changelog      — whether to commit CHANGELOG.md (true/false)
  tag_exists_strategy   — "update-changelog", "skip", or "error"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigSet(*configFile, args[0], args[1])
		},
	}
}

func runConfigSet(configFile, key, value string) error {
	cfgPath, err := resolveConfigFile(configFile)
	if err != nil {
		return err
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := applyConfigKey(cfg, key, value); err != nil {
		return err
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("resulting config would be invalid: %w", err)
	}

	raw, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(cfgPath, raw, 0o644); err != nil {
		return err
	}
	fmt.Printf("✔  %s updated: %s = %s\n", cfgPath, key, value)
	return nil
}

// applyConfigKey sets a single key on the config using a dotted path notation.
func applyConfigKey(cfg *config.Config, key, value string) error {
	parts := strings.SplitN(key, ".", 3)
	switch parts[0] {
	case "tagPrefix", "tag_prefix":
		cfg.TagPrefix = value
	case "version_ceiling":
		cfg.VersionCeiling = value
	case "ceiling_strategy":
		cfg.CeilingStrategy = value
	case "tag_exists_strategy":
		cfg.TagExistsStrategy = value
	case "commit_changelog":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("commit_changelog must be true or false, got %q", value)
		}
		cfg.CommitChangelog = &b
	case "branches":
		if len(parts) < 3 {
			return fmt.Errorf("use branches.<index>.<field> (e.g. branches.0.name)")
		}
		idx, err := strconv.Atoi(parts[1])
		if err != nil || idx < 0 {
			return fmt.Errorf("branches index must be a non-negative integer, got %q", parts[1])
		}
		for len(cfg.Branches) <= idx {
			cfg.Branches = append(cfg.Branches, config.BranchConfig{})
		}
		switch parts[2] {
		case "name":
			cfg.Branches[idx].Name = value
		case "prerelease":
			cfg.Branches[idx].Prerelease = value
		case "maintenance":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return fmt.Errorf("maintenance must be true or false")
			}
			cfg.Branches[idx].Maintenance = b
		default:
			return fmt.Errorf("unknown branch field %q", parts[2])
		}
	default:
		// Try reflection as a last resort for simple string fields.
		v := reflect.ValueOf(cfg).Elem()
		t := v.Type()
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			yamlTag := strings.Split(f.Tag.Get("yaml"), ",")[0]
			if yamlTag == key || strings.EqualFold(f.Name, key) {
				fv := v.Field(i)
				if fv.Kind() == reflect.String && fv.CanSet() {
					fv.SetString(value)
					return nil
				}
			}
		}
		return fmt.Errorf("unknown config key %q — run `semrel config set --help` for supported keys", key)
	}
	return nil
}
