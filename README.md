# MicroHub

A small Tinder-style matching system split into three Go microservices,
built to be a real, runnable demonstration of Docker and Kubernetes
fundamentals — containerization, inter-service networking, persistence,
and health-checked deployments — rather than a toy "hello world" split
across files.

## Architecture

```
                 ┌────────────────────┐
  client ──────▶ │  matching-service   │  (public, :8080)
                 │  swipes + matches   │
                 └──────────┬──────────┘
                     REST    │   REST
             ┌───────────────┤
             ▼                              ▼
  ┌─────────────────────┐      ┌───────────────────────┐
  │    user-service       │      │  notification-service │
  │  profiles, :8081      │      │  match feed, :8082    │
  │  (Postgres-backed)    │      │  (in-memory)           │
  └──────────┬────────────┘      └────────────────────────┘
             │
             ▼
       ┌───────────┐
       │ Postgres  │
       └───────────┘
```

- **user-service** owns user profiles. It's the only service with a real
  persistence layer and is the one you'd point to if asked "where's the
  database in this project." Behind a `Store` interface so it can run
  against an in-memory store (default, zero dependencies) or a
  Postgres-backed store (`-tags postgres`, used in the container image and
  the Kubernetes manifests).
- **matching-service** is the public-facing API. It records swipes, detects
  mutual likes (a match), and calls the other two services over plain HTTP
  using their Kubernetes Service names (`http://user-service:8081`,
  `http://notification-service:8082`) — no hardcoded IPs, no client-side
  service-discovery library, just cluster DNS.
- **notification-service** keeps a per-user feed of "you matched with X"
  events. Deliberately simple and in-memory.

## Running it

**Locally, no Docker:**

```bash
(cd user-service && go run .)             # :8081, in-memory store
(cd notification-service && go run .)     # :8082
USER_SERVICE_URL=http://localhost:8081 \
NOTIFICATION_SERVICE_URL=http://localhost:8082 \
  (cd matching-service && go run .)       # :8080

./scripts/smoke_test.sh
```

**With Docker Compose** (real Postgres, all three services containerized):

```bash
docker compose up --build
./scripts/smoke_test.sh
```

**On Kubernetes** (kind, minikube, GKE/EKS/AKS — any cluster):

```bash
# Build and push images first - see "Building the images" below.
kubectl apply -f k8s/
kubectl -n microhub get pods -w
kubectl -n microhub port-forward svc/matching-service 8080:80 &
./scripts/smoke_test.sh
```

### Building the images

> **One-time setup:** this repo's `user-service/go.sum` doesn't include
> `github.com/lib/pq` yet, because generating it requires network access to
> `proxy.golang.org`, which the sandbox this was built in couldn't reach.
> Before building the Postgres-tagged image (or running
> `docker compose up`, which builds it for you), run this once:
> `cd user-service && go mod tidy`. It will pass on any normal machine or
> CI runner with internet access.

```bash
docker build -t <registry>/microhub-user-service:latest \
  --build-arg BUILD_TAGS=postgres ./user-service
docker build -t <registry>/microhub-matching-service:latest ./matching-service
docker build -t <registry>/microhub-notification-service:latest ./notification-service
docker push <registry>/microhub-user-service:latest
# ...push the other two, then update the `image:` fields in k8s/*.yaml
```

Each Dockerfile is a two-stage build: `golang:1.24-alpine` compiles a
static, `CGO_ENABLED=0` binary, and the final image is `FROM scratch` — no
shell, no package manager, nothing to patch. Final images run ~13-15MB.

## What's actually been verified, and how

Everything below was run for real, not just written and assumed correct:

- All three services build cleanly (`go build`, `go vet`) and pass unit
  tests (`go test ./...` in each service directory) — swipe/match logic,
  the in-memory user store's CRUD + not-found behavior, and the
  notification feed are covered.
- The full flow (create two users → one-sided swipe → reciprocating swipe →
  match fires → both users get notified) was run end to end as plain
  `go run` processes talking over real HTTP — see `scripts/smoke_test.sh`.
- The same flow was re-run through **real Docker containers** on a Docker
  bridge network, resolving each other by container name exactly the way
  Kubernetes Service DNS works, proving the containerization and
  inter-service networking actually function, not just the Go code.

  Kubernetes would schedule and load-balance across replicas.
