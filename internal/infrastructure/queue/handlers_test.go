package queue

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	appmetric "github.com/sine-io/propulse/internal/application/metric"
	appqueue "github.com/sine-io/propulse/internal/application/queue"
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
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("ProcessTask() error = %v, want skip retry", err)
	}
	if !strings.Contains(err.Error(), "invalid_task_payload") {
		t.Fatalf("ProcessTask() error = %q, want code invalid_task_payload", err.Error())
	}
	if service.called {
		t.Fatal("metric service was called for invalid payload")
	}
}

func TestMetricCalculateNeighborhoodHandlerRequiresRunAndSourceProvenance(t *testing.T) {
	for _, test := range []struct {
		name    string
		payload appqueue.MetricCalculateNeighborhoodPayload
	}{
		{
			name:    "collection run",
			payload: appqueue.MetricCalculateNeighborhoodPayload{NeighborhoodID: "neighborhood_1", SourceID: "scheduler.metric_repair"},
		},
		{
			name:    "source",
			payload: appqueue.MetricCalculateNeighborhoodPayload{NeighborhoodID: "neighborhood_1", CollectionRunID: "run_1"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := &stubMetricService{}
			handler := NewHandlers(service, zerolog.Nop())

			err := handler.ProcessTask(context.Background(), NewMetricCalculateNeighborhoodTask(test.payload))

			if !errors.Is(err, ErrInvalidTaskPayload) || !errors.Is(err, asynq.SkipRetry) {
				t.Fatalf("ProcessTask() error = %v, want invalid payload with skip retry", err)
			}
			if service.called {
				t.Fatal("metric service was called for invalid payload")
			}
		})
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
		NeighborhoodID:  "neighborhood_1",
		CollectionRunID: "run_1",
		SourceID:        "import.retry",
	})

	err := handler.ProcessTask(context.Background(), task)

	if err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if service.command.NeighborhoodID != "neighborhood_1" || service.command.CollectionRunID != "run_1" {
		t.Fatalf("service command = %#v", service.command)
	}

	var entry map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(logs.Bytes()), &entry); err != nil {
		t.Fatalf("log entry is not JSON: %v; raw=%q", err, logs.String())
	}
	if entry["task_type"] != appqueue.TypeMetricCalculateNeighborhood {
		t.Fatalf("log task_type = %v, want %s", entry["task_type"], appqueue.TypeMetricCalculateNeighborhood)
	}
	if entry["source_id"] != "import.retry" || entry["collection_run_id"] != "run_1" {
		t.Fatalf("log provenance = source %v run %v", entry["source_id"], entry["collection_run_id"])
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
	called  bool
	command appmetric.CalculateCollectionRunCommand
}

func (s *stubMetricService) CalculateCollectionRun(_ context.Context, command appmetric.CalculateCollectionRunCommand) error {
	s.called = true
	s.command = command
	return nil
}
