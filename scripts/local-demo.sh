#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2026 The semrel Authors

set -euo pipefail

# local-demo.sh — production-like local demo for semrel + real plugin binaries.
#
# What it does:
# 1) Builds semrel from this repo
# 2) Builds 2 real plugin repos (condition-generic, updater-go)
# 3) Creates a temporary git repo with conventional commits
# 4) Runs semrel release (default: real run, optional --dry-run)
# 5) Prints proof that plugins were used (updated version file + created tag)

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
SEMREL_ROOT="$(cd -- "$SCRIPT_DIR/.." && pwd)"
WORKSPACE_ROOT="$(cd -- "$SEMREL_ROOT/.." && pwd)"

BIN_DIR="$SEMREL_ROOT/bin"
SEMREL_BIN="$BIN_DIR/semrel"
PLUGIN_DIR="${SEMREL_DEMO_PLUGIN_DIR:-$HOME/.semrel/plugins}"
DEMO_REPO_BASE="${SEMREL_DEMO_BASE_DIR:-$WORKSPACE_ROOT}"

RUN_MODE="real"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      RUN_MODE="dry-run"
      shift
      ;;
    --help|-h)
      cat <<'EOF'
Usage: ./scripts/local-demo.sh [--dry-run]

Options:
  --dry-run   Do not create a real release tag in the demo repository.
  -h, --help  Show this help.

Environment variables:
  SEMREL_DEMO_PLUGIN_DIR   Target directory for built demo plugins
                           (default: ~/.semrel/plugins)
  SEMREL_DEMO_BASE_DIR     Base directory where the temporary demo repo is created
                           (default: workspace root next to semrel)
EOF
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

need_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "Missing required command: $1" >&2
    exit 1
  fi
}

step() {
  echo ""
  echo "==> $1"
}

ok() {
  echo "  [OK] $1"
}

need_cmd go
need_cmd git

step "Build semrel"
mkdir -p "$BIN_DIR"
VERSION="$(git -C "$SEMREL_ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)"
LDFLAGS="-X github.com/SemRels/semrel/internal/cli.version=$VERSION"
go -C "$SEMREL_ROOT" build -trimpath -ldflags "$LDFLAGS" -o "$SEMREL_BIN" ./cmd/semrel
ok "Built $SEMREL_BIN"

step "Build plugin binaries"
mkdir -p "$PLUGIN_DIR"

build_plugin() {
  local repo="$1"
  local bin_name="$2"
  local repo_dir="$WORKSPACE_ROOT/$repo"
  local out="$PLUGIN_DIR/$bin_name"

  if [[ ! -f "$repo_dir/cmd/plugin/main.go" ]]; then
    echo "Missing plugin repository or entrypoint: $repo_dir/cmd/plugin/main.go" >&2
    exit 1
  fi

  go -C "$repo_dir" build -trimpath -o "$out" ./cmd/plugin
  chmod +x "$out"
  ok "$repo -> $out"
}

build_plugin "condition-generic" "semrel-plugin-condition-generic"
build_plugin "updater-go" "semrel-plugin-updater-go"

step "Create demo git repository"
DEMO_REPO="$(mktemp -d "$DEMO_REPO_BASE/semrel-prod-demo-XXXXXX")"
git -C "$DEMO_REPO" init -b main >/dev/null
git -C "$DEMO_REPO" config user.email demo@semrel.local
git -C "$DEMO_REPO" config user.name "semrel demo"
git -C "$DEMO_REPO" config commit.gpgsign false
git -C "$DEMO_REPO" config tag.gpgsign false

mkdir -p "$DEMO_REPO/internal/version"
cat >"$DEMO_REPO/internal/version/version.go" <<'EOF'
package version

const Version = "0.1.0"
EOF

cat >"$DEMO_REPO/README.md" <<'EOF'
# semrel demo repo
EOF

git -C "$DEMO_REPO" add .
git -C "$DEMO_REPO" commit -m "chore: initial commit" >/dev/null
git -C "$DEMO_REPO" tag v0.1.0

echo "feature" >"$DEMO_REPO/feature.txt"
git -C "$DEMO_REPO" add feature.txt
git -C "$DEMO_REPO" commit -m "feat: add demo capability" >/dev/null

echo "fix" >"$DEMO_REPO/fix.txt"
git -C "$DEMO_REPO" add fix.txt
git -C "$DEMO_REPO" commit -m "fix: patch demo behavior" >/dev/null

ok "Demo repository: $DEMO_REPO"

step "Write semrel config"
cat >"$DEMO_REPO/.semrel.yaml" <<EOF
schemaVersion: 1
tagPrefix: "v"
branches:
  - name: main
rules:
  - type: feat
    bump: minor
  - type: fix
    bump: patch
plugins:
  - path: "$PLUGIN_DIR/semrel-plugin-condition-generic"
    phase: condition
    args:
      command: "test -f internal/version/version.go"
  - path: "$PLUGIN_DIR/semrel-plugin-updater-go"
    phase: pre-tag
    args:
      file: internal/version/version.go
      var_name: Version
EOF
ok "Wrote .semrel.yaml"

step "Run semrel release"
if [[ "$RUN_MODE" == "dry-run" ]]; then
  (
    cd "$DEMO_REPO"
    "$SEMREL_BIN" release --dry-run --config .semrel.yaml
  )
else
  (
    cd "$DEMO_REPO"
    "$SEMREL_BIN" release --config .semrel.yaml
  )
fi

step "Result checks"
echo "Tags:"
git -C "$DEMO_REPO" tag --list | sed 's/^/  - /'

echo ""
echo "internal/version/version.go:"
sed -n '1,20p' "$DEMO_REPO/internal/version/version.go"

echo ""
echo "Latest commit:"
git -C "$DEMO_REPO" log --oneline -1 | sed 's/^/  /'

echo ""
echo "Demo completed successfully."
echo "Inspect repository: $DEMO_REPO"
