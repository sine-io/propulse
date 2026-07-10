package redis

import "testing"

func TestNewUsesConfiguredAddress(t *testing.T) {
	client := New("redis.example:6380")
	t.Cleanup(func() { _ = client.Close() })

	if got := client.Options().Addr; got != "redis.example:6380" {
		t.Fatalf("client address = %q, want redis.example:6380", got)
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
