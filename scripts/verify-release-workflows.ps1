# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2026 The semrel Authors

param(
    [string]$WorkspaceRoot = (Split-Path $PSScriptRoot -Parent | Split-Path -Parent),
    [switch]$ValidateUpstream
)

$ErrorActionPreference = 'Stop'
$failures = [System.Collections.Generic.List[string]]::new()

function Assert-True {
    param([bool]$Condition, [string]$Message)
    if (-not $Condition) {
        $script:failures.Add($Message)
    }
}

function Read-Workflow([string]$Path) {
    return [IO.File]::ReadAllText($Path).Replace("`r`n", "`n")
}

function Get-JobBlock([string]$Text, [string]$JobName) {
    $pattern = "(?ms)^  $([regex]::Escape($JobName)):\n.*?(?=^  [A-Za-z0-9_-]+:\n|\z)"
    return [regex]::Match($Text, $pattern).Value
}

$officialRepositories = @(
    'analyzer-conventional',
    'analyzer-default',
    'condition-bitbucket-pipelines',
    'condition-circleci',
    'condition-generic',
    'condition-gitea-actions',
    'condition-github-actions',
    'condition-gitlab-ci',
    'generator-changelog-html',
    'generator-changelog-md',
    'generator-release-notes',
    'hook-discord',
    'hook-email',
    'hook-gitplugin',
    'hook-jira',
    'hook-matrix',
    'hook-slack',
    'hook-teams',
    'packager-nfpm',
    'provider-bitbucket',
    'provider-git',
    'provider-gitea',
    'provider-github',
    'provider-gitlab',
    'publisher-crates',
    'publisher-docker',
    'publisher-generic-http',
    'publisher-npm',
    'publisher-oci',
    'publisher-pypi',
    'updater-cargo',
    'updater-composer',
    'updater-docker',
    'updater-go',
    'updater-gradle',
    'updater-helm',
    'updater-homebrew',
    'updater-maven',
    'updater-npm',
    'updater-nuget',
    'updater-pubspec',
    'updater-python',
    'updater-terraform'
)
$dockerRepositories = @(
    'analyzer-conventional',
    'analyzer-default',
    'condition-bitbucket-pipelines',
    'condition-circleci',
    'condition-generic',
    'condition-gitea-actions',
    'condition-github-actions',
    'condition-gitlab-ci',
    'generator-changelog-html',
    'generator-changelog-md',
    'generator-release-notes',
    'hook-discord',
    'hook-email',
    'hook-gitplugin',
    'hook-jira',
    'hook-matrix',
    'hook-slack',
    'packager-nfpm',
    'provider-bitbucket',
    'provider-git',
    'provider-gitea',
    'provider-github',
    'provider-gitlab',
    'publisher-crates',
    'publisher-docker',
    'publisher-generic-http',
    'publisher-npm',
    'publisher-oci',
    'publisher-pypi',
    'updater-cargo',
    'updater-composer',
    'updater-docker',
    'updater-go',
    'updater-gradle',
    'updater-helm',
    'updater-homebrew',
    'updater-maven',
    'updater-npm',
    'updater-nuget',
    'updater-pubspec',
    'updater-python',
    'updater-terraform'
)
$nonDockerRationale = @{
    'hook-teams' = 'No Dockerfile exists and its README does not document a container image; it is released as six standalone plugin binaries.'
}
Assert-True ($officialRepositories.Count -eq 43) "Official repository inventory must contain exactly 43 repositories."
Assert-True ($dockerRepositories.Count -eq 42) "Docker repository inventory must contain exactly 42 repositories."
Assert-True ($nonDockerRationale.Count -eq 1 -and $nonDockerRationale.ContainsKey('hook-teams')) "hook-teams must be the one explicitly classified non-Docker repository."
Assert-True (($officialRepositories | Select-Object -Unique).Count -eq 43) "Official repository inventory contains duplicates."
Assert-True (($dockerRepositories | Where-Object { $_ -notin $officialRepositories }).Count -eq 0) "Docker inventory contains a non-official repository."

$excluded = @('repos', 'tmp-repos', 'semrel-core', 'semrel-docs-work')
$repos = Get-ChildItem -LiteralPath $WorkspaceRoot -Directory |
    Where-Object { $_.Name -notin $excluded -and (Test-Path (Join-Path $_.FullName '.git')) }

foreach ($repoName in $officialRepositories) {
    $repoRoot = Join-Path $WorkspaceRoot $repoName
    Assert-True (Test-Path (Join-Path $repoRoot '.git')) "${repoName}: official repository is absent."
    Assert-True (Test-Path (Join-Path $repoRoot 'go.mod')) "${repoName}: official plugin lacks go.mod."
    Assert-True (Test-Path (Join-Path $repoRoot 'cmd\plugin')) "${repoName}: official plugin lacks cmd/plugin."
    $hasDockerfile = Test-Path (Join-Path $repoRoot 'Dockerfile')
    Assert-True ($hasDockerfile -eq ($repoName -in $dockerRepositories)) "${repoName}: Docker classification does not match repository contents."
    if (-not $hasDockerfile) {
        $readme = [IO.File]::ReadAllText((Join-Path $repoRoot 'README.md'))
        $imagePattern = '(?i)docker\s+pull|ghcr\.io/.+/' + [regex]::Escape($repoName)
        Assert-True ($nonDockerRationale.ContainsKey($repoName)) "${repoName}: non-Docker classification lacks a rationale."
        Assert-True (-not ($readme -match $imagePattern)) "${repoName}: classified non-Docker but documents a container image."
    }
}
$templateSyncPath = Join-Path $WorkspaceRoot 'plugin-template\.github\workflows\sync-template.yml'
$templateSync = Read-Workflow $templateSyncPath
foreach ($repoName in $officialRepositories) {
    Assert-True ($templateSync -match "(?m)^\s+- $([regex]::Escape($repoName))$") "plugin-template: sync inventory omits $repoName."
    Assert-True (-not (Test-Path (Join-Path $WorkspaceRoot "$repoName\.github\workflows\sync-template.yml"))) "${repoName}: official plugin must not act as a competing cross-repository template."
}
Assert-True (-not ($templateSync -match '(?m)^\s+- Dockerfile$|template/Dockerfile')) 'plugin-template: template sync can overwrite ecosystem-specific Dockerfiles.'

$automated = @()
foreach ($repo in $repos) {
    $path = Join-Path $repo.FullName '.github\workflows\semrel-release.yaml'
    if (Test-Path $path) {
        $automated += [PSCustomObject]@{ Repo = $repo.Name; Path = $path; Text = Read-Workflow $path }
    }
}

$actualPlugins = @($automated | Where-Object { $_.Repo -notin @('plugin-template', 'semrel') })
Assert-True ($actualPlugins.Count -eq 43) "Expected all 43 official repositories to have automated release workflows; found $($actualPlugins.Count)."
foreach ($repoName in $officialRepositories) {
    Assert-True ($repoName -in $actualPlugins.Repo) "${repoName}: semrel-release.yaml is missing."
}
foreach ($workflow in $automated) {
    Assert-True (-not ($workflow.Text -match '/cmd/plugin@(main|master)\b')) "$($workflow.Repo): mutable plugin source ref."
    Assert-True ($workflow.Text -match 'condition-github-actions/cmd/plugin@v0\.2\.2') "$($workflow.Repo): condition plugin is not pinned to v0.2.2."
    if ($workflow.Repo -eq 'semrel') {
        $releaseJob = Get-JobBlock $workflow.Text 'release'
        Assert-True (-not ($workflow.Text -match '(?m)^concurrency:$')) 'semrel: exact downstream publication is under top-level concurrency.'
        Assert-True ($releaseJob -match '(?m)^    concurrency:\n(?:      #.*\n)*      group: semrel-orchestrator-\$\{\{ github\.ref \}\}\n      cancel-in-progress: false$') 'semrel: version calculation job is not isolated in the semrel-orchestrator group.'
    } else {
        Assert-True ($workflow.Text -match '(?m)^concurrency:\n(?:  #.*\n)*  group: semrel-orchestrator-\$\{\{ github\.ref \}\}\n  cancel-in-progress: false$') "$($workflow.Repo): version calculation is not isolated in the semrel-orchestrator group."
    }
    Assert-True (-not ($workflow.Text -match '(?m)^  group: release-promotion$')) "$($workflow.Repo): orchestrator still shares exact-release concurrency."

    $pushStep = $workflow.Text.IndexOf('- name: Push tag to GitHub')
    if ($pushStep -ge 0) {
        $remoteLookup = $workflow.Text.IndexOf('git ls-remote', $pushStep)
        $crlfCheck = $workflow.Text.IndexOf("*`$'\r'*", $pushStep)
        $semverCheck = $workflow.Text.IndexOf("semver_re='^v?", $pushStep)
        Assert-True ($crlfCheck -ge $pushStep -and $crlfCheck -lt $remoteLookup) "$($workflow.Repo): CR/LF validation is not before remote lookup."
        Assert-True ($semverCheck -ge $pushStep -and $semverCheck -lt $remoteLookup) "$($workflow.Repo): strict SemVer validation is not before remote lookup."
    }
}

