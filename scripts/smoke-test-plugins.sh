#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2026 The semrel Authors
#
# smoke-test-plugins.sh — build and smoke-test all semrel plugin binaries locally.
#
# Usage:
#   ./scripts/smoke-test-plugins.sh [plugin-repo-dir ...]
#
# If no arguments are given the script looks for plugin repos in ../
# (i.e., sibling directories named provider-*, hook-*, condition-*, analyzer-*,
# updater-*, generator-*).
#
# Each plugin repo is built with `go build ./cmd/plugin` and then executed with
# SEMREL_DRY_RUN=true and synthetic SEMREL_* release context env vars.
# A plugin exits 0 → PASS.  Exit non-zero → FAIL (but missing required config
# like webhook URLs is expected and results in a warned SKIP).

set -euo pipefail

PASS=0
FAIL=0
SKIP=0

# Fake release context — safe to use for any plugin.
FAKE_ENV=(
  "SEMREL_VERSION=v1.2.3"
  "SEMREL_TAG_NAME=v1.2.3"
  "SEMREL_NEXT_VERSION=v1.2.3"
  "SEMREL_CURRENT_VERSION=v1.2.2"
  "SEMREL_BUMP=patch"
  "SEMREL_BRANCH=main"
  "SEMREL_TAG_PREFIX=v"
  "SEMREL_CHANGELOG=## v1.2.3\n\n- fix: test change"
  "SEMREL_DRY_RUN=true"
  "SEMREL_COMMITS=[\"fix: test commit\"]"
  # Condition plugins — provide fake CI env vars so they pass
  "GITHUB_ACTIONS=true"
  "GITHUB_TOKEN=fake-token"
  "GITHUB_REF_NAME=main"
  "GITEA_ACTIONS=true"
  "GITEA_TOKEN=fake-token"
  "GITLAB_CI=true"
  "CI_JOB_TOKEN=fake-token"
  "CI_COMMIT_REF_NAME=main"
)

find_plugin_repos() {
  local base="${1:-$(dirname "$0")/../..}"
  # Canonical patterns for semrel plugin repo names
  find "$base" -maxdepth 1 -type d \( \
    -name "provider-*" -o -name "hook-*" -o \
    -name "condition-*" -o -name "analyzer-*" -o \
    -name "updater-*" -o -name "generator-*" \
  \) | sort
}

smoke_test_repo() {
  local repo_dir="$1"
  local repo_name
  repo_name=$(basename "$repo_dir")

  if [[ ! -f "$repo_dir/cmd/plugin/main.go" ]]; then
    echo "  SKIP $repo_name (no cmd/plugin/main.go)"
    ((SKIP++)) || true
    return
  fi

  # Build
  local bin_dir
  bin_dir=$(mktemp -d)
  local bin="$bin_dir/semrel-plugin-$repo_name"
  if ! (cd "$repo_dir" && go build -o "$bin" ./cmd/plugin 2>&1); then
    echo "  FAIL $repo_name (build failed)"
    ((FAIL++)) || true
    rm -rf "$bin_dir"
    return
  fi

  # Run with fake context
  local env_args=()
  for kv in "${FAKE_ENV[@]}"; do
    env_args+=("$kv")
  done

  local output
  local exit_code=0
  output=$(env "${env_args[@]}" "$bin" 2>&1) || exit_code=$?

  rm -rf "$bin_dir"

  if [[ $exit_code -eq 0 ]]; then
    echo "  PASS $repo_name"
    ((PASS++)) || true
  else
    # Plugins that need real config (e.g., webhook URL) will exit 1 with a
    # "required" message — treat those as expected SKIP rather than FAIL.
    if echo "$output" | grep -qiE "(required|missing|not set|must be set)"; then
      echo "  SKIP $repo_name (needs real config — dry-run OK)"
      ((SKIP++)) || true
    else
      echo "  FAIL $repo_name (exit $exit_code)"
      echo "       Output: $output"
      ((FAIL++)) || true
    fi
  fi
}

# ── Main ───────────────────────────────────────────────────────────────────────

repos=("$@")
if [[ ${#repos[@]} -eq 0 ]]; then
  mapfile -t repos < <(find_plugin_repos)
fi

if [[ ${#repos[@]} -eq 0 ]]; then
  echo "No plugin repos found. Clone the plugin repos as siblings of this repo."
  echo "e.g.: git clone https://github.com/SemRels/hook-slack ../hook-slack"
  exit 0
fi

echo "semrel plugin smoke tests"
echo "========================="
echo "Found ${#repos[@]} plugin repo(s)"
echo ""

for repo in "${repos[@]}"; do
  smoke_test_repo "$repo"
done

echo ""
echo "Results: PASS=$PASS  SKIP=$SKIP  FAIL=$FAIL"

if [[ $FAIL -gt 0 ]]; then
  exit 1
fi
