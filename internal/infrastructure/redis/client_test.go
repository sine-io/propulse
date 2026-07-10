package redis

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	redisclient "github.com/redis/go-redis/v9"
)

func TestNewUsesConfiguredAddress(t *testing.T) {
	client := New("redis.example:6380")
	t.Cleanup(func() { _ = client.Close() })

	if got := client.Options().Addr; got != "redis.example:6380" {
		t.Fatalf("client address = %q, want redis.example:6380", got)
	}
}

func TestNewEnablesContextTimeouts(t *testing.T) {
	client := New("redis.example:6380")
	t.Cleanup(func() { _ = client.Close() })

	if !client.Options().ContextTimeoutEnabled {
		t.Fatal("ContextTimeoutEnabled = false, want true")
	}
}

func TestPingClientHonorsContextDeadline(t *testing.T) {
	client := redisclient.NewClient(&redisclient.Options{
		Addr:                  "redis.example:6380",
		ContextTimeoutEnabled: true,
		MaxRetries:            -1,
		Dialer: func(ctx context.Context, _, _ string) (net.Conn, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := NewPingClient(client).Ping(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Ping() error = %v, want context deadline exceeded", err)
	}
}

func TestNewPingClientWrapsClient(t *testing.T) {
	client := New("redis.example:6380")
	t.Cleanup(func() { _ = client.Close() })

	pingClient := NewPingClient(client)
	if pingClient.client != client {
		t.Fatal("NewPingClient did not retain the supplied client")
	}
}
