package advisor

import (
	"context"
	"fmt"
	"time"

	"github.com/kavix/k8s-advisor/internal/classifier"
	"github.com/kavix/k8s-advisor/internal/models"
	"github.com/kavix/k8s-advisor/internal/storage"
)

var actionDescriptions = map[string]string{
	"increase_memory_limits":  "Increase container memory requests/limits and profile heap usage for leaks.",
	"increase_cpu_limits":     "Increase CPU limits or configure HorizontalPodAutoscaler (HPA).",
	"inspect_logs_and_probes": "Inspect pod logs (`kubectl logs --previous`) and tune liveness/readiness probes.",
	"scale_cluster_nodes":     "Scale node pool capacity or relax pod affinity/resource requests.",
	"cordon_and_drain_node":   "Cordon and drain the affected node, then inspect kubelet and disk usage.",
	"no_action_monitor":       "No immediate remediation required; continue monitoring.",
}

// Engine combines real-time K8s events with historical TimescaleDB baselines and Jev classification.
type Engine struct {
	store      storage.AdvisorStore
	classifier classifier.Classifier
}

// NewEngine creates an Advisor Engine.
func NewEngine(store storage.AdvisorStore, cls classifier.Classifier) *Engine {
	return &Engine{
		store:      store,
		classifier: cls,
	}
}

// AnalyzeEvent enriches a Kubernetes event with historical baselines, classifies it, and persists the advisory.
func (e *Engine) AnalyzeEvent(ctx context.Context, event models.K8sEventRecord) (models.Advisory, error) {
	baseline, err := e.store.GetHistoricalBaseline(ctx, event.ClusterName, event.Namespace, event.InvolvedName)
	if err != nil {
		baseline = models.HistoricalBaseline{}
	}

	result := e.classifier.Classify(ctx, event, baseline)
	summary, ok := actionDescriptions[result.RecommendedAction]
	if !ok {
		summary = fmt.Sprintf("Execute recommended action: %s", result.RecommendedAction)
	}

	advisory := models.Advisory{
		CreatedAt:      time.Now().UTC(),
		ClusterName:    event.ClusterName,
		Namespace:      event.Namespace,
		ResourceKind:   event.InvolvedKind,
		ResourceName:   event.InvolvedName,
		Classification: result,
		ActionSummary:  summary,
		Context: map[string]any{
			"event_reason":      event.Reason,
			"event_message":     event.Message,
			"warning_events_1h": baseline.WarningEvents1h,
			"warning_events_7d": baseline.WarningEvents7d,
			"max_restarts_24h":  baseline.MaxRestarts24h,
		},
	}

	if err := e.store.SaveAdvisory(ctx, advisory); err != nil {
		return advisory, err
	}
	return advisory, nil
}
