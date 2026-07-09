package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	appqueue "github.com/sine-io/propulse/backend/internal/application/queue"
	"github.com/rs/zerolog"
)

func TestMetricCalculateNeighborhoodHandlerRejectsEmptyNeighborhoodID(t *testing.T) {
	service := &stubMetricService{}
	var logs bytes.Buffer
	handler := NewHandlers(service, zerolog.New(&logs))
	task := asynq.NewTask(appqueue.TypeMetricCalculateNeighborhood, []byte(`{"sourceId":"scheduler"}`))

	err := handler.ProcessTask(context.Background(), task)

	if !errors.Is(err, ErrInvalidTaskPayload) {
		t.Fatalf("ProcessTask() error = %v, want invalid payload", err)
	}
	if !strings.Contains(err.Error(), "invalid_task_payload") {
		t.Fatalf("ProcessTask() error = %q, want code invalid_task_payload", err.Error())
	}
	if service.called {
		t.Fatal("metric service was called for invalid payload")
	}
}

func TestMetricCalculateNeighborhoodHandlerCallsServiceAndLogsContext(t *testing.T) {
	originalTaskIDFromContext := taskIDFromContext
	defer func() { taskIDFromContext = originalTaskIDFromContext }()
	taskIDFromContext = func(context.Context) (string, bool) {
		return "asynq-task-123", true
	}

	service := &stubMetricService{}
	var logs bytes.Buffer
	handler := NewHandlers(service, zerolog.New(&logs))
	task := NewMetricCalculateNeighborhoodTask(appqueue.MetricCalculateNeighborhoodPayload{
		NeighborhoodID: "neighborhood_1",
		SourceID:       "scheduler",
	})

	err := handler.ProcessTask(context.Background(), task)

	if err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if service.neighborhoodID != "neighborhood_1" {
		t.Fatalf("service neighborhoodID = %q, want neighborhood_1", service.neighborhoodID)
	}

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &entry); err != nil {
		t.Fatalf("log entry is not JSON: %v; raw=%q", err, logs.String())
	}
	if entry["task_type"] != appqueue.TypeMetricCalculateNeighborhood {
		t.Fatalf("log task_type = %v, want %s", entry["task_type"], appqueue.TypeMetricCalculateNeighborhood)
	}
	if entry["source_id"] != "scheduler" {
		t.Fatalf("log source_id = %v, want scheduler", entry["source_id"])
	}
	if entry["job_id"] != "asynq-task-123" {
		t.Fatalf("log job_id = %v, want asynq-task-123", entry["job_id"])
	}
}

func TestTaskTypesDocumentQueueContract(t *testing.T) {
	want := map[string]string{
		"metric":                       appqueue.TypeMetricCalculateNeighborhood,
		"collection fetch source":      appqueue.TypeCollectionFetchSource,
		"collection normalize listing": appqueue.TypeCollectionNormalizeListing,
		"collection deduplicate":       appqueue.TypeCollectionDeduplicateListing,
		"decision refresh":             appqueue.TypeDecisionRefreshWindow,
		"notification weekly report":   appqueue.TypeNotificationGenerateWeeklyReport,
		"notification alert":           appqueue.TypeNotificationSendAlert,
	}

	for name, taskType := range want {
		if taskType == "" {
			t.Fatalf("%s task type is empty", name)
		}
	}
}

type stubMetricService struct {
	called         bool
	neighborhoodID string
}

func (s *stubMetricService) CalculateNeighborhood(_ context.Context, neighborhoodID string) error {
	s.called = true
	s.neighborhoodID = neighborhoodID
	return nil
}
