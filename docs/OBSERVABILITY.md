# Observability

The backend exposes Prometheus metrics on `GET /metrics`. Two collectors ship
by default with the Prometheus Go client:

- **Go runtime** (`go_*`): goroutines, heap, GC pauses, etc.
- **Process** (`process_*`): `process_cpu_seconds_total`,
  `process_resident_memory_bytes`, open FDs, etc.

On top of those, Memory Drive defines these application metrics:

| Metric | Type | Labels | Meaning |
|---|---|---|---|
| `memorydrive_http_requests_total` | counter | `method`, `path`, `status` | HTTP request count. |
| `memorydrive_http_request_duration_seconds` | histogram | `method`, `path` | HTTP latency. |
| `memorydrive_uploads_total` | counter | `kind` | Uploads by kind (note/text/image). |
| `memorydrive_upload_size_bytes` | histogram | – | Upload size distribution. |
| `memorydrive_managed_memory_bytes` | gauge | – | Memory deliberately allocated by the workload generator. |
| `memorydrive_cache_size_bytes` | gauge | – | Current in-memory cache size. |
| `memorydrive_heap_inuse_bytes` | gauge | – | Runtime heap in use (sampled). |
| `memorydrive_cpu_usage_percent` | gauge | – | Approximate process CPU %. |
| `memorydrive_worker_queue_size` | gauge | – | Jobs waiting in the worker queue. |
| `memorydrive_active_workers` | gauge | – | Active background workers. |
| `memorydrive_background_job_duration_seconds` | histogram | – | Background job execution time. |
| `memorydrive_background_jobs_total` | counter | – | Background jobs completed. |

## Verifying metrics

```bash
kubectl -n memory-drive port-forward svc/memory-drive-backend 8080:8080
curl -s localhost:8080/metrics | grep -E 'memorydrive_|process_|go_goroutines'
```

## Wiring into kube-prometheus-stack

Install the stack (Helm — outside this repo's scope), then either:

1. **ServiceMonitor** — apply `k8s/11-servicemonitor.yaml`. Make sure its
   `release:` label matches your Helm release name so the operator selects it.
2. **Annotation scraping** — the backend Service/Pod carry
   `prometheus.io/scrape`, `prometheus.io/port` and `prometheus.io/path`
   annotations for Prometheus configs that use annotation-based discovery.

## Useful Grafana / PromQL queries

```promql
# Request rate by path
sum(rate(memorydrive_http_requests_total[1m])) by (path)

# p95 latency
histogram_quantile(0.95, sum(rate(memorydrive_http_request_duration_seconds_bucket[5m])) by (le, path))

# Deliberately-held memory (MB)
memorydrive_managed_memory_bytes / 1024 / 1024

# Container memory vs. its limit (from cAdvisor/kube-state-metrics)
container_memory_working_set_bytes{namespace="memory-drive"}
  / on() group_left kube_pod_container_resource_limits{namespace="memory-drive",resource="memory"}

# Background job p90 duration
histogram_quantile(0.90, sum(rate(memorydrive_background_job_duration_seconds_bucket[5m])) by (le))

# Worker queue depth
memorydrive_worker_queue_size
```

## A quick observability demo

```bash
# 1. Watch pods and the HPA in one terminal
kubectl -n memory-drive get hpa,pods -w

# 2. In another, drive CPU load for 60s
curl -X POST http://<host>/simulate/load -H 'Content-Type: application/json' \
  -d '{"durationSeconds":60,"workers":4,"async":true}'

# 3. Allocate 150 MB and watch memorydrive_managed_memory_bytes climb in Grafana
curl -X POST http://<host>/simulate/memory -H 'Content-Type: application/json' \
  -d '{"megabytes":150,"holdSeconds":600}'
```
