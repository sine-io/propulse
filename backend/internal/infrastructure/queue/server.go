package queue

import (
	"context"
	"errors"

	"github.com/hibiken/asynq"
	appqueue "github.com/sine-io/propulse/backend/internal/application/queue"
	"github.com/rs/zerolog"
)

type Server struct {
	server *asynq.Server
	mux    *asynq.ServeMux
}

func NewServer(redisAddr string, metric MetricCalculator, log zerolog.Logger) *Server {
	server := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Queues: map[string]int{
				appqueue.QueueCritical: 6,
				appqueue.QueueDefault:  3,
				appqueue.QueueLow:      1,
			},
		},
	)

	mux := asynq.NewServeMux()
	handlers := NewHandlers(metric, log)
	mux.HandleFunc(appqueue.TypeMetricCalculateNeighborhood, handlers.ProcessTask)

	return &Server{
		server: server,
		mux:    mux,
	}
}

func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.server.Run(s.mux)
	}()

	select {
	case <-ctx.Done():
		s.server.Shutdown()
		err := <-errCh
		if err == nil || errors.Is(err, ctx.Err()) {
			return nil
		}
		return err
	case err := <-errCh:
		return err
	}
}
