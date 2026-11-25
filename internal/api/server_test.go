package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kavix/k8s-advisor/internal/advisor"
	"github.com/kavix/k8s-advisor/internal/api"
	"github.com/kavix/k8s-advisor/internal/classifier"
	"github.com/kavix/k8s-advisor/internal/models"
)

type memStore struct {
	advisories []models.Advisory
}

func (m *memStore) HealthCheck(context.Context) error { return nil }
func (m *memStore) InsertMetrics(context.Context, []models.K8sMetricPoint) (int64, error) {
	return 0, nil
}
func (m *memStore) InsertEvents(context.Context, []models.K8sEventRecord) (int, error) {
	return 0, nil
}
func (m *memStore) GetHistoricalBaseline(context.Context, string, string, string) (models.HistoricalBaseline, error) {
	return models.HistoricalBaseline{}, nil
}
func (m *memStore) SaveAdvisory(_ context.Context, a models.Advisory) error {
	m.advisories = append(m.advisories, a)
	return nil
}
func (m *memStore) ListAdvisories(context.Context, int) ([]models.Advisory, error) {
	return m.advisories, nil
}

func TestHealthzAndAnalyzeEndpoint(t *testing.T) {
	store := &memStore{}
	engine := advisor.NewEngine(store, classifier.NewJevClassifier("", "", false))
	srv := api.NewServer(store, engine)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /healthz, got %d", rec.Code)
	}

	body, _ := json.Marshal(models.K8sEventRecord{
		ClusterName:  "dev",
		Namespace:    "default",
		InvolvedKind: "Pod",
		InvolvedName: "worker-1",
		EventType:    "Warning",
		Reason:       "FailedScheduling",
		Message:      "0/3 nodes are available: Insufficient memory.",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/analyze", bytes.NewReader(body))
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/analyze, got %d", rec2.Code)
	}

	var adv models.Advisory
	if err := json.Unmarshal(rec2.Body.Bytes(), &adv); err != nil {
		t.Fatalf("failed to decode advisory: %v", err)
	}
	if adv.Classification.IssueCategory != "scheduling_failure" {
		t.Fatalf("expected scheduling_failure, got %s", adv.Classification.IssueCategory)
	}
}
