# Go version monitoring

The **Go version monitoring** workflow runs weekly (Monday at 06:00 UTC) and
can also be started with **Actions → Go version monitoring → Run workflow**.
It reads the official Go download API, validates the response and selects the
newest stable release. If its major/minor release line is not already in a Go
matrix in the repository workflows, it adds that line to
`.github/workflows/ci.yaml`.

Existing supported versions and prerelease entries are never removed or
replaced. A fixed automation branch is used so repeated runs create or update
one pull request rather than opening duplicates. Malformed or unavailable
release data fails the workflow instead of changing CI.
