# k8s-advisor

A Go-based Kubernetes operational advisor that combines **real-time cluster metrics & events**, **TimescaleDB hypertable historical analytics**, **OpenTelemetry observability**, and **Jev (TypeSafe AI) System-One classification** to detect issues and recommend actionable fixes for DevOps/SRE engineers.

## Architecture Overview

1. **Collection Layer (`internal/collector/`)**
   - Uses `k8s.io/client-go` informers/watchers for Kubernetes `Events` (`core/v1`) with `resourceVersion` tracking.
   - Polls Kubernetes Metrics Server (`metrics.k8s.io/v1beta1`) for Pod/Node CPU and memory usage.
2. **Storage & Historical Analytics (`internal/storage/`, `db/migrations/`)**
   - **TimescaleDB Hypertables** (`pgx/v5` + `pgxpool`) for `k8s_metrics` (1-day chunks, compressed after 7 days, retained 90 days), `k8s_events` (7-day chunks, compressed after 14 days, retained 180 days), and `advisor_recommendations`.
   - **Continuous Aggregates** (`k8s_metrics_hourly`) for fast 24h/7d historical baselines.
3. **Classification & Decision Layer (`internal/classifier/`)**
   - Integrates **Jev (TypeSafe AI)** System-One REST API (`Choice`, `Score`, `Noul` primitives) to classify incidents and select recommended actions, with a deterministic rule-based fallback if Jev is unavailable.
4. **Advisor Engine & HTTP API (`internal/advisor/`, `internal/api/`)**
   - Correlates real-time anomalies with historical TimescaleDB baselines before invoking Jev.
   - Exposes REST endpoints (`/healthz`, `/readyz`, `/api/v1/advisories`, `/api/v1/analyze`).
5. **Observability (`internal/telemetry/`)**
   - Instrumented with OpenTelemetry Go SDK (`go.opentelemetry.io/otel`) exporting traces and metrics via OTLP gRPC.

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full design, OpenTelemetry reliability analysis, and ecosystem integration notes.

## Quickstart

```bash
cp .env.example .env
# Apply TimescaleDB migrations (db/migrations/001_init_timescaledb.sql)
go run ./cmd/advisor
```

## Running Tests

```bash
go test ./...
```
