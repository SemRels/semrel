// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2026 The semrel Authors

// Package builtins registers all built-in semrel release plugins and provides
// a DefaultRegistry() function that returns a plugin.Registry pre-populated
// with in-process Executor implementations for every built-in plugin.
//
// Built-in plugins read their configuration from environment variables,
// with optional overrides via the plugin metadata map (from .semrel.yaml args).
package builtins

import (
"context"
"fmt"
"os"
"strings"

"github.com/GoSemantics/semrel/pkg/cargo"
"github.com/GoSemantics/semrel/pkg/docker"
"github.com/GoSemantics/semrel/pkg/gitea"
"github.com/GoSemantics/semrel/pkg/githubrelease"
"github.com/GoSemantics/semrel/pkg/gitlab"
"github.com/GoSemantics/semrel/pkg/gobinary"
"github.com/GoSemantics/semrel/pkg/gradle"
"github.com/GoSemantics/semrel/pkg/helm"
"github.com/GoSemantics/semrel/pkg/maven"
"github.com/GoSemantics/semrel/pkg/notify"
"github.com/GoSemantics/semrel/pkg/npm"
"github.com/GoSemantics/semrel/pkg/plugin"
"github.com/GoSemantics/semrel/pkg/python"
)

// DefaultRegistry returns a plugin.Registry with all built-in plugins pre-registered.
func DefaultRegistry() *plugin.Registry {
reg := plugin.NewRegistry()
for _, p := range allBuiltins() {
_ = reg.Register(p)
}
return reg
}

func allBuiltins() []plugin.Executor {
return []plugin.Executor{
newGitHubReleasePlugin(),
newNPMPlugin(),
newDockerPlugin(),
newHelmPlugin(),
newSlackPlugin(),
newMatrixPlugin(),
newGitLabPlugin(),
newGiteaPlugin(),
newCargoPlugin(),
newPythonPlugin(),
newGradlePlugin(),
newMavenPlugin(),
newGoBinaryPlugin(),
}
}

func envOr(key, fallback string) string {
if v := os.Getenv(key); v != "" {
return v
}
return fallback
}

func metaOr(meta map[string]string, key, fallback string) string {
if meta != nil {
if v := meta[key]; v != "" {
return v
}
}
return fallback
}

func ownerRepo(repository string) (owner, repo string) {
parts := strings.SplitN(repository, "/", 2)
if len(parts) == 2 {
return parts[0], parts[1]
}
return repository, ""
}

type gitHubReleasePlugin struct{ plugin.BasePlugin }

func newGitHubReleasePlugin() *gitHubReleasePlugin {
return &gitHubReleasePlugin{plugin.NewBasePlugin("github", "1.0.0")}
}
func (p *gitHubReleasePlugin) Validate() error {
if os.Getenv("GITHUB_TOKEN") == "" {
return plugin.ErrInvalidConfig{Plugin: p.Name(), Message: "GITHUB_TOKEN env var is required"}
}
return nil
}
func (p *gitHubReleasePlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
if rel.IsDryRun {
return plugin.SuccessResult(p.Name(), map[string]string{"dry_run": "true"}), nil
}
owner, repo := ownerRepo(rel.Repository)
client := githubrelease.NewClient(githubrelease.Config{
Token: envOr("GITHUB_TOKEN", ""),
Owner: owner,
Repo:  repo,
})
release, err := client.CreateRelease(ctx, githubrelease.CreateReleaseRequest{
TagName:    rel.TagName,
Name:       rel.TagName,
Body:       rel.Changelog,
Draft:      false,
Prerelease: rel.IsPrerelease,
})
if err != nil {
return nil, fmt.Errorf("github: create release: %w", err)
}
return plugin.SuccessResult(p.Name(), map[string]string{
"release_id":  fmt.Sprintf("%d", release.ID),
"release_url": release.HTMLURL,
}), nil
}

type npmPlugin struct{ plugin.BasePlugin }

