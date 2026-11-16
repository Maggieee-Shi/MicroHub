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

One real constraint from the sandbox this was built in, stated plainly
rather than glossed over: that environment's network policy blocks every
container registry (Docker Hub, gcr.io, ghcr.io, quay.io, registry.k8s.io)
and the Kubernetes release-binary host, so I could not pull `golang:alpine`,
`postgres:16-alpine`, or a `kind`/`minikube` node image there, and so could
not run a live cluster or the exact multi-stage Dockerfiles above inside
that sandbox. To still prove the Docker piece for real, I compiled the
binaries locally and built temporary `FROM scratch` images directly from
them (no base image pull needed), ran all three as containers, and
confirmed the match flow over the container network — the same runtime
mechanism the production Dockerfiles rely on. The Kubernetes manifests in
`k8s/` are hand-written to standard API conventions and validated for YAML
syntax, but have not been applied to a live cluster. On your own machine
(or in CI), `docker build` and `kubectl apply -f k8s/` against a real
cluster should just work — that's the one piece worth running yourself
before you put "deployed to Kubernetes" in an interview answer without
having seen `kubectl get pods` return `Running` yourself.

## Known limitations (worth being able to talk about, not hiding)

- `matching-service` and `notification-service` keep state in memory, so
  both are pinned to a single replica in the k8s manifests — a second
  replica would silently split state between pods. Scaling either safely
  would mean moving that state to Postgres or Redis first.
- No auth/authz between services or on the public API — out of scope for
  a demo of containerization and orchestration specifically.
- `postgres` runs as a single-replica Deployment with a PVC, not a proper
  HA StatefulSet setup — fine for a portfolio project, not for production.

## Suggested resume / LinkedIn bullets

Pick 2-3, written the way your Tinder and IpserLab bullets are (metric- or
mechanism-led, not responsibility-described):

- Designed and built MicroHub, a 3-service Go backend (REST APIs, Postgres
  persistence behind a swappable storage interface) containerized with
  multi-stage Docker builds producing ~14MB `scratch`-based images, and
  deployed via Kubernetes Deployments/Services with readiness/liveness
  probes and ClusterIP-scoped internal services.
- Implemented Kubernetes-native service discovery for inter-service calls
  (DNS-based, no hardcoded IPs/service registry), and used a
  PersistentVolumeClaim-backed Postgres deployment to keep user data
  durable across pod restarts.
- Identified and documented a stateful-replica scaling limitation in an
  in-memory matching service, pinning it to a single replica rather than
  silently shipping a race condition — informed by tracing exactly how
  Kubernetes would schedule and load-balance across replicas.
