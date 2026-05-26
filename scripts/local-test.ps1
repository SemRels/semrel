# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2026 The semrel Authors
#
# local-test.ps1 — end-to-end local test for semrel on Windows.
#
# What it does:
#   1. Builds the semrel binary
#   2. Builds all locally-cloned plugin repos → installs to %USERPROFILE%\.semrel\plugins\
#   3. Creates a temporary git repo with conventional commits
#   4. Runs "semrel release --dry-run" against it
#   5. Reports results
#
# Usage:
#   cd C:\Users\mwald\semrel\semrel-core
#   .\scripts\local-test.ps1
#
#   Options:
#     -PluginBase  <path>   Root folder containing plugin repos (default: parent of this repo)
#     -PluginDir   <path>   Where to install plugin .exe files  (default: ~\.semrel\plugins)
#     -SkipBuild            Skip build step (use previously built binaries)
#     -Verbose              Show full plugin output

[CmdletBinding()]
param(
    [string]$PluginBase,
    [string]$PluginDir   = (Join-Path $env:USERPROFILE ".semrel\plugins"),
    [switch]$SkipBuild,
    [switch]$VerboseOutput
)

# Set defaults that depend on $PSScriptRoot (not available in param block defaults)
if (-not $PluginBase) {
    $PluginBase = Resolve-Path (Join-Path $PSScriptRoot "..\..")
}

$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot | Split-Path -Parent   # semrel-core root

function Write-Step($msg) { Write-Host "`n==> $msg" -ForegroundColor Cyan }
function Write-Ok($msg)   { Write-Host "  [PASS] $msg" -ForegroundColor Green }
function Write-Warn($msg) { Write-Host "  [SKIP] $msg" -ForegroundColor Yellow }
function Write-Err($msg)  { Write-Host "  [FAIL] $msg" -ForegroundColor Red }

# ── 1. Build semrel binary ─────────────────────────────────────────────────
Write-Step "Building semrel binary"

$BinDir = Join-Path $Root "bin"
$SemrelBin = Join-Path $BinDir "semrel.exe"

if (-not $SkipBuild) {
    New-Item -ItemType Directory -Force -Path $BinDir | Out-Null
    Push-Location $Root
    try {
        $verRaw = git describe --tags --always --dirty 2>$null
        $ver = if ($verRaw) { $verRaw } else { "dev" }
        $ldflags = "-X github.com/GoSemantics/semrel/internal/cli.version=$ver"
        go build -trimpath -ldflags $ldflags -o $SemrelBin ./cmd/semrel
        if ($LASTEXITCODE -ne 0) { throw "semrel build failed" }
        Write-Ok "semrel.exe built → $SemrelBin"
    } finally { Pop-Location }
} else {
    if (-not (Test-Path $SemrelBin)) { throw "semrel.exe not found at $SemrelBin - run without -SkipBuild first" }
    Write-Warn "Skipped build, using existing $SemrelBin"
}

# ── 2. Build & install plugins ────────────────────────────────────────────
Write-Step "Building plugin binaries → $PluginDir"
New-Item -ItemType Directory -Force -Path $PluginDir | Out-Null

$pass = 0; $skip = 0; $fail = 0
$builtPlugins = @()

Get-ChildItem $PluginBase -Directory | ForEach-Object {
    $repoDir = $_.FullName
    $repoName = $_.Name
    $entrypoint = Join-Path $repoDir "cmd\plugin\main.go"

    if (-not (Test-Path $entrypoint)) { return }
    if ($repoName -eq "plugin-template") { return }  # skip template repo

    $pluginBin = Join-Path $PluginDir "semrel-plugin-$repoName.exe"

    if ($SkipBuild -and (Test-Path $pluginBin)) {
        Write-Warn "$repoName (using cached .exe)"
        $builtPlugins += $repoName
        $skip++
        return
    }

    Push-Location $repoDir
    try {
        go build -trimpath -o $pluginBin ./cmd/plugin 2>&1 | Out-Null
        if ($LASTEXITCODE -eq 0) {
            Write-Ok "$repoName → $pluginBin"
            $builtPlugins += $repoName
            $pass++
        } else {
            Write-Err "$repoName (build failed)"
            $fail++
        }
    } finally { Pop-Location }
}

Write-Host "`n  Plugins built: $pass  cached: $skip  failed: $fail" -ForegroundColor White
if ($fail -gt 0 -and -not $SkipBuild) {
    Write-Host "  (Failed plugins will be skipped in the test)" -ForegroundColor Yellow
}

# ── 3. Create temporary test git repo ────────────────────────────────────
Write-Step "Creating test git repository"