func newNPMPlugin() *npmPlugin { return &npmPlugin{plugin.NewBasePlugin("npm", "1.0.0")} }
func (p *npmPlugin) Validate() error {
if !npm.IsNPMAvailable() {
return plugin.ErrInvalidConfig{Plugin: p.Name(), Message: "npm CLI is not available in PATH"}
}
return nil
}
func (p *npmPlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
if rel.IsDryRun {
return plugin.SuccessResult(p.Name(), map[string]string{"dry_run": "true"}), nil
}
publisher := npm.NewPublisher(npm.Config{
Registry: metaOr(rel.Metadata, "registry", envOr("NPM_REGISTRY", "")),
Token:    envOr("NPM_TOKEN", ""),
})
if _, err := npm.UpdateVersion(metaOr(rel.Metadata, "package_json", "package.json"), rel.Version); err != nil {
return nil, fmt.Errorf("npm: update version: %w", err)
}
if err := publisher.PublishCLI(ctx, "."); err != nil {
return nil, fmt.Errorf("npm: publish: %w", err)
}
return plugin.SuccessResult(p.Name(), map[string]string{"version": rel.Version}), nil
}

type dockerPlugin struct{ plugin.BasePlugin }

func newDockerPlugin() *dockerPlugin { return &dockerPlugin{plugin.NewBasePlugin("docker", "1.0.0")} }
func (p *dockerPlugin) Validate() error {
if !docker.IsDockerAvailable() {
return plugin.ErrInvalidConfig{Plugin: p.Name(), Message: "docker CLI is not available in PATH"}
}
return nil
}
func (p *dockerPlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
if rel.IsDryRun {
return plugin.SuccessResult(p.Name(), map[string]string{"dry_run": "true"}), nil
}
image := metaOr(rel.Metadata, "image", envOr("DOCKER_IMAGE", ""))
if image == "" {
return plugin.SkippedResult(p.Name(), "no image configured (set DOCKER_IMAGE or args.image)"), nil
}
tagger := docker.NewTagger(docker.Config{
Image: image,
Tags:  docker.GenerateTags(rel.Version),
})
tags, err := tagger.TagAndPush(ctx)
if err != nil {
return nil, fmt.Errorf("docker: tag and push: %w", err)
}
return plugin.SuccessResult(p.Name(), map[string]string{"tags": fmt.Sprintf("%v", tags)}), nil
}

func newHelmPlugin() *helm.Plugin {
return helm.NewPlugin(helm.PluginConfig{
ChartDir:         envOr("HELM_CHART_DIR", "chart"),
UpdateAppVersion: true,
})
}

type slackPlugin struct{ plugin.BasePlugin }

func newSlackPlugin() *slackPlugin { return &slackPlugin{plugin.NewBasePlugin("slack", "1.0.0")} }
func (p *slackPlugin) Validate() error {
if os.Getenv("SLACK_WEBHOOK_URL") == "" {
return plugin.ErrInvalidConfig{Plugin: p.Name(), Message: "SLACK_WEBHOOK_URL env var is required"}
}
return nil
}
func (p *slackPlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
if rel.IsDryRun {
return plugin.SuccessResult(p.Name(), map[string]string{"dry_run": "true"}), nil
}
n := notify.NewSlackNotifier(notify.SlackConfig{
WebhookURL: metaOr(rel.Metadata, "webhook_url", envOr("SLACK_WEBHOOK_URL", "")),
})
if err := n.Notify(ctx, rel.Version, rel.Changelog, rel.Repository); err != nil {
return nil, fmt.Errorf("slack: notify: %w", err)
}
return plugin.SuccessResult(p.Name(), nil), nil
}

type matrixPlugin struct{ plugin.BasePlugin }

func newMatrixPlugin() *matrixPlugin {
return &matrixPlugin{plugin.NewBasePlugin("matrix", "1.0.0")}
}
func (p *matrixPlugin) Validate() error {
if os.Getenv("MATRIX_HOMESERVER_URL") == "" || os.Getenv("MATRIX_ROOM_ID") == "" {
return plugin.ErrInvalidConfig{Plugin: p.Name(), Message: "MATRIX_HOMESERVER_URL and MATRIX_ROOM_ID env vars are required"}
}
return nil
}
func (p *matrixPlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
if rel.IsDryRun {
return plugin.SuccessResult(p.Name(), map[string]string{"dry_run": "true"}), nil
}
n := notify.NewMatrixNotifier(notify.MatrixConfig{
HomeserverURL: metaOr(rel.Metadata, "homeserver_url", envOr("MATRIX_HOMESERVER_URL", "")),
RoomID:        metaOr(rel.Metadata, "room_id", envOr("MATRIX_ROOM_ID", "")),
AccessToken:   envOr("MATRIX_ACCESS_TOKEN", ""),
})
if err := n.Notify(ctx, rel.Version, rel.Changelog, rel.Repository); err != nil {
return nil, fmt.Errorf("matrix: notify: %w", err)
}
return plugin.SuccessResult(p.Name(), nil), nil
}

