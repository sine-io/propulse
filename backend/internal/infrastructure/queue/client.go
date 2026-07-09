package queue

import (
	"context"
	"encoding/json"

	"github.com/hibiken/asynq"
	appqueue "github.com/sine-io/propulse/backend/internal/application/queue"
)

type Client struct {
	client *asynq.Client
}

func NewClient(redisAddr string) *Client {
	return &Client{
		client: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr}),
	}
}

func (c *Client) EnqueueMetricCalculateNeighborhood(ctx context.Context, neighborhoodID string, sourceID string) error {
	task := NewMetricCalculateNeighborhoodTask(appqueue.MetricCalculateNeighborhoodPayload{
		NeighborhoodID: neighborhoodID,
		SourceID:       sourceID,
	})
	_, err := c.client.EnqueueContext(ctx, task, asynq.Queue(appqueue.QueueDefault))
	return err
}

func (c *Client) Close() error {
	return c.client.Close()
}

func NewMetricCalculateNeighborhoodTask(payload appqueue.MetricCalculateNeighborhoodPayload) *asynq.Task {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return asynq.NewTask(appqueue.TypeMetricCalculateNeighborhood, data, asynq.Queue(appqueue.QueueDefault))
}
