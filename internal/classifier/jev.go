package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kavix/k8s-advisor/internal/models"
)

var issueCategories = map[string]string{
	"oom_memory":         "Out-of-memory kills, memory leaks, or memory limit exhaustion",
	"crash_loop":         "Repeated container crashes, application panics, or failing health probes",
	"cpu_saturation":     "Sustained CPU throttling or CPU contention",
	"scheduling_failure": "Unschedulable pods due to insufficient CPU/memory, taints, or affinity",
	"node_pressure":      "Node disk, memory, or PID pressure, or NotReady conditions",
	"network_or_dns":     "Service discovery, DNS, or ingress connectivity failures",
	"healthy_or_noise":   "Transient or benign event requiring no intervention",
}

var recommendedActions = map[string]string{
	"increase_memory_limits":  "Raise container memory limits/requests or investigate memory leak",
	"increase_cpu_limits":     "Raise CPU requests/limits or enable HorizontalPodAutoscaler",
	"inspect_logs_and_probes": "Inspect container logs and tune liveness/readiness probes",
	"scale_cluster_nodes":     "Scale node pool or adjust pod affinity/resource requests",
	"cordon_and_drain_node":   "Cordon affected node, drain workloads, and inspect kubelet/disk",
	"no_action_monitor":       "No immediate action required; continue monitoring",
}

// Classifier classifies Kubernetes incidents enriched with historical context.
type Classifier interface {
	Classify(ctx context.Context, event models.K8sEventRecord, baseline models.HistoricalBaseline) models.ClassificationResult
}

// JevClassifier calls TypeSafe AI's Jev System-One API and falls back to deterministic rules on error.
type JevClassifier struct {
	apiKey     string
	baseURL    string
	enabled    bool
	httpClient *http.Client
}

// NewJevClassifier constructs a JevClassifier. If apiKey is empty or enabled is false, rule fallback is used.
func NewJevClassifier(apiKey, baseURL string, enabled bool) *JevClassifier {
	return NewJevClassifierWithClient(apiKey, baseURL, enabled, &http.Client{Timeout: 5 * time.Second})
}

// NewJevClassifierWithClient constructs a JevClassifier with a caller-supplied http.Client.
func NewJevClassifierWithClient(apiKey, baseURL string, enabled bool, httpClient *http.Client) *JevClassifier {
	return &JevClassifier{
		apiKey:     apiKey,
		baseURL:    strings.TrimRight(baseURL, "/"),
		enabled:    enabled && apiKey != "",
		httpClient: httpClient,
	}
}

// Classify evaluates an event + historical baseline via Jev, falling back to rules if disabled or failing.
func (c *JevClassifier) Classify(ctx context.Context, event models.K8sEventRecord, baseline models.HistoricalBaseline) models.ClassificationResult {
	if c.enabled {
		if res, err := c.classifyWithJev(ctx, event, baseline); err == nil {
			return res
		}
	}
	return ClassifyWithRules(event, baseline)
}

type jevQuestion struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions,omitempty"`
	Question     string            `json:"question,omitempty"`
	Criteria     map[string]string `json:"criteria,omitempty"`
	Levels       []string          `json:"levels,omitempty"`
}

type jevRequest struct {
	State     map[string]any         `json:"state"`
	Questions map[string]jevQuestion `json:"questions"`
}

type jevResponse struct {
	Answers struct {
		IssueCategory struct {
			Choice string `json:"choice"`
		} `json:"issue_category"`
		Severity struct {
			Level string  `json:"level"`
			Score float64 `json:"score"`
		} `json:"severity"`
		ImmediateAction struct {
			Noul float64 `json:"noul"`
		} `json:"immediate_action"`
		RecommendedAction struct {
			Choice string `json:"choice"`
		} `json:"recommended_action"`
	} `json:"answers"`
}