type gitLabPlugin struct{ plugin.BasePlugin }

func newGitLabPlugin() *gitLabPlugin {
return &gitLabPlugin{plugin.NewBasePlugin("gitlab", "1.0.0")}
}
func (p *gitLabPlugin) Validate() error {
if os.Getenv("GITLAB_TOKEN") == "" {
return plugin.ErrInvalidConfig{Plugin: p.Name(), Message: "GITLAB_TOKEN env var is required"}
}
return nil
}
func (p *gitLabPlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
if rel.IsDryRun {
return plugin.SuccessResult(p.Name(), map[string]string{"dry_run": "true"}), nil
}
client := gitlab.NewClient(gitlab.Config{
Token:     envOr("GITLAB_TOKEN", ""),
BaseURL:   envOr("GITLAB_BASE_URL", ""),
ProjectID: metaOr(rel.Metadata, "project_id", envOr("GITLAB_PROJECT_ID", rel.Repository)),
})
release, err := client.CreateRelease(ctx, gitlab.CreateReleaseRequest{
TagName:     rel.TagName,
Name:        rel.TagName,
Description: rel.Changelog,
})
if err != nil {
return nil, fmt.Errorf("gitlab: create release: %w", err)
}
return plugin.SuccessResult(p.Name(), map[string]string{"release_name": release.Name}), nil
}

type giteaPlugin struct{ plugin.BasePlugin }

func newGiteaPlugin() *giteaPlugin { return &giteaPlugin{plugin.NewBasePlugin("gitea", "1.0.0")} }
func (p *giteaPlugin) Validate() error {
if os.Getenv("GITEA_TOKEN") == "" || os.Getenv("GITEA_BASE_URL") == "" {
return plugin.ErrInvalidConfig{Plugin: p.Name(), Message: "GITEA_TOKEN and GITEA_BASE_URL env vars are required"}
}
return nil
}
func (p *giteaPlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
if rel.IsDryRun {
return plugin.SuccessResult(p.Name(), map[string]string{"dry_run": "true"}), nil
}
owner, repo := ownerRepo(rel.Repository)
client := gitea.NewClient(gitea.Config{
Token:   envOr("GITEA_TOKEN", ""),
BaseURL: envOr("GITEA_BASE_URL", ""),
Owner:   owner,
Repo:    repo,
})
release, err := client.CreateRelease(ctx, rel.TagName, rel.Changelog)
if err != nil {
return nil, fmt.Errorf("gitea: create release: %w", err)
}
return plugin.SuccessResult(p.Name(), map[string]string{"release_id": fmt.Sprintf("%d", release.ID)}), nil
}

type cargoPlugin struct{ plugin.BasePlugin }

func newCargoPlugin() *cargoPlugin { return &cargoPlugin{plugin.NewBasePlugin("cargo", "1.0.0")} }
func (p *cargoPlugin) Validate() error {
if !cargo.IsCargoAvailable() {
return plugin.ErrInvalidConfig{Plugin: p.Name(), Message: "cargo CLI is not available in PATH"}
}
return nil
}
func (p *cargoPlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
if rel.IsDryRun {
return plugin.SuccessResult(p.Name(), map[string]string{"dry_run": "true"}), nil
}
manifestPath := metaOr(rel.Metadata, "manifest", "Cargo.toml")
if _, err := cargo.UpdateVersion(manifestPath, rel.Version); err != nil {
return nil, fmt.Errorf("cargo: update version: %w", err)
}
pub := cargo.NewPublisher(cargo.Config{
Token: envOr("CARGO_REGISTRY_TOKEN", ""),
})
if err := pub.Publish(ctx, "."); err != nil {
return nil, fmt.Errorf("cargo: publish: %w", err)
}
return plugin.SuccessResult(p.Name(), map[string]string{"version": rel.Version}), nil
}

