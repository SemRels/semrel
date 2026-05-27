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
    -ldflags="-s -w -X github.com/GoSemantics/semrel/internal/cli.version=${VERSION}" \
    -o /semrel \
    ./cmd/semrel

# ── distroless release image ───────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS release

COPY --from=builder /semrel /semrel

# git is not available in distroless — users must mount the workspace and ensure
# git is in PATH via a wrapping image or use the alpine variant.
ENTRYPOINT ["/semrel"]

# ── alpine variant (includes git + ca-certificates) ───────────────────────────
FROM alpine:3 AS alpine

RUN apk add --no-cache git ca-certificates openssh-client

COPY --from=builder /semrel /usr/local/bin/semrel

WORKDIR /workspace
ENTRYPOINT ["semrel"]
