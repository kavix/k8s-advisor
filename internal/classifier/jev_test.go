package classifier_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/kavix/k8s-advisor/internal/classifier"
	"github.com/kavix/k8s-advisor/internal/models"
)

func TestRuleFallbackClassifiesOOM(t *testing.T) {
	cls := classifier.NewJevClassifier("", "https://api.typesafe.ai", false)
	event := models.K8sEventRecord{
		ClusterName:  "prod",
		Namespace:    "payments",
		InvolvedKind: "Pod",
		InvolvedName: "payment-api-123",
		EventType:    "Warning",
		Reason:       "OOMKilling",
		Message:      "Memory cgroup out of memory: Killed process",
	}

	res := cls.Classify(context.Background(), event, models.HistoricalBaseline{})
	if res.IssueCategory != "oom_memory" {
		t.Fatalf("expected oom_memory, got %s", res.IssueCategory)
	}
	if res.RecommendedAction != "increase_memory_limits" {
		t.Fatalf("expected increase_memory_limits, got %s", res.RecommendedAction)
	}
	if res.ClassificationSource != "rule_fallback" {
		t.Fatalf("expected rule_fallback, got %s", res.ClassificationSource)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestJevClassifierUsesMockedSystemOneAPI(t *testing.T) {
	httpClient := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/system_one" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			body, _ := json.Marshal(map[string]any{
				"answers": map[string]any{
					"issue_category":     map[string]any{"choice": "crash_loop"},
					"severity":           map[string]any{"level": "high", "score": 0.88},
					"immediate_action":   map[string]any{"noul": 0.92},
					"recommended_action": map[string]any{"choice": "inspect_logs_and_probes"},
				},
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	cls := classifier.NewJevClassifierWithClient("test-key", "https://api.typesafe.ai", true, httpClient)
	res := cls.Classify(context.Background(), models.K8sEventRecord{
		ClusterName:  "prod",
		Namespace:    "default",
		InvolvedKind: "Pod",
		InvolvedName: "web-0",
		EventType:    "Warning",
		Reason:       "BackOff",
		Message:      "Back-off restarting failed container",
	}, models.HistoricalBaseline{MaxRestarts24h: 8})

	if res.ClassificationSource != "jev" {
		t.Fatalf("expected jev source, got %s", res.ClassificationSource)
	}
	if res.IssueCategory != "crash_loop" || res.Severity != "high" {
		t.Fatalf("unexpected result: %+v", res)
	}
}
