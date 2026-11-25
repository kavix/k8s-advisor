package advisor_test

import (
	"context"
	"testing"

	"github.com/kavix/k8s-advisor/internal/advisor"
	"github.com/kavix/k8s-advisor/internal/classifier"
	"github.com/kavix/k8s-advisor/internal/models"
)

type fakeStore struct {
	baseline   models.HistoricalBaseline
	advisories []models.Advisory
}

func (f *fakeStore) HealthCheck(context.Context) error { return nil }
func (f *fakeStore) InsertMetrics(context.Context, []models.K8sMetricPoint) (int64, error) {
	return 0, nil
}
func (f *fakeStore) InsertEvents(context.Context, []models.K8sEventRecord) (int, error) {
	return 0, nil
}
func (f *fakeStore) GetHistoricalBaseline(context.Context, string, string, string) (models.HistoricalBaseline, error) {
	return f.baseline, nil
}
func (f *fakeStore) SaveAdvisory(_ context.Context, a models.Advisory) error {
	f.advisories = append(f.advisories, a)
	return nil
}
func (f *fakeStore) ListAdvisories(context.Context, int) ([]models.Advisory, error) {
	return f.advisories, nil
}

func TestAnalyzeEventCorrelatesBaselineAndSavesAdvisory(t *testing.T) {
	store := &fakeStore{
		baseline: models.HistoricalBaseline{
			WarningEvents1h: 4,
			WarningEvents7d: 12,
			MaxRestarts24h:  7,
		},
	}
	cls := classifier.NewJevClassifier("", "", false)
	engine := advisor.NewEngine(store, cls)

	adv, err := engine.AnalyzeEvent(context.Background(), models.K8sEventRecord{
		ClusterName:  "gke-prod",
		Namespace:    "orders",
		InvolvedKind: "Pod",
		InvolvedName: "order-svc-abc",
		EventType:    "Warning",
		Reason:       "BackOff",
		Message:      "Back-off restarting failed container",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adv.Classification.IssueCategory != "crash_loop" {
		t.Fatalf("expected crash_loop, got %s", adv.Classification.IssueCategory)
	}
	if adv.Classification.Severity != "high" {
		t.Fatalf("expected high severity due to warning_events_1h=4, got %s", adv.Classification.Severity)
	}
	if len(store.advisories) != 1 {
		t.Fatalf("expected 1 saved advisory, got %d", len(store.advisories))
	}
}
