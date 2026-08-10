# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2026 The semrel Authors

# ── build stage ────────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w -X github.com/GoSemantics/semrel/internal/cli.version=${VERSION}" \
    -o /semrel \
    ./cmd/semrel

RUN chmod 0555 /semrel

# ── git deps stage (for distroless) ──────────────────────────────────────────
# Extract git and CA certificates from Debian so they can be copied into the
# distroless/base image (which has glibc but no package manager).
FROM debian:12-slim AS gitdeps
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        git \
        ca-certificates && \
    rm -rf /var/lib/apt/lists/*

# ── distroless release image ───────────────────────────────────────────────────
# gcr.io/distroless/base-debian12 provides glibc so dynamically-linked git works.
# Runs as nonroot (uid 65532) — no shell, no package manager, minimal attack surface.
FROM gcr.io/distroless/base-debian12:nonroot AS release

ARG VERSION=dev
ARG BUILD_DATE
ARG VCS_REF

LABEL org.opencontainers.image.title="semrel" \
      org.opencontainers.image.description="Automated semantic releases for Go projects" \
      org.opencontainers.image.url="https://semrel.io" \
      org.opencontainers.image.source="https://github.com/SemRels/semrel" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.vendor="SemRels"

# git binary and its runtime dependencies (dynamically linked against glibc)
COPY --from=gitdeps /usr/bin/git                /usr/bin/git
COPY --from=gitdeps /usr/lib/git-core/          /usr/lib/git-core/
COPY --from=gitdeps /usr/share/git-core/        /usr/share/git-core/
# CA certificates for HTTPS registry and forge API calls
COPY --from=gitdeps /etc/ssl/certs/             /etc/ssl/certs/
COPY --from=gitdeps /usr/share/ca-certificates/ /usr/share/ca-certificates/

COPY --from=builder --chown=nonroot:nonroot /semrel /semrel

USER nonroot

ENTRYPOINT ["/semrel"]

# ── alpine variant ─────────────────────────────────────────────────────────────
# Pinned to Alpine 3.22 (latest stable minor) to prevent untracked base upgrades.
# apk upgrade ensures all packages are at their latest patched versions,
# eliminating the outdated-base CVEs reported by Trivy / Harbor SBOM scans.
# Includes openssh-client for deployments that push over SSH.
FROM alpine:3.22 AS alpine

ARG VERSION=dev
ARG BUILD_DATE
ARG VCS_REF

LABEL org.opencontainers.image.title="semrel (alpine)" \
      org.opencontainers.image.description="Automated semantic releases for Go projects" \
      org.opencontainers.image.url="https://semrel.io" \
      org.opencontainers.image.source="https://github.com/SemRels/semrel" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.revision="${VCS_REF}" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.vendor="SemRels"

# Upgrade all base packages first to pick up Alpine security patches,
# then add only the packages semrel needs at runtime.
RUN apk upgrade --no-cache && \
    apk add --no-cache \
        git \
        ca-certificates \
        openssh-client \
    && adduser -D -u 10001 -s /sbin/nologin semrel \
    && mkdir -p /workspace \
    && chown semrel:semrel /workspace

COPY --from=builder --chown=semrel:semrel /semrel /usr/local/bin/semrel

USER semrel
WORKDIR /workspace

ENTRYPOINT ["semrel"]

# ── GitHub Actions container variant ──────────────────────────────────────────
# GitHub bind-mounts its workspace with the runner's uid and requires Docker
# container actions to run as root. Keep this isolated from the normal non-root
# Alpine image: users must explicitly select the action target/tag.
FROM alpine AS action

LABEL org.opencontainers.image.description="Automated semantic releases for GitHub Actions container steps"

USER root
WORKDIR /github/workspace

# Keep an explicitly non-root default when no build target is supplied.
FROM release AS default