$generated = @()
foreach ($repo in $repos) {
    $path = Join-Path $repo.FullName '.github\workflows\release.yml'
    if (Test-Path $path) {
        $text = Read-Workflow $path
        if ($text -match '(?m)^  docker-validate:$') {
            $generated += [PSCustomObject]@{ Repo = $repo.Name; Path = $path; Text = $text }
        }
    }
}
Assert-True ($generated.Count -eq 43) "Expected 42 official Docker releases plus plugin-template; found $($generated.Count)."
foreach ($repoName in $dockerRepositories) {
    Assert-True ($repoName -in $generated.Repo) "${repoName}: full Docker release architecture is missing."
}
Assert-True ('hook-teams' -notin $generated.Repo) 'hook-teams: non-Docker release was incorrectly given Docker jobs.'
$generatedHashes = @($generated | ForEach-Object {
    # Dependabot can advance an independently pinned setup-go patch release
    # without changing the generated workflow architecture checked below.
    $normalized = $_.Text -replace 'uses: actions/setup-go@[0-9a-f]{40} # v6\.[0-9]+\.[0-9]+', 'uses: actions/setup-go@<PIN> # v6.x.x'
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        [Convert]::ToHexString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($normalized)))
    }
    finally {
        $sha.Dispose()
    }
} | Select-Object -Unique)
Assert-True ($generatedHashes.Count -eq 1) "Generated release workflows have structurally drifted into $($generatedHashes.Count) variants."
foreach ($workflow in $generated) {
    Assert-True (-not ($workflow.Text -match '(?m)^concurrency:$')) "$($workflow.Repo): exact release workflow has top-level concurrency and can lose a pending version."
    Assert-True ($workflow.Text -match 'ref: \$\{\{ needs\.version\.outputs\.source_sha \}\}' -and $workflow.Text -match 'source_epoch=\$\(git show -s --format=%ct HEAD\)') "$($workflow.Repo): release builds are not pinned to deterministic tag source metadata."
    Assert-True ($workflow.Text -match 'name: linux/amd64' -and $workflow.Text -match 'name: linux/arm64') "$($workflow.Repo): validation does not cover both Linux platforms."
    Assert-True ([regex]::Matches($workflow.Text, 'uses: docker/build-push-action@').Count -eq 1) "$($workflow.Repo): Dockerfile is built more than once."
    Assert-True ($workflow.Text -match 'outputs: type=oci,dest=\$\{\{ runner\.temp \}\}/plugin-image-\$\{\{ matrix\.platform\.arch \}\}\.tar') "$($workflow.Repo): validation does not export a per-platform OCI archive."
    Assert-True ($workflow.Text -match 'scan_dir=\$\(mktemp -d "\$\{RUNNER_TEMP\}/plugin-image-\$\{ARCH\}-layout\.XXXXXX"\)') "$($workflow.Repo): OCI scan layout is not created as a fresh directory."
    Assert-True ($workflow.Text -match 'tar -xf "\$\{RUNNER_TEMP\}/plugin-image-\$\{ARCH\}\.tar" -C "\$\{scan_dir\}"') "$($workflow.Repo): OCI archive is not extracted for Trivy."
    Assert-True ($workflow.Text -match 'test -f "\$\{scan_dir\}/index\.json"' -and $workflow.Text -match 'test -f "\$\{scan_dir\}/oci-layout"') "$($workflow.Repo): extracted OCI layout is not validated."
    Assert-True ($workflow.Text -match 'input: \$\{\{ steps\.extract\.outputs\.path \}\}') "$($workflow.Repo): Trivy does not scan the extracted OCI layout directory."
    Assert-True (-not ($workflow.Text -match 'input: .*plugin-image-.*\.tar')) "$($workflow.Repo): Trivy still receives an OCI tar archive."
    Assert-True ($workflow.Text -match 'TRIVY_PLATFORM: \$\{\{ matrix\.platform\.name \}\}') "$($workflow.Repo): Trivy is not constrained to the matrix platform."
    Assert-True ($workflow.Text -match 'name: plugin-image-\$\{\{ matrix\.platform\.arch \}\}\n\s+path: \|\n\s+\$\{\{ runner\.temp \}\}/plugin-image-\$\{\{ matrix\.platform\.arch \}\}\.tar') "$($workflow.Repo): scanned platform archive is not uploaded."
    Assert-True ($workflow.Text -match 'sha256sum "plugin-image-\$\{ARCH\}\.tar" > "plugin-image-\$\{ARCH\}\.tar\.sha256"') "$($workflow.Repo): validation archive checksum is not recorded."
    Assert-True (-not ($workflow.Text -match '(?m)^\s+(load|push): true$')) "$($workflow.Repo): validation or publication bypasses archive promotion."
    $qemuBeforeBuildx = [regex]::Matches(
        $workflow.Text,
        '(?m)^      - name: Set up QEMU\n        uses: docker/setup-qemu-action@96fe6ef7f33517b61c61be40b68a1882f3264fb8 # v4\.2\.0\n        with:\n          platforms: arm64\n\n      - name: Set up Docker Buildx$'
    )
    Assert-True ($qemuBeforeBuildx.Count -eq 1) "$($workflow.Repo): QEMU is not pinned and limited to the validation builder."

    $build = $workflow.Text.IndexOf('- name: Build immutable OCI validation archive')
    $checksum = $workflow.Text.IndexOf('- name: Record validation archive checksum')
    $extract = $workflow.Text.IndexOf('- name: Extract immutable OCI layout for scanning')
    $scan = $workflow.Text.IndexOf('- name: Scan immutable extracted OCI layout')
    $upload = $workflow.Text.IndexOf('- name: Upload scanned OCI validation archive')
    $download = $workflow.Text.IndexOf('- name: Download scanned OCI validation archives')
    $promote = $workflow.Text.IndexOf('- name: Promote scanned platform images')
    $manifest = $workflow.Text.IndexOf('- name: Create final multiarch manifest from scanned digests')
    Assert-True ($build -ge 0 -and $checksum -gt $build -and $extract -gt $checksum -and $scan -gt $extract -and $upload -gt $scan -and $download -gt $upload -and $promote -gt $download -and $manifest -gt $promote) "$($workflow.Repo): build, checksum, extraction, scan, artifact, promotion, and manifest ordering is unsafe."
    Assert-True ($workflow.Text -match '(?m)^    needs: \[version, build, docker-validate\]$') "$($workflow.Repo): image promotion does not depend on binaries and every platform scan."
    $dockerJob = Get-JobBlock $workflow.Text 'docker'
    Assert-True ($dockerJob.Length -gt 0) "$($workflow.Repo): Docker publication job is missing."
    Assert-True ($dockerJob -match '(?m)^    concurrency:\n      group: exact-image-\$\{\{ needs\.version\.outputs\.exact_key \}\}\n      cancel-in-progress: false$') "$($workflow.Repo): exact image publication is not serialized by a case-safe exact-version key."
    Assert-True ($workflow.Text -match 'Actions concurrency groups are case-insensitive' -and $workflow.Text -match 'printf ''exact_key=%s\\n''.*sha256sum') "$($workflow.Repo): exact concurrency key does not preserve case-sensitive SemVer identity."
    Assert-True (-not ($dockerJob -match 'docker/build-push-action@|(?m)^\s+context:\s|(?m)^\s+file:\s')) "$($workflow.Repo): publication still performs a Dockerfile rebuild."
    Assert-True (-not ($dockerJob -match 'setup-qemu-action@')) "$($workflow.Repo): archive-only publication unnecessarily installs QEMU."
    Assert-True ($dockerJob -match 'actions/download-artifact@3e5f45b2cfb9172054b4087a40e8e0b5a5461e7c # v8\.0\.1' -and $dockerJob -match 'pattern: plugin-image-\*') "$($workflow.Repo): publication does not download both scan artifacts with the verified action pin."
    Assert-True ($dockerJob -match 'oras-project/setup-oras@1d808f7d7f6995cc68b7bf507bfe5c5446e1dc9d # v2\.0\.1' -and $dockerJob -match '(?m)^          version: 1\.3\.1$') "$($workflow.Repo): ORAS is not immutably installed at the verified version."
    Assert-True ($dockerJob -match 'sha256sum --check -- plugin-image-\*\.tar\.sha256' -and $dockerJob -match "-eq 2") "$($workflow.Repo): publication does not verify exactly two archive checksums."
    Assert-True ($dockerJob -match 'oras cp --from-oci-layout "\$\{archive\}:\$\{arch\}" "\$\{IMAGE\}:\$\{staging_tag\}"') "$($workflow.Repo): publication does not copy the scanned OCI manifests."
    Assert-True ($dockerJob -match '\[\[ "\$\{pushed_digest\}" == "\$\{source_digest\}" \]\]') "$($workflow.Repo): publication does not prove the pushed digest matches the scanned archive."
    Assert-True ($dockerJob -match 'oras resolve --platform "linux/\$\{arch\}" --oci-layout' -and $dockerJob -match '\[\[ "\$\{pushed_platform_digest\}" == "\$\{platform_digest\}" \]\]') "$($workflow.Repo): publication does not prove the platform image digest matches the scanned archive."
    Assert-True ($dockerJob -match '"\$\{IMAGE\}@\$\{AMD64_DIGEST\}"' -and $dockerJob -match '"\$\{IMAGE\}@\$\{ARM64_DIGEST\}"' -and $dockerJob -match 'docker buildx imagetools create') "$($workflow.Repo): final manifest is not assembled from both promoted digests."
    Assert-True ($dockerJob -match 'steps\.promote\.outputs\.amd64_image' -and $dockerJob -match 'steps\.promote\.outputs\.arm64_image' -and $dockerJob -match 'imagetools inspect --raw' -and $dockerJob -match 'any\(\.manifests\[\]; \.digest == \$amd64' -and $dockerJob -match 'any\(\.manifests\[\]; \.digest == \$arm64') "$($workflow.Repo): final manifest does not verify both scanned platform image digests."
    Assert-True ($dockerJob -match 'resolve_registry_ref\(\)' -and $dockerJob -match 'local attempts=4' -and $dockerJob -match 'return 4' -and $dockerJob -match 'Registry authorization or transient failure') "$($workflow.Repo): exact-tag resolve is not bounded and fail-closed."
    Assert-True (-not ($dockerJob -match 'existing_digest=\$\(oras resolve "\$\{exact_tag\}".*2>/dev/null')) "$($workflow.Repo): exact-tag resolve still treats every ORAS error as absence."
    Assert-True ($dockerJob -match 'Refusing to mutate immutable exact tag' -and $dockerJob -match 'existing_digest=\$\(resolve_registry_ref "\$\{exact_tag\}"' -and $dockerJob -match 'resolve_status != 4' -and $dockerJob -match 'oras tag "\$\{IMAGE\}@\$\{digest\}"' -and $dockerJob -match 'published_digest.*!=.*digest') "$($workflow.Repo): exact image tag is not immutable, digest-checked, and idempotent."
    Assert-True ($dockerJob -match 'DIGEST: \$\{\{ steps\.manifest\.outputs\.digest \}\}') "$($workflow.Repo): signing does not use the promoted multiarch digest."
    $publishJob = Get-JobBlock $workflow.Text 'publish'
    Assert-True ($publishJob -match 'pattern: release-\*') "$($workflow.Repo): binary publication also downloads OCI scan artifacts."
    Assert-True ($dockerJob -match 'type=raw,value=\$\{\{ needs\.version\.outputs\.image_version \}\}') "$($workflow.Repo): exact encoded image tag is missing."
    Assert-True (-not ($dockerJob -match 'type=semver|value=latest')) "$($workflow.Repo): exact publication still writes mutable major, minor, or latest tags."
    Assert-True ($workflow.Text -match '\$\{version_short//\+/_\}') "$($workflow.Repo): build metadata is not encoded with underscore."
    Assert-True ($workflow.Text -match "prerelease: \$\{\{ needs\.version\.outputs\.stable != 'true' \}\}" -and $workflow.Text -match "make_latest: \$\{\{ needs\.version\.outputs\.stable == 'true' \}\}") "$($workflow.Repo): GitHub release prerelease/latest policy is not driven by validated stable output."

    $reconcileJob = Get-JobBlock $workflow.Text 'reconcile'
    Assert-True ($reconcileJob.Length -gt 0) "$($workflow.Repo): mutable-tag reconcile job is missing."
    Assert-True ($reconcileJob -match '(?m)^    concurrency:\n      group: mutable-image-tags\n      cancel-in-progress: false$') "$($workflow.Repo): reconcile job lacks stable non-canceling concurrency."
    Assert-True ($reconcileJob -match 'needs: \[version, docker, publish\]') "$($workflow.Repo): reconcile can run before exact image and release publication."
    Assert-True ($reconcileJob -match "if: needs\.version\.outputs\.stable == 'true'") "$($workflow.Repo): prerelease publication can enter stable mutable-tag reconciliation."
    Assert-True ($reconcileJob -match 'gh api --paginate' -and $reconcileJob -match 'published_at != null' -and $reconcileJob -match 'resolved-exact-images\.tsv') "$($workflow.Repo): reconcile does not query every published release and persist resolved exact digests."
    Assert-True ($reconcileJob -match 'local attempts=4' -and $reconcileJob -match 'resolve_status != 4' -and $reconcileJob -match 'Aborting reconcile before alias mutation' -and $reconcileJob -match 'image_digests\[exact\]') "$($workflow.Repo): reconcile does not distinguish definitive absence from transient candidate failure before mutation."
    Assert-True (-not ($reconcileJob -match 'oras resolve "\$\{IMAGE\}:\$\{exact\}" 2>/dev/null')) "$($workflow.Repo): reconcile still silently excludes non-definitive candidate resolve failures."
    Assert-True ($reconcileJob -match 'Build metadata is absent from the precedence tuple' -and $reconcileJob -match 'item\[0\], item\[1\], item\[2\], item\[3\]') "$($workflow.Repo): SemVer precedence or equal-precedence tie-break is not deterministic."
    Assert-True ($reconcileJob -match 'by_major' -and $reconcileJob -match 'by_minor' -and $reconcileJob -match '\("latest", winner\(values\)\)') "$($workflow.Repo): reconcile does not globally derive latest, major, and minor aliases."
    Assert-True ($reconcileJob -match 'read -r alias exact version digest' -and $reconcileJob -match 'oras tag "\$\{IMAGE\}@\$\{digest\}" "\$\{alias\}"' -and $reconcileJob -match 'alias_digest=\$\(resolve_registry_ref') "$($workflow.Repo): reconcile does not mutate aliases only from preflighted immutable digests."
    Assert-True (-not ($reconcileJob -match '\beval\b|bash -c|sh -c')) "$($workflow.Repo): reconcile contains an injection-prone command construction."
}