type pythonPlugin struct{ plugin.BasePlugin }

func newPythonPlugin() *pythonPlugin {
return &pythonPlugin{plugin.NewBasePlugin("python", "1.0.0")}
}
func (p *pythonPlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
if rel.IsDryRun {
return plugin.SuccessResult(p.Name(), map[string]string{"dry_run": "true"}), nil
}
pyprojectPath := metaOr(rel.Metadata, "pyproject", "pyproject.toml")
if _, err := python.UpdatePyprojectVersion(pyprojectPath, rel.Version); err != nil {
if err2 := python.UpdateSetupCfgVersion("setup.cfg", rel.Version); err2 != nil {
return nil, fmt.Errorf("python: update version: %w", err)
}
}
pub := python.NewPublisher(python.Config{
Repository: envOr("PYPI_REPOSITORY_URL", ""),
Username:   envOr("PYPI_USERNAME", "__token__"),
Password:   envOr("PYPI_TOKEN", ""),
})
if err := pub.UploadWithTwine(ctx, ".", "dist/*"); err != nil {
return nil, fmt.Errorf("python: publish: %w", err)
}
return plugin.SuccessResult(p.Name(), map[string]string{"version": rel.Version}), nil
}

type gradlePlugin struct{ plugin.BasePlugin }

func newGradlePlugin() *gradlePlugin {
return &gradlePlugin{plugin.NewBasePlugin("gradle", "1.0.0")}
}
func (p *gradlePlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
if rel.IsDryRun {
return plugin.SuccessResult(p.Name(), map[string]string{"dry_run": "true"}), nil
}
buildFile := metaOr(rel.Metadata, "build_file", "build.gradle")
if _, err := gradle.UpdateVersion(buildFile, rel.Version); err != nil {
return nil, fmt.Errorf("gradle: update version: %w", err)
}
pub := gradle.NewPublisher(gradle.Config{})
if err := pub.Publish(ctx, "."); err != nil {
return nil, fmt.Errorf("gradle: publish: %w", err)
}
return plugin.SuccessResult(p.Name(), map[string]string{"version": rel.Version}), nil
}

type mavenPlugin struct{ plugin.BasePlugin }

func newMavenPlugin() *mavenPlugin { return &mavenPlugin{plugin.NewBasePlugin("maven", "1.0.0")} }
func (p *mavenPlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
if rel.IsDryRun {
return plugin.SuccessResult(p.Name(), map[string]string{"dry_run": "true"}), nil
}
pomPath := metaOr(rel.Metadata, "pom", "pom.xml")
if _, err := maven.UpdatePOMVersion(pomPath, rel.Version); err != nil {
return nil, fmt.Errorf("maven: update version: %w", err)
}
pub := maven.NewPublisher(maven.Config{})
if err := pub.Deploy(ctx, "."); err != nil {
return nil, fmt.Errorf("maven: deploy: %w", err)
}
return plugin.SuccessResult(p.Name(), map[string]string{"version": rel.Version}), nil
}

type goBinaryPlugin struct{ plugin.BasePlugin }

func newGoBinaryPlugin() *goBinaryPlugin {
return &goBinaryPlugin{plugin.NewBasePlugin("gobinary", "1.0.0")}
}
func (p *goBinaryPlugin) Execute(ctx context.Context, rel plugin.ReleaseContext) (*plugin.Result, error) {
if rel.IsDryRun {
return plugin.SuccessResult(p.Name(), map[string]string{"dry_run": "true"}), nil
}
mainPkg := metaOr(rel.Metadata, "main", "./cmd/semrel")
outputDir := metaOr(rel.Metadata, "output_dir", "dist")
builder := gobinary.NewBuilder(gobinary.BuildConfig{
MainPackage: mainPkg,
BinaryName:  "semrel",
Version:     rel.Version,
OutputDir:   outputDir,
})
artifacts, err := builder.Build()
if err != nil {
return nil, fmt.Errorf("gobinary: build: %w", err)
}
return plugin.SuccessResult(p.Name(), map[string]string{
"archives": fmt.Sprintf("%d", len(artifacts)),
"dir":      outputDir,
}), nil
}