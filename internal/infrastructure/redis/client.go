package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func New(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{Addr: addr})
}

type PingClient struct {
	client *redis.Client
}

func NewPingClient(client *redis.Client) PingClient {
	return PingClient{client: client}
}

func (p PingClient) Ping(ctx context.Context) error {
	return p.client.Ping(ctx).Err()
}
