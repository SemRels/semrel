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

# Verify binary is statically linked and has no SUID bits
RUN chmod 0555 /semrel

# ── distroless release image ───────────────────────────────────────────────────
# Runs as nonroot (uid 65532) — no shell, no package manager, minimal attack surface.
# Use the alpine variant for CI pipelines that need git.
FROM gcr.io/distroless/static-debian12:nonroot AS release

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

COPY --from=builder --chown=nonroot:nonroot /semrel /semrel

USER nonroot

ENTRYPOINT ["/semrel"]

# ── alpine variant (includes git + ca-certificates) ───────────────────────────
# Recommended for CI use — includes git, ca-certificates, and openssh.
# Runs as dedicated non-root user (uid 10001).
FROM alpine:3 AS alpine

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

RUN apk add --no-cache git ca-certificates openssh-client \
    && adduser -D -u 10001 -s /sbin/nologin semrel \
    && mkdir -p /workspace \
    && chown semrel:semrel /workspace

COPY --from=builder --chown=semrel:semrel /semrel /usr/local/bin/semrel

USER semrel
WORKDIR /workspace

ENTRYPOINT ["semrel"]
