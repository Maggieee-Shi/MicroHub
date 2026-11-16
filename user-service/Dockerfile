# syntax=docker/dockerfile:1

# --- build stage -----------------------------------------------------------
FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
# Default image is the dependency-free, in-memory build. Pass
# --build-arg BUILD_TAGS=postgres to produce the Postgres-backed binary
# instead (requires go.sum to include github.com/lib/pq).
ARG BUILD_TAGS=""
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -tags "${BUILD_TAGS}" -o /out/user-service .

# --- runtime stage -----------------------------------------------------------
# "scratch" is Docker's empty base image (no shell, no package manager, no
# libc) - nothing to pull, nothing to patch, smallest possible attack
# surface. Safe here because the binary above is a static, CGO-disabled
# Go binary with no shared-library dependencies.
FROM scratch
COPY --from=builder /out/user-service /user-service
EXPOSE 8081
ENTRYPOINT ["/user-service"]
