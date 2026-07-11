# Deployment notes (EKS on t2.micro)

## Cluster prerequisites

- **Ingress controller** — manifests assume the NGINX Ingress Controller
  (`ingressClassName: nginx`). For an AWS ALB, change the class to `alb` and add
  the `alb.ingress.kubernetes.io/*` annotations in `k8s/09-ingress.yaml`.
- **metrics-server** — required for the HorizontalPodAutoscaler.
- **A default StorageClass** — the PVC omits `storageClassName` to use the
  cluster default (EBS `gp2`/`gp3` on EKS via the EBS CSI driver).
- **(Optional) kube-prometheus-stack** — for Prometheus + Grafana.

## Sizing for two t2.micro nodes

A `t2.micro` has 1 vCPU and 1 GiB RAM; after the kubelet/system reservation,
roughly 600–700 MiB is allocatable per node. The default requests/limits fit
comfortably:

| Workload | CPU req/limit | Mem req/limit | Replicas |
|---|---|---|---|
| backend | 100m / 500m | 128Mi / 256Mi | 1 (HPA 1–3) |
| frontend | 25m / 100m | 32Mi / 64Mi | 1 |

Leave headroom for the metrics/monitoring agents. If you install
kube-prometheus-stack on the same two nodes, consider scaling its components
down or using a third node.

## The storage + autoscaling caveat (read this)

The backend uses **SQLite** (a single-file, single-host database) on a
**ReadWriteOnce** volume. Consequences:

- The volume attaches to exactly one node. Kubernetes therefore schedules every
  backend replica onto **that same node**, where they share the SQLite file
  safely through WAL locking (`busy_timeout` is set to 5s).
- This means the backend HPA (`minReplicas: 1`, `maxReplicas: 3`) scales
  **within a single node**. It's perfect for demonstrating CPU-driven scaling,
  but it is **not** a horizontally-scalable database design — that's expected
  for a SQLite-based learning app.
- Rollouts use the `Recreate` strategy so two Pods never contend for the volume
  during an update.

If you want cross-node scaling later, swap SQLite for a networked database
(Postgres/RDS) and switch the volume to `ReadWriteMany` or remove it.

## Images

Update the `image:` fields in `k8s/05-backend-deployment.yaml` and
`k8s/07-frontend-deployment.yaml` to point at your registry (e.g. ECR), then
`kubectl rollout restart` the deployments after pushing new tags.

## Accessing the app

- With NGINX Ingress + the `memory-drive.local` host: map the host to the
  ingress controller's external IP (via DNS or `/etc/hosts`), then browse to
  `http://memory-drive.local`.
- Or port-forward for a quick look:
  ```bash
  kubectl -n memory-drive port-forward svc/memory-drive-frontend 8080:80
  # then also forward the backend, or use the Ingress, so API calls resolve
  ```
  Behind the Ingress the frontend and backend share one origin, so the SPA's
  relative API calls (`/files`, `/upload`, …) just work.

## Common checks

```bash
kubectl -n memory-drive get all
kubectl -n memory-drive describe pvc memory-drive-data
kubectl -n memory-drive logs deploy/memory-drive-backend
kubectl -n memory-drive get hpa memory-drive-backend -w
```