$TestRepo = Join-Path $env:TEMP "semrel-local-test-$(Get-Random)"
New-Item -ItemType Directory -Force -Path $TestRepo | Out-Null

Push-Location $TestRepo
try {
    git init -b main | Out-Null
    git config user.email "test@semrel.local"
    git config user.name  "semrel local test"
    git config commit.gpgsign false
    git config tag.gpgsign false

    # Seed with a v1.0.0 tag so semrel has a baseline
    "# Test repo" | Out-File README.md
    git add . | Out-Null
    git -c commit.gpgsign=false commit -m "chore: initial commit" | Out-Null
    git tag v1.0.0

    # Add some conventional commits that should trigger a patch release
    "change 1" | Out-File change1.txt
    git add . | Out-Null
    git -c commit.gpgsign=false commit -m "fix: correct version handling" | Out-Null

    "change 2" | Out-File change2.txt
    git add . | Out-Null
    git -c commit.gpgsign=false commit -m "fix: handle empty changelog" | Out-Null

    "change 3" | Out-File change3.txt
    git add . | Out-Null
    git -c commit.gpgsign=false commit -m "feat: add --env-file support" | Out-Null

    Write-Ok "Test repo created at $TestRepo (tags: v1.0.0, 3 new commits)"
} catch {
    Pop-Location
    throw
}

# ── 4. Write .semrel.yaml ─────────────────────────────────────────────────
Write-Step "Writing .semrel.yaml"

# Pick the best available analyzer + generator
$analyzer = if ($builtPlugins -contains "analyzer-conventional") { "analyzer-conventional" }
            elseif ($builtPlugins -contains "analyzer-default")  { "analyzer-default" }
            else { $null }

$generator = if ($builtPlugins -contains "generator-changelog-md") { "generator-changelog-md" }
             elseif ($builtPlugins -contains "generator-release-notes") { "generator-release-notes" }
             else { $null }

$pluginLines = @()
if ($analyzer) {
    $pluginLines += "  - uses: $analyzer"
}
if ($generator) {
    $pluginLines += "  - uses: $generator"
}
if ($builtPlugins -contains "condition-generic") {
    $pluginLines += @(
        "  - uses: condition-generic",
        "    env:",
        "      SEMREL_PLUGIN_COMMAND: 'echo ok'"
    )
}

$pluginsYaml = if ($pluginLines.Count -gt 0) {
    "plugins:`n" + ($pluginLines -join "`n")
} else {
    "plugins: []"
}

$semrelYaml = @"
branches:
  - name: main

tagPrefix: "v"

rules:
  - type: feat
    bump: minor
  - type: fix
    bump: patch
  - type: perf
    bump: patch

$pluginsYaml
"@
[System.IO.File]::WriteAllText((Join-Path $TestRepo ".semrel.yaml"), $semrelYaml)

$pluginsDesc = if ($pluginLines.Count -gt 0) { $pluginLines -join ', ' } else { 'none' }
Write-Ok "Config written (plugins: $pluginsDesc)"

# ── 5. Write .env test file ───────────────────────────────────────────────
Write-Step "Writing .env (test values)"
[System.IO.File]::WriteAllText(
    (Join-Path $TestRepo ".env"),
    "# semrel local test .env`n# These are fake values for dry-run`nSEMREL_PLUGIN_TOKEN=fake-token-for-dry-run`nGITHUB_ACTIONS=true`n"
)
Write-Ok ".env written"

# ── 6. Run semrel release --dry-run ──────────────────────────────────────
Write-Step "Running: semrel release --dry-run"

$env:PATH = "$BinDir;$PluginDir;$env:PATH"

try {
    & $SemrelBin release --dry-run --config (Join-Path $TestRepo ".semrel.yaml") --env-file (Join-Path $TestRepo ".env")
    if ($LASTEXITCODE -eq 0) {
        Write-Ok "Dry-run completed successfully"
    } else {
        Write-Err "Dry-run exited with code $LASTEXITCODE"
    }
} catch {
    Write-Err "Error running semrel: $_"
}

# ── 7. Summary ────────────────────────────────────────────────────────────
Write-Step "Summary"
Write-Host "  semrel binary : $SemrelBin" -ForegroundColor White
Write-Host "  plugins dir   : $PluginDir" -ForegroundColor White
Write-Host "  test repo     : $TestRepo" -ForegroundColor White
Write-Host ""
Write-Host "  To explore manually:" -ForegroundColor White
Write-Host "    cd $TestRepo" -ForegroundColor Gray
Write-Host "    $SemrelBin release --dry-run" -ForegroundColor Gray
Write-Host "    $SemrelBin release --dry-run --output json" -ForegroundColor Gray
Write-Host ""

Pop-Location