$officialReleases = @()
foreach ($repoName in $officialRepositories) {
    $repoRoot = Join-Path $WorkspaceRoot $repoName
    $path = Join-Path $repoRoot '.github\workflows\release.yml'
    if (-not (Test-Path $path)) {
        $path = Join-Path $repoRoot '.github\workflows\release.yaml'
    }
    Assert-True (Test-Path $path) "${repoName}: release workflow is missing."
    if (Test-Path $path) {
        $officialReleases += [PSCustomObject]@{ Repo = $repoName; Path = $path; Text = Read-Workflow $path }
    }
}
Assert-True ($officialReleases.Count -eq 43) "Expected release workflows for all 43 official repositories; found $($officialReleases.Count)."
foreach ($workflow in $officialReleases) {
    Assert-True ($workflow.Text -match "semver_re='\^v\?\(0\|\[1-9\]\[0-9\]\*\)" -and $workflow.Text -match '\^0\[0-9\]\+\$') "$($workflow.Repo): release version is not validated as strict SemVer."
    Assert-True ($workflow.Text -match 'go build -trimpath' -and $workflow.Text -match 'if-no-files-found: error') "$($workflow.Repo): release build/artifact checks are incomplete."
    Assert-True ($workflow.Text -match 'test "\$\(find dist .*wc -l\)" -eq 6' -and $workflow.Text -match 'sha256sum --check -- \*\.sha256') "$($workflow.Repo): release artifact cardinality or checksums are not validated."
    Assert-True ($workflow.Text -match "prerelease: \$\{\{ needs\.version\.outputs\.stable != 'true' \}\}" -and $workflow.Text -match "make_latest: \$\{\{ needs\.version\.outputs\.stable == 'true' \}\}") "$($workflow.Repo): prerelease/latest release semantics are missing."
    Assert-True ($workflow.Text -match '\(cd dist && sha256sum "\$\{binary\}" > "\$\{binary\}\.sha256"\)') "$($workflow.Repo): individual checksum does not contain a release-asset basename."
    Assert-True (-not ($workflow.Text -match 'sha256sum "dist/\$\{(?:binary|file)\}"')) "$($workflow.Repo): checksum manifest contains a dist/ path."
    Assert-True ($workflow.Text -match '(?m)^            cd dist$') "$($workflow.Repo): aggregate checksums are not generated from within dist."
    Assert-True ($workflow.Text -match "! -name 'checksums\.txt'") "$($workflow.Repo): aggregate checksum input does not explicitly exclude checksums.txt."
    Assert-True ($workflow.Text -match 'done\n\s+\) > "\$\{RUNNER_TEMP\}/checksums\.txt"\n\s+mv "\$\{RUNNER_TEMP\}/checksums\.txt" dist/checksums\.txt') "$($workflow.Repo): checksum manifest is not generated outside dist."
}
Assert-True ((Get-JobBlock ($officialReleases | Where-Object Repo -eq 'hook-teams').Text 'publish') -match '(?m)^    concurrency:\n      group: exact-release-\$\{\{ needs\.version\.outputs\.exact_key \}\}\n      cancel-in-progress: false$') 'hook-teams: exact binary release is not serialized by a case-safe version key.'