func (c *JevClassifier) classifyWithJev(ctx context.Context, event models.K8sEventRecord, baseline models.HistoricalBaseline) (models.ClassificationResult, error) {
	payload := jevRequest{
		State: map[string]any{
			"cluster":                          event.ClusterName,
			"namespace":                        event.Namespace,
			"resource_kind":                    event.InvolvedKind,
			"resource_name":                    event.InvolvedName,
			"event_type":                       event.EventType,
			"reason":                           event.Reason,
			"message":                          event.Message,
			"event_count":                      event.Count,
			"historical_24h_avg_cpu_millicore": baseline.AvgCPUMillicores24h,
			"historical_24h_max_memory_bytes":  baseline.MaxMemoryBytes24h,
			"historical_24h_max_restarts":      baseline.MaxRestarts24h,
			"warning_events_1h":                baseline.WarningEvents1h,
			"warning_events_7d":                baseline.WarningEvents7d,
		},
		Questions: map[string]jevQuestion{
			"issue_category": {
				Type:         "choice",
				Instructions: "Classify the primary Kubernetes operational issue.",
				Criteria:     issueCategories,
			},
			"severity": {
				Type:         "score",
				Instructions: "Rate operational severity based on blast radius and history.",
				Levels:       []string{"low", "medium", "high", "critical"},
			},
			"immediate_action": {
				Type:     "noul",
				Question: "Does this issue require immediate DevOps/SRE intervention?",
			},
			"recommended_action": {
				Type:         "choice",
				Instructions: "Select the best remediation action for a DevOps engineer.",
				Criteria:     recommendedActions,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return models.ClassificationResult{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/system_one", bytes.NewReader(body))
	if err != nil {
		return models.ClassificationResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.ClassificationResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return models.ClassificationResult{}, fmt.Errorf("jev api returned status %d", resp.StatusCode)
	}

	var parsed jevResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return models.ClassificationResult{}, err
	}

	sev := strings.ToLower(parsed.Answers.Severity.Level)
	switch sev {
	case "low", "medium", "high", "critical":
	default:
		sev = "medium"
	}

	return models.ClassificationResult{
		IssueCategory:              parsed.Answers.IssueCategory.Choice,
		Severity:                   sev,
		SeverityScore:              parsed.Answers.Severity.Score,
		ImmediateActionProbability: parsed.Answers.ImmediateAction.Noul,
		RecommendedAction:          parsed.Answers.RecommendedAction.Choice,
		ClassificationSource:       "jev",
	}, nil
}

// ClassifyWithRules provides deterministic classification when Jev is disabled or unavailable.
func ClassifyWithRules(event models.K8sEventRecord, baseline models.HistoricalBaseline) models.ClassificationResult {
	reason := strings.ToLower(event.Reason)
	msg := strings.ToLower(event.Message)

	switch {
	case strings.Contains(reason, "oom") || strings.Contains(msg, "oomkilled"):
		return models.ClassificationResult{
			IssueCategory:              "oom_memory",
			Severity:                   "high",
			SeverityScore:              0.85,
			ImmediateActionProbability: 0.90,
			RecommendedAction:          "increase_memory_limits",
			ClassificationSource:       "rule_fallback",
		}
	case strings.Contains(reason, "backoff") || strings.Contains(reason, "unhealthy") || baseline.MaxRestarts24h >= 5:
		sev := "medium"
		if baseline.WarningEvents1h >= 3 {
			sev = "high"
		}
		return models.ClassificationResult{
			IssueCategory:              "crash_loop",
			Severity:                   sev,
			SeverityScore:              0.75,
			ImmediateActionProbability: 0.80,
			RecommendedAction:          "inspect_logs_and_probes",
			ClassificationSource:       "rule_fallback",
		}
	case strings.Contains(reason, "failedscheduling"):
		return models.ClassificationResult{
			IssueCategory:              "scheduling_failure",
			Severity:                   "high",
			SeverityScore:              0.75,
			ImmediateActionProbability: 0.75,
			RecommendedAction:          "scale_cluster_nodes",
			ClassificationSource:       "rule_fallback",
		}
	case strings.Contains(reason, "pressure") || strings.Contains(reason, "notready") || strings.Contains(reason, "evicted"):
		return models.ClassificationResult{
			IssueCategory:              "node_pressure",
			Severity:                   "critical",
			SeverityScore:              0.95,
			ImmediateActionProbability: 0.95,
			RecommendedAction:          "cordon_and_drain_node",
			ClassificationSource:       "rule_fallback",
		}
	default:
		return models.ClassificationResult{
			IssueCategory:              "healthy_or_noise",
			Severity:                   "low",
			SeverityScore:              0.20,
			ImmediateActionProbability: 0.10,
			RecommendedAction:          "no_action_monitor",
			ClassificationSource:       "rule_fallback",
		}
	}
}
