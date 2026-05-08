# typesense-snapshot-controller

Kubernetes controller that automates [Typesense](https://typesense.org) snapshots. Triggers on a cron schedule, pod eviction events, and Karpenter node disruption signals. Deduplicates via a shared cooldown window so scheduled and event-driven snapshots never collide.

## How it works

Two modes share one binary:

**`serve`** — long-running controller (Deployment). Watches for:
- Pod `DeletionTimestamp` set or hard-delete events on labelled Typesense pods → immediate snapshot
- Karpenter `karpenter.sh/disruption` taint added to a node → immediate snapshot

**`run`** — one-shot (CronJob). Takes a single snapshot and exits.

Both check a shared ConfigMap before acting. If a snapshot was taken within the cooldown window (default 3 minutes), the trigger is silently skipped. This prevents a scheduled run and a simultaneous eviction from both firing.

Leader discovery probes each Typesense pod's `/readyz` endpoint and forwards the snapshot request to the first ready pod. Node drains are never blocked — reconcilers return immediately and fire snapshots in background goroutines.

## Prerequisites

- Kubernetes 1.31+
- Typesense cluster deployed via the `ts.opentelekomcloud.com/v1alpha1` CRD (watched but not owned)
- Typesense API key stored in a Kubernetes Secret
- Karpenter v1 (optional — node disruption trigger only)
- ArgoCD 2.6+ or Helm 3

---

## Configuration

All settings are controlled via Helm values. Key fields:

| Value | Default | Description |
|---|---|---|
| `typesense.podLabelSelector` | `app=typesense-global-2-sts` | Label selector to identify Typesense pods |
| `typesense.port` | `8108` | Typesense HTTP port |
| `typesense.secretName` | `typesense-cluster-bootstrap-key` | Secret containing the API key |
| `typesense.secretKey` | `typesense-api-key` | Key within the secret |
| `typesense.snapshotPath` | `/var/lib/typesense/snapshots` | Path passed to the snapshot API |
| `cooldown.duration` | `3m` | Minimum time between snapshots |
| `cooldown.configMapName` | `typesense-snapshot-cooldown` | ConfigMap used to track last snapshot time |
| `controller.replicaCount` | `2` | Controller replicas (leader-elected, one active at a time) |
| `controller.leaderElection` | `true` | Enable Kubernetes Lease-based leader election |
| `cronjob.schedule` | `*/30 * * * *` | Cron expression for scheduled snapshots |
| `cronjob.concurrencyPolicy` | `Forbid` | Prevents overlapping CronJob runs |
| `cronjob.backoffLimit` | `2` | Job retries on failure |
| `networkPolicy.enabled` | `true` | Restrict controller egress to Typesense + kube-apiserver + DNS |
| `networkPolicy.typesensePodLabels` | `app: typesense-global-2-sts` | Labels matching Typesense pods for NetworkPolicy egress |

---

## Deploy with ArgoCD

### 1. Create the API key Secret

The controller reads the Typesense API key from a Secret in the same namespace. Create it before deploying:

```bash
kubectl create namespace typesense-controller

kubectl create secret generic typesense-cluster-bootstrap-key \
  --namespace typesense-controller \
  --from-literal=typesense-api-key=<your-api-key>
```

> The secret name and key must match `typesense.secretName` and `typesense.secretKey` in values.

### 2. ArgoCD Application

Create an ArgoCD `Application` pointing at the Helm chart in this repo:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: typesense-snapshot-controller
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/mfeldheim/typesense-snapshot-controller
    targetRevision: main          # or pin to a tag: v1.0.0
    path: deploy/helm/typesense-snapshot-controller
    helm:
      values: |
        typesense:
          podLabelSelector: "app=typesense-global-2-sts"
          secretName: typesense-cluster-bootstrap-key
          port: 8108
          snapshotPath: /var/lib/typesense/snapshots
        cooldown:
          duration: "3m"
        cronjob:
          schedule: "*/30 * * * *"
        image:
          tag: "sha-abc1234"      # pin to a specific image SHA from ghcr.io
  destination:
    server: https://kubernetes.default.svc
    namespace: typesense-controller
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

Apply it:

```bash
kubectl apply -f argocd-application.yaml
```

Then sync:

```bash
argocd app sync typesense-snapshot-controller
```

### 3. Pin the image tag

The `image.tag` defaults to `latest`. For production, pin to a specific image digest published by the release pipeline:

```bash
# List available tags
gh api /users/mfeldheim/packages/container/typesense-snapshot-controller/versions \
  --jq '.[].metadata.container.tags[]' | head -20
```

Set `image.tag` in your ArgoCD values to the `sha-<short>` tag for the commit you want to run, or to a semver tag after a release is cut.

---

## Releasing a new version

Tag a commit to trigger the release pipeline, which builds and pushes to ghcr.io with the version tag:

```bash
git tag v1.0.0
git push origin v1.0.0
```

The pipeline publishes:
- `ghcr.io/mfeldheim/typesense-snapshot-controller:1.0.0`
- `ghcr.io/mfeldheim/typesense-snapshot-controller:1.0`
- `ghcr.io/mfeldheim/typesense-snapshot-controller:latest`

Update `image.tag` in your ArgoCD Application values and sync.

---

## Operations

### Check controller health

```bash
# Pod status — expect 2 running, one holding the leader lease
kubectl get pods -n typesense-controller

# Which pod is the leader
kubectl get lease typesense-snapshot-controller -n typesense-controller -o jsonpath='{.spec.holderIdentity}'

# Controller logs
kubectl logs -n typesense-controller -l app.kubernetes.io/name=typesense-snapshot-controller -f
```

### Check cooldown state

The cooldown ConfigMap records the last snapshot time and trigger:

```bash
kubectl get configmap typesense-snapshot-cooldown -n typesense-controller -o yaml
```

Example output:
```yaml
data:
  last-snapshot-time: "2026-05-06T14:32:01Z"
  trigger: eviction
```

To force the next trigger to run regardless of cooldown (e.g., after a failed snapshot), clear the ConfigMap:

```bash
kubectl patch configmap typesense-snapshot-cooldown -n typesense-controller \
  --type merge -p '{"data":{}}'
```

### Prometheus metrics

The controller exposes metrics on `:9090/metrics`. Useful queries:

```promql
# Snapshot rate by trigger over the last hour
rate(snapshot_success_total[1h])

# Failure rate
rate(snapshot_failure_total[5m])

# 95th percentile snapshot duration
histogram_quantile(0.95, rate(snapshot_duration_seconds_bucket[10m]))
```

Trigger label values: `scheduled`, `eviction`, `node-disruption`.

### CronJob history

```bash
# Last 3 job runs
kubectl get jobs -n typesense-controller --sort-by=.metadata.creationTimestamp | tail -5

# Logs from the most recent run
kubectl logs -n typesense-controller \
  $(kubectl get pods -n typesense-controller -l app.kubernetes.io/name=typesense-snapshot-controller \
    --sort-by=.metadata.creationTimestamp -o name | tail -1)
```

### Triggering a manual snapshot

Run the one-shot command directly from a temporary pod using the same image:

```bash
kubectl run snapshot-manual \
  --image=ghcr.io/mfeldheim/typesense-snapshot-controller:latest \
  --restart=Never \
  --rm -it \
  --namespace typesense-controller \
  --overrides='{
    "spec": {
      "serviceAccountName": "typesense-snapshot-controller",
      "containers": [{
        "name": "snapshot-manual",
        "image": "ghcr.io/mfeldheim/typesense-snapshot-controller:latest",
        "command": ["/typesense-snapshot-controller", "run"],
        "env": [
          {"name": "NAMESPACE", "value": "typesense-controller"},
          {"name": "POD_LABEL_SELECTOR", "value": "app=typesense-global-2-sts"},
          {"name": "SECRET_NAME", "value": "typesense-cluster-bootstrap-key"},
          {"name": "COOLDOWN_CONFIGMAP", "value": "typesense-snapshot-cooldown"}
        ]
      }]
    }
  }'
```

> If the cooldown window is active this will exit cleanly without taking a snapshot. Clear the ConfigMap first if you want to force it.

### Adjusting the snapshot schedule

Update the ArgoCD Application values and sync:

```yaml
cronjob:
  schedule: "0 * * * *"   # hourly instead of every 30 minutes
```

```bash
argocd app sync typesense-snapshot-controller
```

### Disabling event-driven snapshots

To run scheduled snapshots only (no eviction/node-disruption triggers), scale the controller Deployment to zero:

```bash
kubectl scale deployment typesense-snapshot-controller \
  -n typesense-controller --replicas=0
```

The CronJob continues to run independently.

---

## Troubleshooting

**Snapshots not triggering on eviction**

Check that the pod label selector matches your Typesense pods:
```bash
kubectl get pods -n typesense-controller -l app=typesense-global-2-sts
```
If no pods are returned, update `typesense.podLabelSelector` in Helm values.

**`no ready pod found` errors in logs**

The controller probes `/readyz` on each Typesense pod to find the leader. If all pods are unready (e.g., during a rolling restart), discovery fails and the snapshot is skipped. This is expected — the snapshot will retry on the next trigger.

**`snapshot_failure_total` increasing**

Check the controller logs for the root cause:
```bash
kubectl logs -n typesense-controller \
  -l app.kubernetes.io/name=typesense-snapshot-controller \
  --since=1h | grep "error"
```

Common causes: wrong API key in Secret, Typesense pod unreachable (check NetworkPolicy), snapshot path not writable on the Typesense pod.

**Cooldown stuck / snapshot never runs**

If `last-snapshot-time` in the ConfigMap is far in the future (clock skew), the controller will be permanently in cooldown. Fix by clearing the ConfigMap:
```bash
kubectl patch configmap typesense-snapshot-cooldown -n typesense-controller \
  --type merge -p '{"data":{}}'
```
