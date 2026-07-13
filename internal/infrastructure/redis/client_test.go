package redis

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
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
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	pingReceived := make(chan struct{})
	releaseServer := make(chan struct{})
	serverErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			serverErr <- err
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		for {
			command, err := readRedisCommand(reader)
			if err != nil {
				serverErr <- err
				return
			}
			switch command {
			case "HELLO":
				if _, err := io.WriteString(conn, "-ERR unknown command 'hello'\r\n"); err != nil {
					serverErr <- err
					return
				}
			case "PING":
				close(pingReceived)
				<-releaseServer
				serverErr <- nil
				return
			default:
				serverErr <- errors.New("unexpected Redis command: " + command)
				return
			}
		}
	}()

	client := redisclient.NewClient(&redisclient.Options{
		Addr:                  listener.Addr().String(),
		ContextTimeoutEnabled: true,
		MaxRetries:            -1,
		DisableIdentity:       true,
	})
	t.Cleanup(func() { _ = client.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	err = NewPingClient(client).Ping(ctx)
	elapsed := time.Since(started)
	close(releaseServer)
	if err == nil {
		t.Fatal("Ping() error = nil, want command I/O timeout")
	}
	if elapsed < 75*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Fatalf("Ping() elapsed = %v, want context deadline to bound command I/O near 100ms", elapsed)
	}
	select {
	case <-pingReceived:
	default:
		t.Fatal("server did not receive PING before the context deadline")
	}
	if err := <-serverErr; err != nil {
		t.Fatalf("Redis test server error = %v", err)
	}
}

func readRedisCommand(reader *bufio.Reader) (string, error) {
	header, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	header = string(bytes.TrimSuffix([]byte(header), []byte("\r\n")))
	if len(header) < 2 || header[0] != '*' {
		return "", errors.New("invalid Redis array header")
	}
	argumentCount, err := strconv.Atoi(header[1:])
	if err != nil {
		return "", fmt.Errorf("parse Redis array header %q: %w", header, err)
	}
	var command string
	for argument := 0; argument < argumentCount; argument++ {
		lengthLine, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		lengthLine = string(bytes.TrimSuffix([]byte(lengthLine), []byte("\r\n")))
		if len(lengthLine) < 2 || lengthLine[0] != '$' {
			return "", fmt.Errorf("invalid Redis bulk string length %q", lengthLine)
		}
		length, err := strconv.Atoi(lengthLine[1:])
		if err != nil {
			return "", fmt.Errorf("parse Redis bulk string length %q: %w", lengthLine, err)
		}
		value := make([]byte, length+2)
		if _, err := io.ReadFull(reader, value); err != nil {
			return "", err
		}
		if argument == 0 {
			command = string(bytes.ToUpper(value[:length]))
		}
	}
	return command, nil
}

func TestNewPingClientWrapsClient(t *testing.T) {
	client := New("redis.example:6380")
	t.Cleanup(func() { _ = client.Close() })

	pingClient := NewPingClient(client)
	if pingClient.client != client {
		t.Fatal("NewPingClient did not retain the supplied client")
	}
}