foreach ($repoName in $officialRepositories) {
    $ciPath = Join-Path $WorkspaceRoot "$repoName\.github\workflows\ci.yml"
    if (-not (Test-Path $ciPath)) {
        $ciPath = Join-Path $WorkspaceRoot "$repoName\.github\workflows\ci.yaml"
    }
    Assert-True (Test-Path $ciPath) "${repoName}: CI workflow is missing."
    if (Test-Path $ciPath) {
        $ciText = Read-Workflow $ciPath
        Assert-True ($ciText -match '\bgo test\b') "${repoName}: CI does not run Go tests."
    }
}
$nfpmCI = Read-Workflow (Join-Path $WorkspaceRoot 'packager-nfpm\.github\workflows\ci.yml')
$dockerPublisherCI = Read-Workflow (Join-Path $WorkspaceRoot 'publisher-docker\.github\workflows\ci.yml')
$ociCI = Read-Workflow (Join-Path $WorkspaceRoot 'publisher-oci\.github\workflows\ci.yml')
Assert-True ($nfpmCI -match 'goreleaser/nfpm/v2/cmd/nfpm@v2\.43\.0' -and $nfpmCI -match 'go test -count=1 -tags=integration') 'packager-nfpm: ecosystem-specific package integration checks are missing.'
Assert-True ($dockerPublisherCI -match 'registry:2\.8\.3' -and $dockerPublisherCI -match 'SEMREL_TEST_DOCKER_REGISTRY' -and $dockerPublisherCI -match 'go test -count=1 -tags=integration') 'publisher-docker: ecosystem-specific registry integration checks are missing.'
Assert-True ($ociCI -match 'registry:2\.8\.3' -and $ociCI -match 'SEMREL_TEST_OCI_REF') 'publisher-oci: ecosystem-specific registry integration checks are missing.'

$coreFiles = @(
    (Join-Path $WorkspaceRoot 'semrel\.github\workflows\semrel-release.yaml'),
    (Join-Path $WorkspaceRoot 'semrel\.github\workflows\docker.yml'),
    (Join-Path $WorkspaceRoot 'semrel\.github\workflows\release.yaml')
)
$coreOrchestrator = Read-Workflow $coreFiles[0]
Assert-True (-not ($coreOrchestrator -match '(?m)^concurrency:$')) 'semrel/semrel-release.yaml: exact downstream publication has top-level concurrency.'
$coreReleaseJob = Get-JobBlock $coreOrchestrator 'release'
Assert-True ($coreReleaseJob -match '(?m)^      group: semrel-orchestrator-\$\{\{ github\.ref \}\}$') 'semrel/semrel-release.yaml: version calculation job concurrency is not isolated.'
foreach ($path in $coreFiles[1..2]) {
    $text = Read-Workflow $path
    Assert-True (-not ($text -match '(?m)^concurrency:$')) "semrel/$([IO.Path]::GetFileName($path)): exact publication has top-level concurrency."
}
$coreDockerWorkflow = Read-Workflow $coreFiles[1]
$coreTagRelease = Read-Workflow $coreFiles[2]
$coreDockerJob = Get-JobBlock $coreDockerWorkflow 'docker'
Assert-True ($coreDockerJob -match '(?m)^    concurrency:\n      group: exact-image-\$\{\{ needs\.version\.outputs\.exact_key \}\}\n      cancel-in-progress: false$') 'semrel/docker.yml: exact publication is not serialized by a case-safe version key.'
Assert-True ($coreTagRelease -match '(?m)^      actions: write$' -and $coreTagRelease -match 'name: Dispatch exact Docker publication' -and $coreTagRelease -match 'gh workflow run docker\.yml' -and $coreTagRelease -match '--ref "\$\{CANONICAL_TAG\}"' -and $coreTagRelease -match '--field "version=\$\{NORMALIZED_VERSION\}"') 'semrel/release.yaml: successful GoReleaser does not explicitly dispatch normalized Docker publication with actions permission.'
Assert-True ($coreTagRelease -match 'name: Validate release tag' -and $coreTagRelease.IndexOf('name: Validate release tag') -lt $coreTagRelease.IndexOf('name: Run GoReleaser') -and $coreTagRelease -match 'Numeric prerelease identifiers must not contain leading zeros') 'semrel/release.yaml: tag is not strict-SemVer validated before GoReleaser.'
Assert-True ($coreTagRelease.IndexOf('name: Dispatch exact Docker publication') -gt $coreTagRelease.IndexOf('name: Run GoReleaser')) 'semrel/release.yaml: Docker dispatch can run before GoReleaser succeeds.'
Assert-True ($coreTagRelease -match 'workflow_dispatch is intentionally emitted by GITHUB_TOKEN') 'semrel/release.yaml: workflow_dispatch token exception and duplicate-run semantics are undocumented.'

$coreRegistryE2E = Read-Workflow (Join-Path $WorkspaceRoot 'semrel\.github\workflows\core-registry-e2e.yml')
Assert-True ($coreRegistryE2E -match '# provider-git v0\.3\.0\n\s+ref: 722a1e3e4e22f3adcf69398d8b966a33037bfee9') 'core-registry-e2e does not use the peeled provider-git v0.3.0 commit.'

$releaseFlowNames = @('release.yml', 'release.yaml', 'semrel-release.yaml', 'docker.yml')
foreach ($repo in $repos) {
    foreach ($name in $releaseFlowNames) {
        $path = Join-Path $repo.FullName ".github\workflows\$name"
        if (Test-Path $path) {
            $text = Read-Workflow $path
            Assert-True (-not ($text -match '(?m)^\s*queue(?:\s*:|:max)')) "$($repo.Name)/$name uses nonexistent Actions queue syntax."
            Assert-True (-not ($text -match '(?m)^  group: release-promotion$')) "$($repo.Name)/$name still uses the lossy shared release-promotion group."
            if ($name -eq 'semrel-release.yaml') {
                if ($repo.Name -eq 'semrel') {
                    $releaseJob = Get-JobBlock $text 'release'
                    Assert-True (-not ($text -match '(?m)^concurrency:$')) "$($repo.Name)/$name puts exact downstream jobs in a replaceable pending slot."
                    Assert-True ($releaseJob -match '(?m)^      group: semrel-orchestrator-\$\{\{ github\.ref \}\}$' -and $releaseJob -match '(?m)^      cancel-in-progress: false$') "$($repo.Name)/$name does not isolate version calculation at job scope."
                } else {
                    Assert-True ($text -match '(?m)^  group: semrel-orchestrator-\$\{\{ github\.ref \}\}$') "$($repo.Name)/$name does not isolate version orchestration."
                    Assert-True ($text -match '(?m)^  cancel-in-progress: false$') "$($repo.Name)/$name can cancel a running version calculation."
                }
            } else {
                Assert-True (-not ($text -match '(?m)^concurrency:$')) "$($repo.Name)/$name has top-level concurrency, so an exact pending release can be replaced."
            }
        }
    }
}

