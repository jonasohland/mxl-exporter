# mxl-exporter

A Prometheus metrics exporter for [MXL (Media eXchange Layer)](https://github.com/dmf-mxl/mxl) domains and flows. It discovers MXL media flows on a Linux system, monitors their runtime state in real time, and exposes Prometheus-compatible metrics via HTTP.

It does **not** participate in MXL data transport — it is a passive observer that reads what the MXL runtime writes to disk/shared memory.

---

## Metrics

| Metric | Type | Description |
|---|---|---|
| `mxl_flow_metadata` | Counter | Static labels for all flow properties (dummy value 1) |
| `mxl_flow_present` | Gauge | 1 if flow is currently on the system |
| `mxl_flow_active` | Gauge | 1 if head index is progressing (data is flowing) |
| `mxl_flow_head_index_grains` | Gauge | Current write head position in grains |
| `mxl_flow_latency_grains` | Gauge | Estimated latency in grains |
| `mxl_flow_last_write_time_ns` | Gauge | TAI timestamp of last write (nanoseconds) |
| `mxl_flow_last_read_time_ns` | Gauge | TAI timestamp of last read (nanoseconds) |
| `mxl_flow_rate_num` / `_den` | Gauge | Grain rate numerator / denominator |
| `mxl_flow_payload_slice_size_bytes` | Gauge | Slice size (discrete flows: video/data) |
| `mxl_flow_ring_buffer_size_grains` | Gauge | Ring buffer depth in grains (discrete) |
| `mxl_flow_channels_total` | Gauge | Channel count (continuous flows: audio) |
| `mxl_flow_ring_buffer_size_samples` | Gauge | Ring buffer depth in samples (continuous) |
| `mxl_domain_num_flows` | Gauge | Number of flows in a domain |
| `mxl_domain_size_bytes` | Gauge | Total size of all flow data in a domain |
| `mxl_domain_fs_space_total_bytes` | Gauge | Total filesystem capacity |
| `mxl_domain_fs_space_available_bytes` | Gauge | Available filesystem space |
| `mxl_domain_fs_space_used_bytes` | Gauge | Used filesystem space |

---

## CLI Flags

```
-l, --listen              HTTP listen address (default: 127.0.0.1:2284)
-s, --search PATH         Directory to scan for MXL domains (repeatable)
-d, --domain PATH         Static domain path, never auto-removed (repeatable)
    --default-lifetime    TTL for removed entries (default: 24h)
    --fs-lifetime         Override TTL for filesystem entries
    --domain-lifetime     Override TTL for domain entries
    --flow-lifetime       Override TTL for flow entries
```

---

## Kubernetes Deployment

The exporter runs as a sidecar container inside your existing MXL application pod, sharing its domain volume read-only.

```yaml
# Deployment → spec.template.spec.containers
containers:
  - name: your-app
    # ...
  - name: mxl-exporter
    image: <your-registry>/mxl-exporter:latest
    args:
      - --listen=0.0.0.0:2284
      - --search=/domain
    ports:
      - name: metrics
        containerPort: 2284
    volumeMounts:
      - name: domain-volume
        mountPath: /domain
        readOnly: true
```

Add the label to your pod template for Prometheus discovery:

```yaml
# Deployment → spec.template.metadata.labels
labels:
  mxl-metrics: "true"
```

**With Prometheus Operator** — use a `PodMonitor`:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: mxl-exporter
spec:
  selector:
    matchLabels:
      mxl-metrics: "true"
  podMetricsEndpoints:
    - port: metrics
```

**Without Prometheus Operator** — use pod annotations:

```yaml
annotations:
  prometheus.io/scrape: "true"
  prometheus.io/port: "2284"
```

---

## Build

```bash
make mxl-exporter        # local binary → build/mxl-exporter
docker build -t mxl-exporter .
```

---

## License

See [LICENSE](LICENSE).
