# 🧠 Memory Drive

A lightweight, cloud-native **"Memory Drive"** app built for practising Kubernetes
concepts on Amazon EKS: observability, autoscaling, persistent storage, probes,
and resource utilisation.

Users can upload **notes**, **text files** and **small images**, then **view**,
**search** and **delete** them. Files are stored on a mounted volume and metadata
lives in SQLite. The backend exposes **Prometheus metrics** and ships a built-in
**workload generator** so you can deliberately raise memory and CPU usage and
watch it in Grafana — without ever crashing the app.

> Scope note: this repo intentionally contains **only** the application, its
> Dockerfiles, and plain Kubernetes manifests. There is **no** CI/CD, GitHub
> Actions, ArgoCD, Helm, Kustomize, Terraform, or AWS infrastructure — those are
> left for you to build as a learning exercise.

---

## Architecture

```
                    ┌───────────────────────── Kubernetes (EKS) ─────────────────────────┐
                    │                                                                     │
   Browser ──▶ Ingress ──┬─▶  Service: frontend (nginx SPA, :80)  ──▶  React/Vite bundle  │
                         │                                                                 │
                         └─▶  Service: backend  (Go/Gin, :8080)   ──▶  Backend Pod         │
                                                                        ├─ SQLite (metadata)│
                                                                        ├─ /data volume     │
                                                                        │   (PVC, uploads)  │
                                                                        ├─ /metrics (Prom)  │
                                                                        └─ workload gen     │
                    │                                                                     │
                    │   Prometheus (kube-prometheus-stack) scrapes backend /metrics ──▶ Grafana │
                    └─────────────────────────────────────────────────────────────────────┘
```

- **Frontend** — React + Vite + TypeScript, served as static files by an
  unprivileged nginx container.
- **Backend** — Go + Gin. Handles uploads, search, metrics and workload
  generation. Single-writer SQLite via the pure-Go `modernc.org/sqlite` driver
  (no CGO → tiny static binary).
- **Storage** — a PersistentVolumeClaim mounted at `/data` holds both the SQLite
  file and the uploaded blobs.
- **Observability** — `/metrics` exposes Prometheus counters, histograms and
  gauges, including custom workload metrics.

---

## Folder structure

```
K8s-Project/
├── backend/                    # Go + Gin API
│   ├── main.go                 # entrypoint, routing, graceful shutdown
│   ├── go.mod
│   ├── .env.example
│   └── internal/
│       ├── config/             # env-var configuration
│       ├── models/             # File model
│       ├── db/                 # SQLite store (metadata)
│       ├── storage/            # blob storage on the volume
│       ├── metrics/            # Prometheus metrics + runtime sampler
│       ├── simulate/           # memory cache + memory/CPU load generation
│       ├── workers/            # background worker pool + periodic jobs
│       └── handlers/           # HTTP handlers + Prometheus middleware
├── frontend/                   # React + Vite + TS SPA
│   ├── index.html
│   ├── package.json
│   ├── vite.config.ts
│   ├── nginx.conf              # production static-serving config
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── styles.css
│       ├── api/client.ts       # typed API client
│       └── components/         # UploadPanel, FileList, WorkloadPanel
├── k8s/                        # plain Kubernetes manifests (apply in order)
│   ├── 00-namespace.yaml
│   ├── 01-serviceaccount.yaml
│   ├── 02-configmap.yaml
│   ├── 03-secret.yaml          # TEMPLATE — replace before real use
│   ├── 04-pvc.yaml
│   ├── 05-backend-deployment.yaml
│   ├── 06-backend-service.yaml
│   ├── 07-frontend-deployment.yaml
│   ├── 08-frontend-service.yaml
│   ├── 09-ingress.yaml
│   ├── 10-hpa.yaml
│   └── 11-servicemonitor.yaml  # OPTIONAL (needs Prometheus Operator)
├── docs/
│   ├── API.md
│   ├── DEPLOYMENT.md
│   └── OBSERVABILITY.md
├── Dockerfile.backend
├── Dockerfile.frontend
├── .dockerignore
├── .gitignore
└── README.md
```

---

## Local development

### Backend

```bash
cd backend
cp .env.example .env            # optional: tweak values
set -a; source .env; set +a     # load env vars into the shell
mkdir -p ./data
go mod tidy                     # resolves deps and writes go.sum
go run .
```

The API is now on `http://localhost:8080` (try `curl localhost:8080/health`).

### Frontend

```bash
cd frontend
npm install
npm run dev                     # http://localhost:5173
```

The Vite dev server proxies `/upload`, `/files`, `/simulate`, `/stats` and
`/health` to `http://localhost:8080`, so run the backend alongside it.

---

## Docker build commands

Build both images from the repo root (the Dockerfiles expect the repo root as
the build context):

```bash
# Backend
docker build -f Dockerfile.backend  -t memory-drive-backend:latest  .

# Frontend
docker build -f Dockerfile.frontend -t memory-drive-frontend:latest .
```

Run them locally to smoke-test:

```bash
docker run --rm -p 8080:8080 -v "$PWD/data:/data" memory-drive-backend:latest
docker run --rm -p 8081:8080 memory-drive-frontend:latest   # http://localhost:8081
```

Push to your registry (e.g. ECR) and update the `image:` fields in
`k8s/05-backend-deployment.yaml` and `k8s/07-frontend-deployment.yaml`.

---

## Kubernetes deployment order

Apply the manifests in numeric order. The `k8s/` folder is prefixed so a single
`kubectl apply -f k8s/` also works, but the explicit order is:

