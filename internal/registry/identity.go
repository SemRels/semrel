// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

package registry

import "strings"

const firstPartyNamespace = "@semrel"

var pluginCategories = map[string]bool{
	"provider": true, "condition": true, "analyzer": true, "generator": true,
	"updater": true, "hook": true, "packager": true, "publisher": true,
}

var firstPartyCanonicalNames = []string{
	"analyzer-conventional", "analyzer-default",
	"condition-bitbucket-pipelines", "condition-circleci", "condition-generic",
	"condition-gitea-actions", "condition-github-actions", "condition-gitlab-ci",
	"generator-changelog-html", "generator-changelog-md", "generator-release-notes",
	"hook-discord", "hook-email", "hook-gitplugin", "hook-jira", "hook-matrix", "hook-slack", "hook-teams",
	"packager-nfpm",
	"provider-bitbucket", "provider-git", "provider-gitea", "provider-github", "provider-gitlab",
	"publisher-crates", "publisher-generic-http", "publisher-npm", "publisher-oci", "publisher-pypi",
	"updater-cargo", "updater-composer", "updater-docker", "updater-go", "updater-gradle",
	"updater-helm", "updater-homebrew", "updater-maven", "updater-npm", "updater-nuget",
	"updater-pubspec", "updater-python", "updater-terraform",
}

// legacyArtifactNames records executable/cache basenames used before package
// identities became typed. New plugins must not inherit a short basename,
// because categories can legitimately contain the same short name.
var legacyArtifactNames = map[string]string{
	"analyzer-conventional": "conventional", "analyzer-default": "default",
	"condition-generic": "generic", "condition-gitea-actions": "gitea-actions",
	"condition-github-actions": "github-actions", "condition-gitlab-ci": "gitlab-ci",
	"generator-changelog-html": "changelog-html", "generator-changelog-md": "changelog-md",
	"generator-release-notes": "release-notes",
	"hook-email":              "email", "hook-gitplugin": "gitplugin", "hook-jira": "jira",
	"hook-matrix": "matrix", "hook-slack": "slack", "hook-teams": "teams",
	"packager-nfpm":      "nfpm",
	"provider-bitbucket": "bitbucket", "provider-git": "git", "provider-gitea": "gitea",
	"provider-github": "github", "provider-gitlab": "gitlab",
	"publisher-generic-http": "generic-http", "publisher-oci": "oci",
	"updater-cargo": "cargo", "updater-docker": "docker", "updater-go": "go",
	"updater-gradle": "gradle", "updater-helm": "helm", "updater-homebrew": "homebrew",
	"updater-maven": "maven", "updater-npm": "npm", "updater-nuget": "nuget",
	"updater-python": "python", "updater-terraform": "terraform",
}

// legacyFirstPartyAliases is deliberately explicit. These aliases are part of
// the compatibility contract and must not depend on registry ordering. In
// particular, npm historically meant updater-npm, even though publisher-npm
// also exists.
var legacyFirstPartyAliases = buildLegacyFirstPartyAliases(map[string]string{
	"bitbucket":           "provider-bitbucket",
	"git":                 "provider-git",
	"gitea":               "provider-gitea",
	"github":              "provider-github",
	"gitlab":              "provider-gitlab",
	"conventional":        "analyzer-conventional",
	"default":             "analyzer-default",
	"changelog-html":      "generator-changelog-html",
	"changelog-md":        "generator-changelog-md",
	"release-notes":       "generator-release-notes",
	"bitbucket-pipelines": "condition-bitbucket-pipelines",
	"circleci":            "condition-circleci",
	"generic":             "condition-generic",
	"gitea-actions":       "condition-gitea-actions",
	"github-actions":      "condition-github-actions",
	"gitlab-ci":           "condition-gitlab-ci",
	"discord":             "hook-discord",
	"email":               "hook-email",
	"gitplugin":           "hook-gitplugin",
	"jira":                "hook-jira",
	"matrix":              "hook-matrix",
	"slack":               "hook-slack",
	"teams":               "hook-teams",
	"cargo":               "updater-cargo",
	"composer":            "updater-composer",
	"docker":              "updater-docker",
	"go":                  "updater-go",
	"gradle":              "updater-gradle",
	"helm":                "updater-helm",
	"homebrew":            "updater-homebrew",
	"maven":               "updater-maven",
	"npm":                 "updater-npm",
	"nuget":               "updater-nuget",
	"pubspec":             "updater-pubspec",
	"python":              "updater-python",
	"terraform":           "updater-terraform",
	"nfpm":                "packager-nfpm",
	"crates":              "publisher-crates",
	"generic-http":        "publisher-generic-http",
	"oci":                 "publisher-oci",
	"pypi":                "publisher-pypi",
})

func buildLegacyFirstPartyAliases(shortToTyped map[string]string) map[string]string {
	aliases := make(map[string]string, len(shortToTyped)*2+len(firstPartyCanonicalNames)*2)
	for _, typed := range firstPartyCanonicalNames {
		canonical := firstPartyNamespace + "/" + typed
		aliases[typed] = canonical
		aliases[canonical] = canonical
	}
	for short, typed := range shortToTyped {
		canonical := firstPartyNamespace + "/" + typed
		aliases[short] = canonical
		aliases[firstPartyNamespace+"/"+short] = canonical
	}
	return aliases
}

// CanonicalLegacyRef canonicalizes a known first-party compatibility alias.
// Unknown and third-party references are intentionally left to registry
// metadata; this function never guesses by stripping a category prefix.
func CanonicalLegacyRef(ref string) (string, bool) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	canonical, ok := legacyFirstPartyAliases[ref]
	return canonical, ok
}

// LegacyExecutableName returns a pre-migration executable/cache basename only
// for artifacts that actually used one.
func LegacyExecutableName(ref string) (string, bool) {
	canonical, ok := CanonicalLegacyRef(ref)
	if !ok {
		return "", false
	}
	name := canonical[strings.LastIndex(canonical, "/")+1:]
	legacy, ok := legacyArtifactNames[name]
	return legacy, ok
}

func normalizeNamespace(namespace string) string {
	namespace = strings.ToLower(strings.TrimSpace(namespace))
	if namespace != "" && !strings.HasPrefix(namespace, "@") {
		namespace = "@" + namespace
	}
	return namespace
}

func normalizeCategory(category string) string {
	category = strings.ToLower(strings.TrimSpace(category))
	category = strings.TrimSuffix(category, "s")
	if pluginCategories[category] {
		return category
	}
	return ""
}
