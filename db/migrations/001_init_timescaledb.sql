-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- 1. Kubernetes Resource Metrics Hypertable
CREATE TABLE IF NOT EXISTS k8s_metrics (
    time TIMESTAMPTZ NOT NULL,
    cluster_name TEXT NOT NULL,
    namespace TEXT NOT NULL,
    resource_kind TEXT NOT NULL, -- 'Pod' | 'Node'
    resource_name TEXT NOT NULL,
    cpu_millicores DOUBLE PRECISION NOT NULL,
    memory_bytes BIGINT NOT NULL,
    cpu_limit_millicores DOUBLE PRECISION,
    memory_limit_bytes BIGINT,
    restart_count INTEGER NOT NULL DEFAULT 0,
    labels JSONB NOT NULL DEFAULT '{}'::jsonb
);

SELECT create_hypertable(
    'k8s_metrics',
    by_range('time', INTERVAL '1 day'),
    if_not_exists => TRUE
);

CREATE INDEX IF NOT EXISTS idx_k8s_metrics_lookup
    ON k8s_metrics (cluster_name, namespace, resource_kind, resource_name, time DESC);

ALTER TABLE k8s_metrics SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'cluster_name, namespace, resource_kind, resource_name',
    timescaledb.compress_orderby = 'time DESC'
);

SELECT add_compression_policy('k8s_metrics', INTERVAL '7 days', if_not_exists => TRUE);
SELECT add_retention_policy('k8s_metrics', INTERVAL '90 days', if_not_exists => TRUE);

-- 2. Kubernetes Events Hypertable
CREATE TABLE IF NOT EXISTS k8s_events (
    time TIMESTAMPTZ NOT NULL,
    event_uid TEXT NOT NULL,
    cluster_name TEXT NOT NULL,
    namespace TEXT NOT NULL,
    involved_kind TEXT NOT NULL,
    involved_name TEXT NOT NULL,
    event_type TEXT NOT NULL, -- 'Normal' | 'Warning'
    reason TEXT NOT NULL,
    message TEXT NOT NULL,
    source_component TEXT,
    count INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (time, event_uid)
);

SELECT create_hypertable(
    'k8s_events',
    by_range('time', INTERVAL '7 days'),
    if_not_exists => TRUE
);

CREATE INDEX IF NOT EXISTS idx_k8s_events_lookup
    ON k8s_events (cluster_name, namespace, involved_name, reason, time DESC);

ALTER TABLE k8s_events SET (
    timescaledb.compress,
    timescaledb.compress_segmentby = 'cluster_name, namespace, reason',
    timescaledb.compress_orderby = 'time DESC'
);

SELECT add_compression_policy('k8s_events', INTERVAL '14 days', if_not_exists => TRUE);
SELECT add_retention_policy('k8s_events', INTERVAL '180 days', if_not_exists => TRUE);

-- 3. Continuous Aggregate: Hourly Resource Baselines
CREATE MATERIALIZED VIEW IF NOT EXISTS k8s_metrics_hourly
WITH (timescaledb.continuous) AS
SELECT
    time_bucket('1 hour', time) AS bucket,
    cluster_name,
    namespace,
    resource_kind,
    resource_name,
    AVG(cpu_millicores) AS avg_cpu_millicores,
    MAX(cpu_millicores) AS max_cpu_millicores,
    AVG(memory_bytes)::BIGINT AS avg_memory_bytes,
    MAX(memory_bytes) AS max_memory_bytes,
    MAX(restart_count) AS max_restart_count
FROM k8s_metrics
GROUP BY bucket, cluster_name, namespace, resource_kind, resource_name
WITH NO DATA;

SELECT add_continuous_aggregate_policy(
    'k8s_metrics_hourly',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '15 minutes',
    schedule_interval => INTERVAL '15 minutes',
    if_not_exists => TRUE
);

-- 4. Advisor Recommendations Table
CREATE TABLE IF NOT EXISTS advisor_recommendations (
    id BIGSERIAL PRIMARY KEY,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    cluster_name TEXT NOT NULL,
    namespace TEXT NOT NULL,
    resource_kind TEXT NOT NULL,
    resource_name TEXT NOT NULL,
    issue_category TEXT NOT NULL,
    severity TEXT NOT NULL,
    severity_score DOUBLE PRECISION NOT NULL,
    immediate_action_probability DOUBLE PRECISION NOT NULL,
    recommended_action TEXT NOT NULL,
    action_summary TEXT NOT NULL,
    classification_source TEXT NOT NULL, -- 'jev' | 'rule_fallback'
    context JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_advisor_recs_recent
    ON advisor_recommendations (cluster_name, namespace, created_at DESC);
