package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hibiken/asynq"
	appqueue "github.com/propulse/propulse/backend/internal/application/queue"
	"github.com/rs/zerolog"
)

var ErrInvalidTaskPayload = errors.New("invalid_task_payload")

type MetricCalculator interface {
	CalculateNeighborhood(ctx context.Context, neighborhoodID string) error
}

type Handlers struct {
	metric MetricCalculator
	log    zerolog.Logger
}

var taskIDFromContext = asynq.GetTaskID

func NewHandlers(metric MetricCalculator, log zerolog.Logger) *Handlers {
	return &Handlers{metric: metric, log: log}
}

func (h *Handlers) ProcessTask(ctx context.Context, task *asynq.Task) error {
	switch task.Type() {
	case appqueue.TypeMetricCalculateNeighborhood:
		return h.processMetricCalculateNeighborhood(ctx, task)
	default:
		return fmt.Errorf("unsupported task type %q", task.Type())
	}
}

func (h *Handlers) processMetricCalculateNeighborhood(ctx context.Context, task *asynq.Task) error {
	var payload appqueue.MetricCalculateNeighborhoodPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTaskPayload, err)
	}
	if payload.NeighborhoodID == "" {
		return fmt.Errorf("%w: neighborhoodId is required", ErrInvalidTaskPayload)
	}

	event := h.log.Info().
		Str("job_id", taskID(ctx)).
		Str("task_type", task.Type()).
		Str("source_id", payload.SourceID).
		Str("neighborhood_id", payload.NeighborhoodID)

	if err := h.metric.CalculateNeighborhood(ctx, payload.NeighborhoodID); err != nil {
		event.Err(err).Msg("metric task failed")
		return err
	}

	event.Msg("metric task completed")
	return nil
}

func taskID(ctx context.Context) string {
	id, ok := taskIDFromContext(ctx)
	if !ok {
		return ""
	}
	return id
}