foreach ($path in $coreFiles[0..1]) {
    $text = Read-Workflow $path
    $fileName = [IO.Path]::GetFileName($path)
    Assert-True ($text -match 'target: \[release, alpine, action\]') "${fileName}: all three variants are not validated."
    Assert-True ($text -match 'name: linux/amd64' -and $text -match 'name: linux/arm64') "${fileName}: both platforms are not validated."
    Assert-True ([regex]::Matches($text, 'uses: docker/build-push-action@').Count -eq 1) "${fileName}: Dockerfile is built more than once."
    Assert-True ($text -match 'outputs: type=oci,dest=\$\{\{ runner\.temp \}\}/semrel-image-\$\{\{ matrix\.target \}\}-\$\{\{ matrix\.platform\.arch \}\}\.tar') "${fileName}: validation does not export all variant/platform OCI archives."
    Assert-True ($text -match 'scan_dir=\$\(mktemp -d "\$\{RUNNER_TEMP\}/semrel-image-\$\{TARGET\}-\$\{ARCH\}-layout\.XXXXXX"\)') "${fileName}: OCI scan layout is not created as a fresh directory."
    Assert-True ($text -match 'tar -xf "\$\{RUNNER_TEMP\}/semrel-image-\$\{TARGET\}-\$\{ARCH\}\.tar" -C "\$\{scan_dir\}"') "${fileName}: OCI archive is not extracted for Trivy."
    Assert-True ($text -match 'test -f "\$\{scan_dir\}/index\.json"' -and $text -match 'test -f "\$\{scan_dir\}/oci-layout"') "${fileName}: extracted OCI layout is not validated."
    Assert-True ($text -match 'input: \$\{\{ steps\.extract\.outputs\.path \}\}') "${fileName}: Trivy does not scan the extracted OCI layout directory."
    Assert-True (-not ($text -match 'input: .*semrel-image-.*\.tar')) "${fileName}: Trivy still receives an OCI tar archive."
    Assert-True ($text -match 'TRIVY_PLATFORM: \$\{\{ matrix\.platform\.name \}\}') "${fileName}: Trivy is not constrained to the matrix platform."
    Assert-True ($text -match 'name: semrel-image-\$\{\{ matrix\.target \}\}-\$\{\{ matrix\.platform\.arch \}\}\n\s+path: \|\n\s+\$\{\{ runner\.temp \}\}/semrel-image-') "${fileName}: scanned archives are not uploaded per variant and platform."
    Assert-True (-not ($text -match '(?m)^\s+(load|push): true$')) "${fileName}: validation or publication bypasses archive promotion."
    Assert-True ($text -match '\$\{normalized//\+/_\}') "${fileName}: build metadata encoding is not collision-free."
    $build = $text.IndexOf('- name: Build immutable OCI validation archive')
    $checksum = $text.IndexOf('- name: Record validation archive checksum')
    $extract = $text.IndexOf('- name: Extract immutable OCI layout for scanning')
    $scan = $text.IndexOf('- name: Scan immutable extracted OCI layout')
    $upload = $text.IndexOf('- name: Upload scanned OCI validation archive')
    $download = $text.IndexOf('- name: Download scanned OCI validation archives')
    $promote = $text.IndexOf('- name: Promote scanned platform images')
    $manifest = $text.IndexOf('- name: Create final multiarch manifests from scanned digests')
    Assert-True ($build -ge 0 -and $checksum -gt $build -and $extract -gt $checksum -and $scan -gt $extract -and $upload -gt $scan -and $download -gt $upload -and $promote -gt $download -and $manifest -gt $promote) "${fileName}: build, checksum, extraction, scan, artifact, promotion, and manifest ordering is unsafe."
    Assert-True ($text -match '(?m)^    needs: \[(release, docker-validate|version, validate)\]$') "${fileName}: publish job does not depend on every variant/platform scan."
    $dockerJob = Get-JobBlock $text 'docker'
    Assert-True ($dockerJob.Length -gt 0) "${fileName}: Docker publication job is missing."
    if ($fileName -eq 'semrel-release.yaml') {
        Assert-True ($dockerJob -match '(?m)^    concurrency:\n      group: exact-image-\$\{\{ needs\.release\.outputs\.exact_key \}\}\n      cancel-in-progress: false$') "${fileName}: exact publication is not serialized by a case-safe version key."
        Assert-True ($text -match 'build_date: \$\{\{ steps\.tag\.outputs\.build_date \}\}' -and $text -match 'source_sha: \$\{\{ steps\.tag\.outputs\.source_sha \}\}') "${fileName}: exact artifacts do not use reproducible tag source metadata."
    } else {
        Assert-True ($dockerJob -match '(?m)^    concurrency:\n      group: exact-image-\$\{\{ needs\.version\.outputs\.exact_key \}\}\n      cancel-in-progress: false$') "${fileName}: exact publication is not serialized by a case-safe version key."
        Assert-True ($text -match 'ref: \$\{\{ needs\.version\.outputs\.source_sha \}\}' -and $text -match 'source_epoch=\$\(git show -s --format=%ct HEAD\)') "${fileName}: exact artifacts do not use reproducible tag source metadata."
    }
    Assert-True (-not ($dockerJob -match 'docker/build-push-action@|(?m)^\s+context:\s|(?m)^\s+file:\s')) "${fileName}: publication still performs a Dockerfile rebuild."
    Assert-True (-not ($dockerJob -match 'setup-qemu-action@')) "${fileName}: archive-only publication unnecessarily installs QEMU."
    Assert-True ($dockerJob -match 'pattern: semrel-image-\*' -and $dockerJob -match 'merge-multiple: true') "${fileName}: publication does not download all six scan artifacts."
    Assert-True ($dockerJob -match 'oras-project/setup-oras@1d808f7d7f6995cc68b7bf507bfe5c5446e1dc9d # v2\.0\.1' -and $dockerJob -match '(?m)^          version: 1\.3\.1$') "${fileName}: ORAS is not immutably installed at the verified version."
    Assert-True ($dockerJob -match "-eq 6" -and $dockerJob -match 'sha256sum --check -- semrel-image-\*\.tar\.sha256') "${fileName}: publication does not verify exactly six archive checksums."
    Assert-True ($dockerJob -match 'for target in release alpine action' -and $dockerJob -match 'for arch in amd64 arm64') "${fileName}: publication does not promote every variant/platform pair."
    Assert-True ($dockerJob -match 'oras cp --from-oci-layout "\$\{archive\}:\$\{target\}-\$\{arch\}" "\$\{IMAGE\}:\$\{staging_tag\}"') "${fileName}: publication does not copy the scanned OCI manifests."
    Assert-True ($dockerJob -match '\[\[ "\$\{pushed_digest\}" == "\$\{source_digest\}" \]\]') "${fileName}: publication does not prove pushed digests match scanned archives."
    Assert-True ($dockerJob -match 'oras resolve --platform "linux/\$\{arch\}" --oci-layout' -and $dockerJob -match '\[\[ "\$\{pushed_platform_digest\}" == "\$\{platform_digest\}" \]\]') "${fileName}: publication does not prove platform image digests match scanned archives."
    Assert-True ($dockerJob -match 'steps\.promote\.outputs\.release_amd64' -and $dockerJob -match 'steps\.promote\.outputs\.release_arm64' -and $dockerJob -match 'steps\.promote\.outputs\.alpine_amd64' -and $dockerJob -match 'steps\.promote\.outputs\.alpine_arm64' -and $dockerJob -match 'steps\.promote\.outputs\.action_amd64' -and $dockerJob -match 'steps\.promote\.outputs\.action_arm64') "${fileName}: final manifests do not consume all six promoted digests."
    Assert-True ($dockerJob -match 'steps\.promote\.outputs\.release_amd64_image' -and $dockerJob -match 'steps\.promote\.outputs\.release_arm64_image' -and $dockerJob -match 'steps\.promote\.outputs\.alpine_amd64_image' -and $dockerJob -match 'steps\.promote\.outputs\.alpine_arm64_image' -and $dockerJob -match 'steps\.promote\.outputs\.action_amd64_image' -and $dockerJob -match 'steps\.promote\.outputs\.action_arm64_image') "${fileName}: final manifests do not verify all six scanned platform image digests."
    Assert-True ([regex]::Matches($dockerJob, 'create_manifest (distroless|alpine|action) ').Count -eq 3 -and $dockerJob -match 'docker buildx imagetools create') "${fileName}: all three final multiarch manifests are not assembled from promoted digests."
    Assert-True ($dockerJob -match 'imagetools inspect --raw' -and $dockerJob -match 'any\(\.manifests\[\]; \.digest == \$amd64' -and $dockerJob -match 'any\(\.manifests\[\]; \.digest == \$arm64') "${fileName}: final manifests do not verify both scanned platform children."
    Assert-True ($dockerJob -match 'resolve_registry_ref\(\)' -and $dockerJob -match 'local attempts=4' -and $dockerJob -match 'return 4' -and $dockerJob -match 'Registry authorization or transient failure') "${fileName}: exact resolve is not bounded and fail-closed."
    Assert-True (-not ($dockerJob -match 'existing_digest=\$\(oras resolve "\$\{exact_tag\}".*2>/dev/null')) "${fileName}: every exact ORAS failure is still treated as absence."
    Assert-True ($dockerJob -match 'Refusing to mutate immutable exact tag' -and $dockerJob -match 'existing_digest=\$\(resolve_registry_ref "\$\{exact_tag\}"' -and $dockerJob -match 'resolve_status != 4' -and $dockerJob -match 'published_digest.*!=.*digest') "${fileName}: exact image tags are not immutable, digest-checked, and idempotent."
    Assert-True ($dockerJob -match 'steps\.manifests\.outputs\.distroless' -and $dockerJob -match 'steps\.manifests\.outputs\.alpine' -and $dockerJob -match 'steps\.manifests\.outputs\.action') "${fileName}: signing does not use all promoted manifest digests."
    Assert-True (-not ($dockerJob -match 'type=semver|value=latest')) "${fileName}: exact publication still writes mutable major, minor, or latest tags."
    Assert-True ($dockerJob -match 'image_version \}\}-alpine' -and $dockerJob -match 'image_version \}\}-action') "${fileName}: exact variant tag semantics are incomplete."

    $reconcileJob = Get-JobBlock $text 'reconcile'
    Assert-True ($reconcileJob.Length -gt 0) "${fileName}: mutable-tag reconcile job is missing."
    Assert-True ($reconcileJob -match '(?m)^    concurrency:\n      group: mutable-image-tags\n      cancel-in-progress: false$') "${fileName}: reconcile job lacks stable non-canceling concurrency."
    if ($fileName -eq 'semrel-release.yaml') {
        Assert-True ($reconcileJob -match 'needs: \[release, docker, publish\]' -and $reconcileJob -match "needs\.release\.outputs\.stable == 'true'") "${fileName}: reconcile can run before exact publication or for a prerelease."
    } else {
        Assert-True ($reconcileJob -match 'needs: \[version, docker\]' -and $reconcileJob -match "needs\.version\.outputs\.stable == 'true'") "${fileName}: reconcile can run before exact publication or for a prerelease."
    }
    Assert-True ($reconcileJob -match 'gh api --paginate' -and $reconcileJob -match 'published_at != null' -and $reconcileJob -match 'for suffix in '''' -alpine -action') "${fileName}: reconcile does not query all published releases and exact image variants."
    Assert-True ($reconcileJob -match 'resolved-exact-images\.tsv' -and $reconcileJob -match 'local attempts=4' -and $reconcileJob -match 'resolve_status == 4' -and $reconcileJob -match 'Aborting reconcile before alias mutation') "${fileName}: reconcile does not fail closed before alias mutation on variant resolve errors."
    Assert-True (-not ($reconcileJob -match 'oras resolve "\$\{IMAGE\}:\$\{exact\}\$\{suffix\}" 2>/dev/null')) "${fileName}: reconcile silently treats transient variant failures as absence."
    Assert-True ($reconcileJob -match 'Build metadata is absent from the precedence tuple' -and $reconcileJob -match 'item\[0\], item\[1\], item\[2\], item\[3\]') "${fileName}: SemVer precedence or equal-precedence tie-break is not deterministic."
    Assert-True ($reconcileJob -match 'by_major' -and $reconcileJob -match 'by_minor' -and $reconcileJob -match '\("latest", winner\(values\)\)') "${fileName}: reconcile does not globally derive latest, major, and minor aliases."
    Assert-True ($reconcileJob -match 'read -r alias exact version digest alpine_digest action_digest' -and $reconcileJob -match 'oras tag "\$\{IMAGE\}@\$\{digest\}" "\$\{alias\}"' -and $reconcileJob -match 'latest-alpine' -and $reconcileJob -match 'latest-action') "${fileName}: reconcile does not update aliases from preflighted immutable variant digests."
    Assert-True (-not ($reconcileJob -match '\beval\b|bash -c|sh -c')) "${fileName}: reconcile contains an injection-prone command construction."
}

function Encode-ImageVersion([string]$Version) {
    return $Version.TrimStart('v').Replace('+', '_')
}
$encoded = @(
    (Encode-ImageVersion 'v1.2.3+build'),
    (Encode-ImageVersion 'v1.2.3-build'),
    (Encode-ImageVersion 'v1.2.3+build.1'),
    (Encode-ImageVersion 'v1.2.3+build.2')
)
Assert-True (($encoded | Select-Object -Unique).Count -eq 4) 'Build metadata image tags collide.'
Assert-True ($encoded[0] -eq '1.2.3_build') 'Build metadata encoding has the wrong exact form.'

function Get-ExactConcurrencyKey([string]$Version) {
    $algorithm = [Security.Cryptography.SHA256]::Create()
    try {
        $bytes = [Text.Encoding]::UTF8.GetBytes($Version.TrimStart('v'))
        return ([BitConverter]::ToString($algorithm.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant()
    } finally {
        $algorithm.Dispose()
    }
}
Assert-True ((Get-ExactConcurrencyKey 'v1.2.3-RC.1') -ne (Get-ExactConcurrencyKey 'v1.2.3-rc.1')) 'Case-sensitive SemVer values collide in case-insensitive Actions concurrency groups.'
Assert-True ((Get-ExactConcurrencyKey 'v1.2.3+build') -eq (Get-ExactConcurrencyKey '1.2.3+build')) 'Optional v prefix changes the exact-version concurrency key.'

function Test-StrictSemVer([string]$Version) {
    if ($Version.Contains("`r") -or $Version.Contains("`n")) {
        return $false
    }
    if ($Version -notmatch '^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$') {
        return $false
    }
    $withoutBuild = $Version.TrimStart('v').Split('+')[0]
    if ($withoutBuild.Contains('-')) {
        foreach ($identifier in $withoutBuild.Substring($withoutBuild.IndexOf('-') + 1).Split('.')) {
            if ($identifier -match '^0[0-9]+$') {
                return $false
            }
        }
    }
    return $true
}

foreach ($valid in @('1.2.3', 'v1.2.3-rc.1', 'v1.2.3+build.1')) {
    Assert-True (Test-StrictSemVer $valid) "Valid SemVer was rejected by the invariant test: $valid."
}
foreach ($invalid in @('v01.2.3', 'v1.2.3-01', "v1.2.3`nmalicious", 'dev', 'v1.2')) {
    Assert-True (-not (Test-StrictSemVer $invalid)) "Invalid SemVer passed the invariant test: $invalid."
}

function Compare-ReconcileCandidate($Left, $Right) {
    foreach ($property in @('Major', 'Minor', 'Patch')) {
        $comparison = $Left.$property.CompareTo($Right.$property)
        if ($comparison -ne 0) {
            return $comparison
        }
    }
    # SemVer precedence ignores build metadata. Ordinal full-version ordering
    # is the documented deterministic tie-break for equal precedence.
    return [string]::CompareOrdinal($Left.Normalized, $Right.Normalized)
}

function Select-GreatestReconcileCandidate([object[]]$Candidates) {
    $best = $null
    foreach ($candidate in $Candidates) {
        if ($null -eq $best -or (Compare-ReconcileCandidate $candidate $best) -gt 0) {
            $best = $candidate
        }
    }
    return $best
}

function Get-ReconcileSelection([string[]]$PublishedVersions, [string[]]$ImageTags) {
    $images = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($imageTag in $ImageTags) {
        [void]$images.Add($imageTag)
    }
    $candidates = @()
    foreach ($version in $PublishedVersions) {
        if (-not (Test-StrictSemVer $version)) {
            continue
        }
        $normalized = $version.TrimStart('v')
        $withoutBuild = $normalized.Split('+')[0]
        if ($withoutBuild.Contains('-')) {
            continue
        }
        $exact = Encode-ImageVersion $version
        if (-not $images.Contains($exact)) {
            continue
        }
        $core = $withoutBuild.Split('.')
        $candidates += [PSCustomObject]@{
            Major = [uint64]$core[0]
            Minor = [uint64]$core[1]
            Patch = [uint64]$core[2]
            Normalized = $normalized
            Exact = $exact
        }
    }
    if ($candidates.Count -eq 0) {
        return @{}
    }
    $selection = @{}
    $selection['latest'] = (Select-GreatestReconcileCandidate $candidates).Exact
    foreach ($group in $candidates | Group-Object Major) {
        $selection[[string]$group.Name] = (Select-GreatestReconcileCandidate @($group.Group)).Exact
    }
    foreach ($group in $candidates | Group-Object { "$($_.Major).$($_.Minor)" }) {
        $selection[[string]$group.Name] = (Select-GreatestReconcileCandidate @($group.Group)).Exact
    }
    return $selection
}

# Model three rapid exact releases where both older pending reconcile jobs are
# replaced. The one eventual run sees the global published set and converges.
$rapidVersions = @('v2.4.1', 'v2.4.3', 'v2.4.2')
$rapidImages = @('2.4.1', '2.4.3', '2.4.2')
$rapidSelection = Get-ReconcileSelection $rapidVersions $rapidImages
Assert-True ($rapidSelection['latest'] -eq '2.4.3') 'Final reconcile after three rapid releases did not select the global latest version.'
Assert-True ($rapidSelection['2'] -eq '2.4.3') 'Final reconcile after three rapid releases regressed the major tag.'
Assert-True ($rapidSelection['2.4'] -eq '2.4.3') 'Final reconcile after three rapid releases regressed the minor tag.'

$tieVersions = @('v1.2.3+build.10', 'v1.2.3+build.2', 'v9.0.0-rc.1', 'not-a-version')
$tieImages = @('1.2.3_build.10', '1.2.3_build.2', '9.0.0-rc.1')
$tieSelection = Get-ReconcileSelection $tieVersions $tieImages
Assert-True ($tieSelection['latest'] -eq '1.2.3_build.2') 'Equal-precedence build metadata tie-break is not stable ordinal ordering.'
Assert-True ($tieSelection['1.2'] -eq '1.2.3_build.2') 'Build metadata incorrectly changed SemVer line selection.'

$missingImageSelection = Get-ReconcileSelection @('v3.0.0', 'v2.9.9') @('2.9.9')
Assert-True ($missingImageSelection['latest'] -eq '2.9.9') 'Reconcile selected a release without a published exact image.'

function Get-RegistryResolveModel([string[]]$AttemptResults) {
    $attempts = [Math]::Min(4, $AttemptResults.Count)
    $last = ''
    for ($index = 0; $index -lt $attempts; $index++) {
        $last = $AttemptResults[$index]
        if ($last -match '^sha256:[0-9a-f]{64}$') {
            return [PSCustomObject]@{ Outcome = 'exists'; Attempts = $index + 1; Digest = $last }
        }
    }
    $fatal = '(^|[^0-9])(401|403|408|409|425|429|5[0-9][0-9])([^0-9]|$)|unauthorized|denied|forbidden|too many requests|timed? out|connection|tls|eof|server error|service unavailable|temporar'
    $absent = '(^|[^0-9])404([^0-9]|$)|manifest[ _]unknown|name[ _]unknown|(^|[^a-z])not found([^a-z]|$)'
    if ($last -match $fatal) {
        return [PSCustomObject]@{ Outcome = 'error'; Attempts = $attempts; Digest = $null }
    }
    if ($last -match $absent) {
        return [PSCustomObject]@{ Outcome = 'absent'; Attempts = $attempts; Digest = $null }
    }
    return [PSCustomObject]@{ Outcome = 'error'; Attempts = $attempts; Digest = $null }
}

$digestA = 'sha256:' + ('a' * 64)
$digestB = 'sha256:' + ('b' * 64)
$transientRecovery = Get-RegistryResolveModel @('503 Service Unavailable', $digestA)
$definitiveAbsent = Get-RegistryResolveModel @('404 manifest unknown', '404 manifest unknown', '404 manifest unknown', '404 manifest unknown')
$authFailure = Get-RegistryResolveModel @('401 Unauthorized', '401 Unauthorized', '401 Unauthorized', '401 Unauthorized')
$transientFailure = Get-RegistryResolveModel @('503 Service Unavailable', '503 Service Unavailable', '503 Service Unavailable', '503 Service Unavailable')
Assert-True ($transientRecovery.Outcome -eq 'exists' -and $transientRecovery.Attempts -eq 2) 'Bounded resolve model did not recover from a transient error.'
Assert-True ($definitiveAbsent.Outcome -eq 'absent' -and $definitiveAbsent.Attempts -eq 4) 'A definitive absence was accepted before bounded retry completed.'
Assert-True ($authFailure.Outcome -eq 'error') 'Authentication failure was incorrectly classified as absence.'
Assert-True ($transientFailure.Outcome -eq 'error') 'Exhausted server failure was incorrectly classified as absence.'

function Invoke-ExactPublicationModel([hashtable]$Tags, [string]$Version, [string]$IntendedDigest) {
    if ($Tags.ContainsKey($Version)) {
        if ($Tags[$Version] -eq $IntendedDigest) {
            return 'identical'
        }
        return 'conflict'
    }
    $Tags[$Version] = $IntendedDigest
    return 'created'
}

$exactTags = @{}
Assert-True ((Invoke-ExactPublicationModel $exactTags '1.2.3' $digestA) -eq 'created') 'First exact publication was not created.'
Assert-True ((Invoke-ExactPublicationModel $exactTags '1.2.3' $digestA) -eq 'identical') 'Duplicate same-version/same-digest publication was not idempotent.'
Assert-True ((Invoke-ExactPublicationModel $exactTags '1.2.3' $digestB) -eq 'conflict') 'Concurrent same-version/different-digest publication did not hard-conflict.'
Assert-True ($exactTags['1.2.3'] -eq $digestA) 'Conflicting exact publication overwrote the winning digest.'

function Invoke-ReconcilePreflightModel([hashtable]$Candidates) {
    $resolved = @{}
    foreach ($candidate in $Candidates.Keys) {
        $result = Get-RegistryResolveModel $Candidates[$candidate]
        if ($result.Outcome -eq 'error') {
            return [PSCustomObject]@{ Succeeded = $false; AliasesTouched = $false; Resolved = @{} }
        }
        if ($result.Outcome -eq 'exists') {
            $resolved[$candidate] = $result.Digest
        }
    }
    return [PSCustomObject]@{ Succeeded = $true; AliasesTouched = $false; Resolved = $resolved }
}

$failedPreflight = Invoke-ReconcilePreflightModel @{
    '3.0.0' = @('503 Service Unavailable', '503 Service Unavailable', '503 Service Unavailable', '503 Service Unavailable')
    '2.9.9' = @($digestA)
}
$absentPreflight = Invoke-ReconcilePreflightModel @{
    '3.0.0' = @('404 manifest unknown', '404 manifest unknown', '404 manifest unknown', '404 manifest unknown')
    '2.9.9' = @($digestA)
}
Assert-True (-not $failedPreflight.Succeeded -and -not $failedPreflight.AliasesTouched) 'Transient candidate failure did not abort reconcile before all alias mutation.'
Assert-True ($absentPreflight.Succeeded -and $absentPreflight.Resolved.Count -eq 1 -and $absentPreflight.Resolved.ContainsKey('2.9.9')) 'Definitively absent candidate was not safely excluded.'

$go125Repositories = @(
    'condition-generic',
    'condition-gitlab-ci',
    'hook-email',
    'hook-gitplugin',
    'hook-matrix',
    'hook-slack',
    'publisher-docker',
    'updater-composer',
    'updater-docker',
    'updater-helm',
    'updater-npm',
    'updater-pubspec',
    'updater-python'
)
Assert-True ($go125Repositories.Count -eq 13) 'Go 1.25 Dockerfile allowlist must contain exactly 13 repositories.'
foreach ($repoName in $go125Repositories) {
    $dockerfile = Join-Path $WorkspaceRoot "$repoName\Dockerfile"
    $dockerText = [IO.File]::ReadAllText($dockerfile)
    Assert-True ($dockerText -match '(?m)^FROM --platform=\$BUILDPLATFORM golang:1\.25-alpine AS build\r?$') "$repoName/Dockerfile is not using the repository-standard Go 1.25 Alpine builder."
    if (-not (Test-Path (Join-Path $WorkspaceRoot "$repoName\go.sum"))) {
        Assert-True (-not ($dockerText -match '(?m)^COPY go\.mod go\.sum ')) "$repoName/Dockerfile requires a nonexistent go.sum."
    }
}
$dependencyFreeDockerModules = @(
    'condition-bitbucket-pipelines',
    'condition-circleci',
    'provider-github',
    'provider-gitlab',
    'updater-cargo',
    'updater-go',
    'updater-gradle',
    'updater-homebrew',
    'updater-maven',
    'updater-nuget',
    'updater-terraform'
)
Assert-True ($dependencyFreeDockerModules.Count -eq 11) 'Dependency-free Docker module inventory must contain exactly 11 repositories.'
foreach ($repoName in $dependencyFreeDockerModules) {
    $repoRoot = Join-Path $WorkspaceRoot $repoName
    $moduleText = [IO.File]::ReadAllText((Join-Path $repoRoot 'go.mod'))
    $dockerText = [IO.File]::ReadAllText((Join-Path $repoRoot 'Dockerfile'))
    Assert-True (-not ($moduleText -match '(?m)^\s*require(?:\s|\()')) "${repoName}: classified dependency-free but go.mod contains requirements; generate and copy go.sum instead."
    Assert-True (-not (Test-Path (Join-Path $repoRoot 'go.sum'))) "${repoName}: dependency-free module unexpectedly has go.sum; classification must be revisited."
    Assert-True ($dockerText -match '(?m)^COPY go\.mod \./\r?$') "${repoName}: dependency-free Docker build must copy go.mod only."
    Assert-True (-not ($dockerText -match '(?m)^COPY go\.mod go\.sum ')) "${repoName}: Dockerfile copies a nonexistent go.sum."
    Assert-True ($dockerText -match '(?m)^RUN go mod download\r?$') "${repoName}: Docker dependency layer no longer validates the dependency-free module."
}
foreach ($repo in $repos) {
    Get-ChildItem -LiteralPath $repo.FullName -Filter Dockerfile -File -Recurse -ErrorAction SilentlyContinue |
        ForEach-Object {
            $dockerText = [IO.File]::ReadAllText($_.FullName)
            Assert-True (-not ($dockerText -match 'golang:1\.24(?:[.-]|$)')) "$($repo.Name)/$($_.FullName.Substring($repo.FullName.Length + 1)) still uses a Go 1.24 builder."
            if ($dockerText -match '(?m)^COPY go\.mod go\.sum ') {
                Assert-True (Test-Path (Join-Path $_.DirectoryName 'go.sum')) "$($repo.Name)/$($_.Name) copies go.sum but the file does not exist."
            }
        }
}

$nfpmDocker = [IO.File]::ReadAllText((Join-Path $WorkspaceRoot 'packager-nfpm\Dockerfile'))
$dockerPublisherDocker = [IO.File]::ReadAllText((Join-Path $WorkspaceRoot 'publisher-docker\Dockerfile'))
$npmDocker = [IO.File]::ReadAllText((Join-Path $WorkspaceRoot 'publisher-npm\Dockerfile'))
$ociDocker = [IO.File]::ReadAllText((Join-Path $WorkspaceRoot 'publisher-oci\Dockerfile'))
$npmReadme = [IO.File]::ReadAllText((Join-Path $WorkspaceRoot 'publisher-npm\README.md'))
Assert-True ($nfpmDocker -match 'goreleaser/nfpm/v2/cmd/nfpm@v2\.43\.0' -and $nfpmDocker -match 'COPY --from=build /out/nfpm') 'packager-nfpm: published image does not include its pinned nFPM runtime dependency.'
Assert-True ($dockerPublisherDocker -match '(?m)^FROM docker:28\.3\.3-cli AS docker-cli\r?$' -and $dockerPublisherDocker -match 'COPY --from=docker-cli /usr/local/bin/docker') 'publisher-docker: published image does not include its pinned Docker CLI runtime dependency.'
Assert-True ($ociDocker -match 'oras\.land/oras/cmd/oras@v1\.3\.1' -and $ociDocker -match 'COPY --from=build /out/oras') 'publisher-oci: published image does not include its pinned ORAS runtime dependency.'
Assert-True ($npmReadme -match 'docker pull ghcr\.io/semrels/publisher-npm:latest') 'publisher-npm: documented GHCR publication was not detected.'
Assert-True ($npmDocker -match '(?m)^FROM node:24-alpine\r?$' -and $npmDocker -match 'corepack@0\.34\.0' -and $npmDocker -match 'corepack enable' -and $npmDocker -match 'pnpm@10\.14\.0' -and $npmDocker -match 'yarn@1\.22\.22') 'publisher-npm: published image lacks npm/pnpm/yarn runtime support.'

$pins = @{}
foreach ($repo in $repos) {
    Get-ChildItem (Join-Path $repo.FullName '.github\workflows') -Recurse -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Extension -in @('.yml', '.yaml') } |
        ForEach-Object {
            $workflowText = Read-Workflow $_.FullName
            Assert-True (-not ($workflowText -match '(?m)^\s*queue(?:\s*:|:max)')) "$($repo.Name)/$($_.Name): nonexistent Actions queue syntax."
            $lineNumber = 0
            foreach ($line in Get-Content $_.FullName) {
                $lineNumber++
                if ($line -match 'uses:\s+([^\s@]+)@([0-9a-f]{40})(?:\s+#\s*(\S+))?') {
                    $action = $Matches[1]
                    $sha = $Matches[2]
                    $tag = $Matches[3]
                    Assert-True ($tag -match '^v?\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$') "$($repo.Name)/$($_.Name):$lineNumber action comment is not an exact tag."
                    $repoName = (($action -split '/')[0..1] -join '/')
                    $pins["$repoName|$sha|$tag"] = $true
                } elseif ($line -match 'uses:\s+[^\s@]+@\S+') {
                    $failures.Add("$($repo.Name)/$($_.Name):$lineNumber action is not pinned to a commit SHA.")
                }
            }
        }
}

if ($ValidateUpstream) {
    foreach ($pin in $pins.Keys) {
        $repoName, $sha, $tag = $pin.Split('|')
        $refs = & git ls-remote --tags "https://github.com/$repoName.git" "refs/tags/$tag" "refs/tags/$tag^{}"
        Assert-True ($LASTEXITCODE -eq 0) "$repoName@$tag could not be queried."
        $resolved = @($refs | ForEach-Object { ($_ -split '\s+')[0] })
        Assert-True ($sha -in $resolved) "$repoName@$sha does not resolve from exact tag $tag."
    }
}

if ($failures.Count -gt 0) {
    $failures | ForEach-Object { Write-Error $_ }
    exit 1
}

foreach ($repoName in $officialRepositories) {
    if ($repoName -in $dockerRepositories) {
        Write-Host "classification: $repoName = Docker (repository Dockerfile; full scanned OCI promotion)"
    } else {
        Write-Host "classification: $repoName = non-Docker ($($nonDockerRationale[$repoName]))"
    }
}
Write-Host "Verified all $($officialRepositories.Count) official releases, $($dockerRepositories.Count) Docker architectures, one justified non-Docker release, and $($pins.Count) unique action pins."
