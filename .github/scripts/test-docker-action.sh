#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2026 The semrel Authors

set -euo pipefail

normal_image="semrel:ownership-test-alpine"
action_image="semrel:ownership-test-action"
fixture="${PWD}/.docker-action-test"

cleanup() {
  docker run --rm \
    --entrypoint /bin/sh \
    -v "${fixture}:/cleanup" \
    "${action_image}" \
    -c 'rm -rf /cleanup/.semrel' >/dev/null 2>&1 || true
  rm -rf "${fixture}"
}
trap cleanup EXIT

rm -rf "${fixture}"
mkdir -p "${fixture}"
cat >"${fixture}/.semrel.lock" <<'JSON'
{
  "semrelLockVersion": 1,
  "updatedAt": "2026-01-01T00:00:00Z",
  "plugins": [
    {
      "binaryName": "semrel-plugin-publisher-test",
      "ref": "@semrel/publisher-test",
      "version": "0.0.0",
      "checksums": {}
    }
  ]
}
JSON
chmod 0755 "${fixture}"

docker build --target alpine -t "${normal_image}" .
docker build --target action -t "${action_image}" .

test "$(docker image inspect --format '{{.Config.User}}' "${normal_image}")" = "semrel"
test "$(docker image inspect --format '{{.Config.User}}' "${action_image}")" = "root"

set +e
normal_output="$(docker run --rm \
  -e SEMREL_REGISTRY_URL=http://127.0.0.1:1 \
  -v "${fixture}:/github/workspace" \
  -w /github/workspace \
  "${normal_image}" plugin restore 2>&1)"
normal_status=$?
set -e

if [[ ${normal_status} -eq 0 ]] || ! grep -qi "permission denied" <<<"${normal_output}"; then
  echo "normal image did not reproduce the GitHub Actions ownership failure" >&2
  echo "${normal_output}" >&2
  exit 1
fi

set +e
action_output="$(docker run --rm \
  -e GITHUB_ACTIONS=true \
  -e SEMREL_REGISTRY_URL=http://127.0.0.1:1 \
  -v "${fixture}:/github/workspace" \
  -w /github/workspace \
  "${action_image}" plugin restore 2>&1)"
action_status=$?
set -e

if [[ ${action_status} -eq 0 ]]; then
  echo "action restore unexpectedly reached the unavailable test registry" >&2
  exit 1
fi
if grep -qi "permission denied" <<<"${action_output}" || [[ ! -d "${fixture}/.semrel" ]]; then
  echo "action image could not create its plugin cache in the mounted workspace" >&2
  echo "${action_output}" >&2
  exit 1
fi

echo "GitHub Actions image can write the mounted workspace; normal image remains non-root."
