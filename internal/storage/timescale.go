package storage

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kavix/k8s-advisor/internal/models"
)

// AdvisorStore abstracts persistence for TimescaleDB and in-memory test fakes.
type AdvisorStore interface {
	HealthCheck(ctx context.Context) error
	InsertMetrics(ctx context.Context, points []models.K8sMetricPoint) (int64, error)
	InsertEvents(ctx context.Context, events []models.K8sEventRecord) (int, error)
	GetHistoricalBaseline(ctx context.Context, clusterName, namespace, resourceName string) (models.HistoricalBaseline, error)
	SaveAdvisory(ctx context.Context, advisory models.Advisory) error
	ListAdvisories(ctx context.Context, limit int) ([]models.Advisory, error)
}

// TimescaleRepository implements AdvisorStore backed by PostgreSQL/TimescaleDB.
type TimescaleRepository struct {
	pool *pgxpool.Pool
}

// NewTimescaleRepository opens a pgx connection pool against TimescaleDB.
func NewTimescaleRepository(ctx context.Context, dsn string) (*TimescaleRepository, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, err
	}
	return &TimescaleRepository{pool: pool}, nil
}

// Close shuts down the connection pool.
func (r *TimescaleRepository) Close() {
	r.pool.Close()
}

// HealthCheck verifies TimescaleDB connectivity.
func (r *TimescaleRepository) HealthCheck(ctx context.Context) error {
	var one int
	return r.pool.QueryRow(ctx, "SELECT 1").Scan(&one)
}

// InsertMetrics streams resource metrics into k8s_metrics using PostgreSQL COPY.
func (r *TimescaleRepository) InsertMetrics(ctx context.Context, points []models.K8sMetricPoint) (int64, error) {
	if len(points) == 0 {
		return 0, nil
	}
	cols := []string{
		"time", "cluster_name", "namespace", "resource_kind", "resource_name",
		"cpu_millicores", "memory_bytes", "cpu_limit_millicores", "memory_limit_bytes",
		"restart_count", "labels",
	}
	rows := make([][]any, 0, len(points))
	for _, p := range points {
		labelsJSON, _ := json.Marshal(p.Labels)
		rows = append(rows, []any{
			p.Time, p.ClusterName, p.Namespace, p.ResourceKind, p.ResourceName,
			p.CPUMillicores, p.MemoryBytes, p.CPULimitMillicores, p.MemoryLimitBytes,
			p.RestartCount, string(labelsJSON),
		})
	}
	return r.pool.CopyFrom(ctx, pgx.Identifier{"k8s_metrics"}, cols, pgx.CopyFromRows(rows))
}

// InsertEvents batch-upserts Kubernetes events into k8s_events.
func (r *TimescaleRepository) InsertEvents(ctx context.Context, events []models.K8sEventRecord) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	const sql = `
		INSERT INTO k8s_events (
			time, event_uid, cluster_name, namespace, involved_kind,
			involved_name, event_type, reason, message, source_component, count
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (time, event_uid) DO UPDATE SET
			count = EXCLUDED.count,
			message = EXCLUDED.message`

	batch := &pgx.Batch{}
	for _, e := range events {
		batch.Queue(sql,
			e.Time, e.EventUID, e.ClusterName, e.Namespace, e.InvolvedKind,
			e.InvolvedName, e.EventType, e.Reason, e.Message, e.SourceComponent, e.Count,
		)
	}
	br := r.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range events {
		if _, err := br.Exec(); err != nil {
			return 0, err
		}
	}
	return len(events), nil
}

// GetHistoricalBaseline queries the 24h continuous aggregate and 1h/7d event counts.
func (r *TimescaleRepository) GetHistoricalBaseline(ctx context.Context, clusterName, namespace, resourceName string) (models.HistoricalBaseline, error) {
	var b models.HistoricalBaseline
	const metricsSQL = `
		SELECT
			COALESCE(AVG(avg_cpu_millicores), 0),
			COALESCE(MAX(max_cpu_millicores), 0),
			COALESCE(AVG(avg_memory_bytes), 0)::BIGINT,
			COALESCE(MAX(max_memory_bytes), 0),
			COALESCE(MAX(max_restart_count), 0)
		FROM k8s_metrics_hourly
		WHERE cluster_name = $1
		  AND namespace = $2
		  AND resource_name = $3
		  AND bucket >= NOW() - INTERVAL '24 hours'`

	if err := r.pool.QueryRow(ctx, metricsSQL, clusterName, namespace, resourceName).Scan(
		&b.AvgCPUMillicores24h,
		&b.MaxCPUMillicores24h,
		&b.AvgMemoryBytes24h,
		&b.MaxMemoryBytes24h,
		&b.MaxRestarts24h,
	); err != nil {
		return b, err
	}

	const eventsSQL = `
		SELECT
			COUNT(*) FILTER (WHERE time >= NOW() - INTERVAL '1 hour') AS warn_1h,
			COUNT(*) AS warn_7d
		FROM k8s_events
		WHERE cluster_name = $1
		  AND namespace = $2
		  AND involved_name = $3
		  AND event_type = 'Warning'
		  AND time >= NOW() - INTERVAL '7 days'`

	if err := r.pool.QueryRow(ctx, eventsSQL, clusterName, namespace, resourceName).Scan(
		&b.WarningEvents1h,
		&b.WarningEvents7d,
	); err != nil {
		return b, err
	}

	return b, nil
}

// SaveAdvisory persists an advisor recommendation to TimescaleDB.
func (r *TimescaleRepository) SaveAdvisory(ctx context.Context, a models.Advisory) error {
	const sql = `
		INSERT INTO advisor_recommendations (
			created_at, cluster_name, namespace, resource_kind, resource_name,
			issue_category, severity, severity_score, immediate_action_probability,
			recommended_action, action_summary, classification_source, context
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13::jsonb)`

	ctxJSON, _ := json.Marshal(a.Context)
	_, err := r.pool.Exec(ctx, sql,
		a.CreatedAt, a.ClusterName, a.Namespace, a.ResourceKind, a.ResourceName,
		a.Classification.IssueCategory, a.Classification.Severity, a.Classification.SeverityScore,
		a.Classification.ImmediateActionProbability, a.Classification.RecommendedAction,
		a.ActionSummary, a.Classification.ClassificationSource, string(ctxJSON),
	)
	return err
}

// ListAdvisories returns the most recent advisor recommendations.
func (r *TimescaleRepository) ListAdvisories(ctx context.Context, limit int) ([]models.Advisory, error) {
	if limit <= 0 {
		limit = 50
	}
	const sql = `
		SELECT
			created_at, cluster_name, namespace, resource_kind, resource_name,
			issue_category, severity, severity_score, immediate_action_probability,
			recommended_action, action_summary, classification_source, context
		FROM advisor_recommendations
		ORDER BY created_at DESC
		LIMIT $1`

	rows, err := r.pool.Query(ctx, sql, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Advisory
	for rows.Next() {
		var a models.Advisory
		var rawCtx []byte
		if err := rows.Scan(
			&a.CreatedAt, &a.ClusterName, &a.Namespace, &a.ResourceKind, &a.ResourceName,
			&a.Classification.IssueCategory, &a.Classification.Severity, &a.Classification.SeverityScore,
			&a.Classification.ImmediateActionProbability, &a.Classification.RecommendedAction,
			&a.ActionSummary, &a.Classification.ClassificationSource, &rawCtx,
		); err != nil {
			return nil, err
		}
		if len(rawCtx) > 0 {
			_ = json.Unmarshal(rawCtx, &a.Context)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
