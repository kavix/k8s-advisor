# K8s Advisor Architecture & Implementation Plan (Go)

## 1. System Architecture

```text
+-------------------------------------------------------------------+
| Kubernetes Cluster                                                |
|  - K8s API Server (Events Watch, Pod/Node Status)                 |
|  - Metrics Server (metrics.k8s.io/v1beta1)                        |
|  - OTel Collector DaemonSet/Deployment (kubeletstats, k8scluster) |
+-------------------+---------------------------+-------------------+
                    |                           | OTLP (gRPC/4317)
                    v                           v
+-------------------------------------------------------------------+
| k8s-advisor Service (Go)                                          |
|  1. Collector  (client-go Event Watcher + Metrics Poller)         |
|  2. Storage    (pgxpool + TimescaleDB Hypertables & CAGGs)        |
|  3. Advisor    (Real-Time + Historical Baseline Correlator)       |
|  4. Classifier (Jev System-One HTTP Client + Rule Fallback)       |
|  5. API        (net/http: /advisories, /analyze, /healthz/readyz) |
+-------------------+---------------------------+-------------------+
                    |                           |
                    v                           v
        +---------------------+     +-----------------------+
        | TimescaleDB /       |     | TypeSafe AI (Jev)     |
        | Tiger Cloud         |     | System-One API        |
        +---------------------+     +-----------------------+
```

## 2. Why Go for K8s Advisor?

- **Native Kubernetes ecosystem**: `k8s.io/client-go` and `k8s.io/metrics` are the canonical Kubernetes libraries with built-in informer caches, watch bookmarking, and `resource.Quantity` parsing.
- **High-throughput TimescaleDB writes**: `github.com/jackc/pgx/v5` supports `CopyFrom` and pipelined batches with minimal memory overhead.
- **Low-latency concurrency**: Goroutines + bounded channels decouple K8s watch loops from DB batching and Jev HTTP calls.
- **First-class OpenTelemetry**: `go.opentelemetry.io/otel` has mature, low-overhead SDKs and OTLP exporters.

## 3. TimescaleDB Hypertable Strategy

- **`k8s_metrics`**: Partitioned by `time` (1-day chunks), `segmentby = 'cluster_name, namespace, resource_kind, resource_name'`, `orderby = 'time DESC'`. Compression after 7 days; retention 90 days.
- **`k8s_events`**: Partitioned by `time` (7-day chunks), `segmentby = 'cluster_name, namespace, reason'`. Compression after 14 days; retention 180 days.
- **`k8s_metrics_hourly` (Continuous Aggregate)**: Pre-computes `avg(cpu_millicores)`, `max(cpu_millicores)`, `avg(memory_bytes)`, `max(memory_bytes)`, and `max(restart_count)` per hour so baseline comparisons take milliseconds.

## 4. Combining Real-Time Data with Historical Insight

1. **Real-Time Signal**: An incoming `Warning` event (e.g., `OOMKilling`, `BackOff`, `FailedScheduling`) triggers an investigation.
2. **Historical Context**: Query `k8s_metrics_hourly` (24h) and `k8s_events` (1h / 7d) for the same `(cluster_name, namespace, resource_name)`.
3. **Jev Classification**: Send the compact structured `state` (real-time event + historical summary) to Jev using `Choice`, `Score`, and `Noul` questions.
4. **Deterministic Fallback**: If Jev is disabled, rate-limited, or unreachable, a deterministic rule engine classifies well-known K8s reasons (`OOMKilled`, `CrashLoopBackOff`, `FailedScheduling`, `NodePressure`).

## 5. OpenTelemetry Reliability Assessment

Using OpenTelemetry (OTel) for both **cluster telemetry ingestion** and **advisor self-observability** is highly reliable for production Kubernetes when configured with resilience primitives:

- **OTel Collector resiliency**:
  - `memory_limiter` processor prevents OOMKills on collector pods under bursty event storms.
  - `batch` processor groups telemetry into efficient payloads, reducing network & DB overhead.
  - `sending_queue` + `retry_on_failure` (backed by `file_storage` extension) buffers telemetry across transient TimescaleDB or network outages.
- **Risks & Mitigations**:
  - *Cardinality explosion*: Drop ephemeral pod UIDs/container hashes in the OTel Collector `transform`/`attributes` processor.
  - *Collector SPOF*: Run `kubeletstats` as a `DaemonSet` and `k8s_cluster`/`k8s_events` as a leader-elected `Deployment`.
  - *Feedback loops*: Use bounded `BatchSpanProcessor` queues and timeouts so OTLP export failures never block advisor request handling.

## 6. Phased Implementation Plan

1. **Phase 1 (Foundation — Initial Repo)**: Go module, TimescaleDB hypertable migrations, config, domain models, Jev System-One HTTP client + rule fallback, Advisor correlator, HTTP API, OTel setup, unit tests, K8s manifests.
2. **Phase 2 (Live Cluster & DB Integration)**: Wire `k8s.io/client-go` + `pgxpool` in `cmd/advisor` background workers and test against `kind`/`k3d` + Tiger Cloud.
3. **Phase 3 (OTel Collector Pipeline)**: Deploy OTel Collector `Deployment` + `DaemonSet` shipping K8s metrics/events directly into TimescaleDB / advisor ingestion.
4. **Phase 4 (Ecosystem Integrations)**: Connect with external platforms (e.g., Backstage portal cards, OpenChoreo Observability Plane alerts, Kueue queue-starvation advisor).
