package models

import "time"

// K8sMetricPoint represents a single point-in-time CPU/memory measurement for a Pod or Node.
type K8sMetricPoint struct {
	Time               time.Time         `json:"time"`
	ClusterName        string            `json:"cluster_name"`
	Namespace          string            `json:"namespace"`
	ResourceKind       string            `json:"resource_kind"` // "Pod" | "Node"
	ResourceName       string            `json:"resource_name"`
	CPUMillicores      float64           `json:"cpu_millicores"`
	MemoryBytes        int64             `json:"memory_bytes"`
	CPULimitMillicores *float64          `json:"cpu_limit_millicores,omitempty"`
	MemoryLimitBytes   *int64            `json:"memory_limit_bytes,omitempty"`
	RestartCount       int               `json:"restart_count"`
	Labels             map[string]string `json:"labels,omitempty"`
}

// K8sEventRecord represents a Kubernetes cluster event captured from the Watch API.
type K8sEventRecord struct {
	Time            time.Time `json:"time"`
	EventUID        string    `json:"event_uid"`
	ClusterName     string    `json:"cluster_name"`
	Namespace       string    `json:"namespace"`
	InvolvedKind    string    `json:"involved_kind"`
	InvolvedName    string    `json:"involved_name"`
	EventType       string    `json:"event_type"` // "Normal" | "Warning"
	Reason          string    `json:"reason"`
	Message         string    `json:"message"`
	SourceComponent string    `json:"source_component,omitempty"`
	Count           int       `json:"count"`
}

// HistoricalBaseline holds aggregated 24h/7d metrics and event counts from TimescaleDB.
type HistoricalBaseline struct {
	AvgCPUMillicores24h float64 `json:"avg_cpu_millicores_24h"`
	MaxCPUMillicores24h float64 `json:"max_cpu_millicores_24h"`
	AvgMemoryBytes24h   int64   `json:"avg_memory_bytes_24h"`
	MaxMemoryBytes24h   int64   `json:"max_memory_bytes_24h"`
	WarningEvents1h     int     `json:"warning_events_1h"`
	WarningEvents7d     int     `json:"warning_events_7d"`
	MaxRestarts24h      int     `json:"max_restarts_24h"`
}

// ClassificationResult holds structured classification output from Jev or rule fallback.
type ClassificationResult struct {
	IssueCategory              string  `json:"issue_category"`
	Severity                   string  `json:"severity"`
	SeverityScore              float64 `json:"severity_score"`
	ImmediateActionProbability float64 `json:"immediate_action_probability"`
	RecommendedAction          string  `json:"recommended_action"`
	ClassificationSource       string  `json:"classification_source"` // "jev" | "rule_fallback"
}

// Advisory is an actionable operational recommendation produced by the Advisor Engine.
type Advisory struct {
	CreatedAt      time.Time            `json:"created_at"`
	ClusterName    string               `json:"cluster_name"`
	Namespace      string               `json:"namespace"`
	ResourceKind   string               `json:"resource_kind"`
	ResourceName   string               `json:"resource_name"`
	Classification ClassificationResult `json:"classification"`
	ActionSummary  string               `json:"action_summary"`
	Context        map[string]any       `json:"context,omitempty"`
}
