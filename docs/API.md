# API Reference

Base URL is the Ingress host in-cluster, or `http://localhost:8080` locally.
All responses are JSON unless noted. Errors use `{"error": "message"}`.

---

## `GET /health`

Liveness/readiness probe target.

```json
200 OK
{ "status": "ok", "time": "2026-07-11T10:00:00Z" }
```

---

## `GET /metrics`

Prometheus exposition format (text). Includes Go runtime + process collectors
and all `memorydrive_*` application metrics. See `docs/OBSERVABILITY.md`.

---

## `POST /upload`

`multipart/form-data`. Two modes:

**Note** — send a `content` field (and optional `name`):

```bash
curl -X POST $BASE/upload -F 'name=Ideas' -F 'content=learn kubernetes'
```

**File / image** — send a `file` field (and optional `name`). The kind is
inferred: `image/*` content types become `image`, everything else `text`.

```bash
curl -X POST $BASE/upload -F 'file=@photo.png'
```

Response `201 Created`:

```json
{
  "id": "0f1e...",
  "name": "Ideas",
  "kind": "note",
  "contentType": "text/plain; charset=utf-8",
  "size": 15,
  "content": "learn kubernetes",
  "createdAt": "2026-07-11T10:00:00Z"
}
```

Errors: `400` (no file and no content), `413` (file exceeds `MAX_UPLOAD_MB`).

---

## `GET /files`

List items, newest first. Optional `?search=` filters case-insensitively by
name **or** text content.

```json
200 OK
{ "count": 2, "files": [ { "id": "...", "name": "...", "kind": "text", ... } ] }
```

---

## `GET /files/{id}`

Returns the item's **raw content**, not JSON:

- `note` → `text/plain`
- `text` → the stored text file
- `image` → the image bytes with its original content type

Useful directly in the browser or as an `<img src>`. `404` if not found.

---

## `DELETE /files/{id}`

Deletes metadata and the blob (if any).

```json
200 OK
{ "deleted": "0f1e..." }
```

---

## `POST /simulate/memory`

`application/json`. Deliberately allocate memory or resize the in-memory cache.
Never crashes the process.

| Field | Type | Meaning |
|---|---|---|
| `megabytes` | int | MB to allocate on top of current usage. |
| `holdSeconds` | int | Auto-release after N seconds; `0` = hold until restart. |
| `cacheSizeMB` | int | Optional: set the cache to this size (grows/shrinks). |

```bash
curl -X POST $BASE/simulate/memory -H 'Content-Type: application/json' \
  -d '{"megabytes":200,"holdSeconds":300}'
```

```json
200 OK
{ "allocatedMB": 200, "cacheMB": 64 }
```

---

## `POST /simulate/load`

`application/json`. Drive CPU utilisation for a bounded duration.

| Field | Type | Default | Meaning |
|---|---|---|---|
| `durationSeconds` | int | 5 | How long to run. |
| `workers` | int | #CPUs | Number of busy goroutines. |
| `async` | bool | false | Return immediately instead of blocking. |

```bash
curl -X POST $BASE/simulate/load -H 'Content-Type: application/json' \
  -d '{"durationSeconds":30,"workers":4,"async":true}'
```

```json
202 Accepted
{ "status": "started", "workers": 4, "durationSeconds": 30 }
```

---

## `GET /stats`

Human-friendly snapshot of resource usage (mirrors much of `/metrics`).

```json
200 OK
{
  "uptimeSeconds": 120,
  "files": 3,
  "totalFileBytes": 20480,
  "cacheMB": 64,
  "allocatedMB": 200,
  "workerCount": 2,
  "goroutines": 14,
  "heapAllocBytes": 12345678,
  "heapInuseBytes": 15000000,
  "sysBytes": 90000000,
  "numGC": 4,
  "config": { "memoryCacheEnabled": true, "backgroundWorkers": true, "maxUploadMB": 10 }
}
```