```bash
kubectl apply -f k8s/00-namespace.yaml
kubectl apply -f k8s/01-serviceaccount.yaml
kubectl apply -f k8s/02-configmap.yaml
kubectl apply -f k8s/03-secret.yaml           # replace with real values first
kubectl apply -f k8s/04-pvc.yaml
kubectl apply -f k8s/05-backend-deployment.yaml
kubectl apply -f k8s/06-backend-service.yaml
kubectl apply -f k8s/07-frontend-deployment.yaml
kubectl apply -f k8s/08-frontend-service.yaml
kubectl apply -f k8s/09-ingress.yaml
kubectl apply -f k8s/10-hpa.yaml              # needs metrics-server
# Optional, only after kube-prometheus-stack is installed:
kubectl apply -f k8s/11-servicemonitor.yaml
```

Prerequisites in the cluster: an **Ingress controller** (manifests assume NGINX),
**metrics-server** (for the HPA), and optionally **kube-prometheus-stack** (for
metrics + Grafana). See `docs/DEPLOYMENT.md` for details and the scaling caveat.

---

## Environment variables

All backend behaviour is controlled by environment variables (set via
`k8s/02-configmap.yaml` in-cluster, or `.env` locally).

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port. |
| `GIN_MODE` | `release` | Gin mode (`release` or `debug`). |
| `DB_PATH` | `/data/memorydrive.db` | SQLite file path (on the volume). |
| `UPLOAD_DIR` | `/data/uploads` | Directory for uploaded blobs. |
| `MAX_UPLOAD_MB` | `10` | Max size of a single upload (MB). |
| `ENABLE_MEMORY_CACHE` | `false` | Pre-fill an in-memory cache at startup. |
| `CACHE_SIZE_MB` | `0` | Size of that cache in MB. |
| `BASELINE_MEMORY_MB` | `0` | Memory allocated at startup and held forever. |
| `ENABLE_BACKGROUND_WORKERS` | `false` | Start the background worker pool. |
| `WORKER_COUNT` | `2` | Number of background workers. |
| `WORKER_INTERVAL_SECONDS` | `10` | How often the periodic job fires. |

> ⚠️ Keep `CACHE_SIZE_MB + BASELINE_MEMORY_MB` comfortably below the pod's memory
> **limit** (256Mi by default) so the container is never OOM-killed. To push
> usage higher, raise the limit in `k8s/05-backend-deployment.yaml` first.

---

## API documentation

Full details and examples are in [`docs/API.md`](docs/API.md). Summary:

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/health` | Liveness/readiness check. |
| `GET` | `/metrics` | Prometheus metrics. |
| `POST` | `/upload` | Upload a note (`content`) or a file (`file`). |
| `GET` | `/files` | List items; `?search=` filters by name/content. |
| `GET` | `/files/{id}` | Return an item's raw content. |
| `DELETE` | `/files/{id}` | Delete an item and its blob. |
| `POST` | `/simulate/memory` | Allocate memory / resize the cache. |
| `POST` | `/simulate/load` | Drive CPU load for N seconds. |
| `GET` | `/stats` | JSON snapshot of resource usage. |

---

## How to trigger memory usage

Three ways, from least to most dynamic:

1. **At startup (config):** set `ENABLE_MEMORY_CACHE=true`, `CACHE_SIZE_MB=200`
   and/or `BASELINE_MEMORY_MB=100` in the ConfigMap and roll the deployment.
2. **On demand (API):** allocate 200 MB held for 5 minutes:
   ```bash
   curl -X POST http://<host>/simulate/memory \
     -H 'Content-Type: application/json' \
     -d '{"megabytes":200,"holdSeconds":300}'
   ```
   Or resize the cache live: `{"cacheSizeMB":150}`.
3. **CPU load burst:** run 4 workers for 30 seconds (drives the HPA):
   ```bash
   curl -X POST http://<host>/simulate/load \
     -H 'Content-Type: application/json' \
     -d '{"durationSeconds":30,"workers":4}'
   ```

The UI's **Resource playground** panel exposes the same controls.

---

## How to verify Prometheus metrics

```bash
# Port-forward the backend and scrape it directly.
kubectl -n memory-drive port-forward svc/memory-drive-backend 8080:8080
curl -s localhost:8080/metrics | grep memorydrive_
```

You should see counters/gauges such as `memorydrive_http_requests_total`,
`memorydrive_uploads_total`, `memorydrive_cache_size_bytes`,
`memorydrive_managed_memory_bytes`, `memorydrive_worker_queue_size` and
`memorydrive_background_job_duration_seconds`. Full list and example Grafana
queries are in [`docs/OBSERVABILITY.md`](docs/OBSERVABILITY.md).

---

## How to test the application

Quick end-to-end smoke test (adjust the base URL):

```bash
BASE=http://localhost:8080

# Health
curl -s $BASE/health

# Create a note
curl -s -X POST $BASE/upload -F 'name=Shopping' -F 'content=milk, eggs, bread'

# Upload a text file
echo "hello kubernetes" > /tmp/hello.txt
curl -s -X POST $BASE/upload -F 'file=@/tmp/hello.txt'

# List and search
curl -s $BASE/files
curl -s "$BASE/files?search=kubernetes"

# Stats
curl -s $BASE/stats

# Delete (take an id from the list output)
curl -s -X DELETE $BASE/files/<id>
```

Type-check the frontend and vet the backend:

```bash
cd frontend && npm run lint      # tsc --noEmit
cd backend  && go vet ./...
```
