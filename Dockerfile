# ─────────────────────────────────────────────────────────────────────────────
# Stage 1: Build OttoClaw (Brain)
# Build context: repo root (Siam-Synapse/)
# docker build -f ottoclaw-worker/Dockerfile -t siam-synapse-ottoclaw-worker:latest .
# ─────────────────────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS ottoclaw-builder
WORKDIR /build
COPY ottoclaw-worker/ottoclaw/ .
# go:embed requires workspace/ to exist inside the onboard package directory
RUN cp -r workspace cmd/ottoclaw/internal/onboard/workspace
RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o ottoclaw ./cmd/ottoclaw



# ─────────────────────────────────────────────────────────────────────────────
# Stage 2: Build siam-worker (Arm)
# ─────────────────────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS siam-builder
WORKDIR /app/worker
# Build using the local standalone siam-arm directory
COPY ottoclaw-worker/siam-arm/go.mod ottoclaw-worker/siam-arm/go.sum ./
COPY ottoclaw-worker/siam-arm/proto/ ./proto/
RUN rm -f ./proto/control*

COPY ottoclaw-worker/siam-arm/*.go ./
# Fix: copy the ottoclaw local dependency to satisfy go.mod "replace github.com/sipeed/ottoclaw => ../ottoclaw"
COPY ottoclaw-worker/ottoclaw/ ../ottoclaw/
RUN go mod download && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/siam-worker .


# ─────────────────────────────────────────────────────────────────────────────
# Stage 3: Final Image (Ultra-Lightweight)
# ─────────────────────────────────────────────────────────────────────────────
FROM python:3.11-slim
RUN apt-get update && apt-get install -y ca-certificates curl bash && \
    pip install --no-cache-dir playwright && \
    playwright install --with-deps chromium && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Copy binaries
COPY --from=ottoclaw-builder /build/ottoclaw .
COPY --from=siam-builder /app/siam-worker .

# Copy SIAM skills
COPY ottoclaw-worker/skills/ /app/skills/
RUN chmod +x /app/skills/siam/register.sh

# Copy entrypoint
COPY ottoclaw-worker/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

# OttoClaw workspace — pre-load SIAM skill
RUN mkdir -p /root/.ottoclaw/workspace/skills
COPY ottoclaw-worker/skills/ /root/.ottoclaw/workspace/skills/

# Orchestrator workspace — SOUL.md, AGENTS.md etc. loaded in orchestrator mode
RUN mkdir -p /app/workspace
COPY ottoclaw-worker/workspace/ /app/workspace/

ENV OTTOCLAW_HOME=/root/.ottoclaw

ENTRYPOINT ["/app/entrypoint.sh"]
