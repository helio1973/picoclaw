# ============================================================
# Stage 1: Build the picoclaw binary
# ============================================================
FROM golang:1.26.0-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /src

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .
RUN make build

# ============================================================
# Stage 2: Minimal runtime image
# ============================================================
FROM alpine:3.23

RUN apk add --no-cache ca-certificates tzdata curl

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q --spider http://localhost:18790/health || exit 1

# Copy binary and entrypoint
COPY --from=builder /src/build/picoclaw /usr/local/bin/picoclaw
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN chmod +x /usr/local/bin/entrypoint.sh

# Create non-root user and group
RUN addgroup -g 1000 picoclaw && \
    adduser -D -u 1000 -G picoclaw picoclaw

# Switch to non-root user
USER picoclaw

# Run onboard to create initial directories, config, and skills
RUN /usr/local/bin/picoclaw onboard

# Preserve built-in skills separately so they survive volume mounts.
# The entrypoint script syncs these into the workspace on startup.
RUN cp -r /home/picoclaw/.picoclaw/workspace/skills /home/picoclaw/.picoclaw/_builtin_skills

ENTRYPOINT ["entrypoint.sh"]
CMD ["gateway"]
