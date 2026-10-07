# mxl-exporter Helm chart

This chart installs [mxl-exporter](https://github.com/jonasohland/mxl-exporter) as a
DaemonSet. Every node gets one pod. Each pod mounts the shared memory directory of
its node, `/dev/shm`, read only, and scans it for MXL domains.

The exporter opens every file read only and maps flow data read only. It writes
nothing to the mount.

## Install

```bash
helm install mxl-exporter ./charts/mxl-exporter \
  --namespace monitoring --create-namespace
```

With the Prometheus Operator:

```bash
helm install mxl-exporter ./charts/mxl-exporter \
  --namespace monitoring --create-namespace \
  --set podMonitor.enabled=true \
  --set podMonitor.labels.release=kube-prometheus-stack
```

The `podMonitor.labels` value must match the `podMonitorSelector` of your Prometheus
resource. Read it with:

```bash
kubectl get prometheus -A -o jsonpath='{.items[*].spec.podMonitorSelector}'
```

## The /dev/shm mount

The mount replaces the shared memory volume that the container runtime creates for
the pod. That is safe here, because the exporter does not use shared memory itself.

The container mount path is `/dev/shm`, the same as the host path. Keep both equal.
The `domain` and `fs_path` labels contain the path as the container sees it, so a
different mount path produces labels that do not match the node.

To scan a second location, for example a hostPath that holds domains on a data disk:

```yaml
extraVolumes:
  - name: domains
    hostPath:
      path: /srv/mxl
      type: Directory
extraVolumeMounts:
  - name: domains
    mountPath: /srv/mxl
    readOnly: true
exporter:
  search:
    - /srv/mxl
```

## File permissions

The pod runs as user 65534 (`nobody`) and reads the domain files of other processes.
If the MXL application creates its files without read permission for other users, the
exporter logs errors and reports no flows. Two ways to fix that:

- Run the exporter as the same group as the MXL application:

  ```yaml
  podSecurityContext:
    runAsNonRoot: true
    runAsUser: 65534
    runAsGroup: <gid of the MXL application>
  ```

- Run the exporter as root:

  ```yaml
  podSecurityContext:
    runAsNonRoot: false
    runAsUser: 0
  ```

## Values

| Key | Default | Description |
|---|---|---|
| `image.repository` | `ghcr.io/jonasohland/mxl-exporter` | Container image |
| `image.tag` | `""` | Image tag. Empty means the chart `appVersion` |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy |
| `imagePullSecrets` | `[]` | Pull secrets for a private registry |
| `nameOverride` | `""` | Overrides the chart name |
| `fullnameOverride` | `""` | Overrides the generated resource name |
| `exporter.listen.address` | `0.0.0.0` | Bind address |
| `exporter.listen.port` | `2284` | Bind port and container port |
| `exporter.search` | `[]` | Extra directories to scan (`--search`) |
| `exporter.domains` | `[]` | Static domain paths (`--domain`) |
| `exporter.lifetime.default` | `""` | `--default-lifetime`. Empty means 24h |
| `exporter.lifetime.filesystem` | `""` | `--fs-lifetime` |
| `exporter.lifetime.domain` | `""` | `--domain-lifetime` |
| `exporter.lifetime.flow` | `""` | `--flow-lifetime` |
| `exporter.extraArgs` | `[]` | Extra command line arguments |
| `exporter.env` | `[]` | Extra environment variables |
| `exporter.envFrom` | `[]` | Environment from ConfigMaps or Secrets |
| `shm.enabled` | `true` | Mount the shared memory directory of the node |
| `shm.hostPath` | `/dev/shm` | Path on the node |
| `shm.mountPath` | `/dev/shm` | Path in the container |
| `shm.search` | `true` | Pass `shm.mountPath` to `--search` |
| `extraVolumes` | `[]` | Extra pod volumes |
| `extraVolumeMounts` | `[]` | Extra container volume mounts |
| `serviceAccount.create` | `true` | Create a service account |
| `serviceAccount.automount` | `false` | Mount the API token in the pod |
| `serviceAccount.annotations` | `{}` | Service account annotations |
| `serviceAccount.name` | `""` | Service account name |
| `service.enabled` | `true` | Create a Service |
| `service.type` | `ClusterIP` | Service type |
| `service.port` | `2284` | Service port |
| `service.clusterIP` | `None` | `None` makes the Service headless |
| `service.annotations` | `{}` | Service annotations |
| `service.labels` | `{}` | Extra Service labels |
| `podMonitor.enabled` | `false` | Create a PodMonitor |
| `podMonitor.namespace` | `""` | PodMonitor namespace. Empty means the release namespace |
| `podMonitor.labels` | `{}` | Extra PodMonitor labels, for the Prometheus selector |
| `podMonitor.interval` | `30s` | Scrape interval |
| `podMonitor.scrapeTimeout` | `""` | Scrape timeout |
| `podMonitor.honorLabels` | `false` | Keep target labels on conflict |
| `podMonitor.relabelings` | `[]` | Target relabel rules |
| `podMonitor.metricRelabelings` | `[]` | Metric relabel rules |
| `serviceMonitor.*` | see `podMonitor` | Same keys, for a ServiceMonitor |
| `updateStrategy` | `RollingUpdate`, `maxUnavailable: 1` | DaemonSet update strategy |
| `terminationGracePeriodSeconds` | `30` | Shutdown grace period |
| `podAnnotations` | `{}` | Pod annotations |
| `podLabels` | `{}` | Extra pod labels |
| `podSecurityContext` | runs as 65534, non root | Pod security context |
| `securityContext` | no capabilities, read only root | Container security context |
| `resources` | `{}` | Requests and limits |
| `livenessProbe.enabled` | `true` | HTTP probe on `/metrics` |
| `readinessProbe.enabled` | `true` | HTTP probe on `/metrics` |
| `hostNetwork` | `false` | Use the network namespace of the node |
| `dnsPolicy` | `""` | DNS policy. Empty and `hostNetwork: true` gives `ClusterFirstWithHostNet` |
| `priorityClassName` | `""` | Priority class |
| `nodeSelector` | `{}` | Node selector |
| `tolerations` | `[]` | Tolerations. Empty means schedulable nodes only |
| `affinity` | `{}` | Affinity rules |

The chart sets no resource requests or limits. The exporter maps the data file of
every flow, but it reads headers only, so the resident set stays small. A test on a
three node k3s cluster measured 2 MiB with no flows and 7 MiB with five flows, at 1 to
3 millicores. Memory still grows with the flow count. Measure your own workload before
you set a limit.
