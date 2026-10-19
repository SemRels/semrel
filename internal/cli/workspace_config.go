// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package cli

import (
	"context"

	"github.com/SemRels/semrel/pkg/config"
)

// inheritedConfig tells runRelease to build a workspace package's effective
// config from the root config instead of loading a standalone config file.
type inheritedConfig struct {
	// BaseConfig is the absolute path of the root config.
	BaseConfig string
	// TagPrefix is the tag prefix assigned by the workspace entry.
	TagPrefix *string
}

type inheritedConfigKey struct{}

func withInheritedConfig(ctx context.Context, ic inheritedConfig) context.Context {
	return context.WithValue(ctx, inheritedConfigKey{}, &ic)
}

func inheritedConfigFrom(ctx context.Context) *inheritedConfig {
	ic, _ := ctx.Value(inheritedConfigKey{}).(*inheritedConfig)
	return ic
}

// loadReleaseConfig loads the config for a release run. In workspace mode it
// merges the optional package config (configFile) over the root config;
// otherwise it loads configFile as before.
func loadReleaseConfig(ctx context.Context, configFile string) (*config.Config, error) {
	if ic := inheritedConfigFrom(ctx); ic != nil {
		return config.LoadInherited(ic.BaseConfig, configFile, ic.TagPrefix)
	}
	resolved, err := resolveConfigFile(configFile)
	if err != nil {
		return nil, err
	}
	return config.LoadConfig(resolved)
}
