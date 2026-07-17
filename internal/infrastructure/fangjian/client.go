package fangjian

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const maxResponseBytes = 16 * 1024 * 1024

type ClientConfig struct {
	BaseURL       string
	Authorization string
	AK            string
	Version       string
	MinInterval   time.Duration
	MaxAttempts   int
}

type Client struct {
	baseURL       string
	authorization string
	ak            string
	version       string
	httpClient    *http.Client
	minInterval   time.Duration
	maxAttempts   int
	mu            sync.Mutex
	lastRequest   time.Time
}

func NewClient(config ClientConfig, httpClient *http.Client) (*Client, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("Fangjian base URL is invalid")
	}
	if strings.TrimSpace(config.Authorization) == "" || strings.TrimSpace(config.AK) == "" || strings.TrimSpace(config.Version) == "" {
		return nil, errors.New("Fangjian authorization, ak, and version are required")
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 25 * time.Second}
	}
	if config.MinInterval <= 0 {
		config.MinInterval = 150 * time.Millisecond
	}
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	return &Client{
		baseURL: baseURL, authorization: strings.TrimSpace(config.Authorization),
		ak: strings.TrimSpace(config.AK), version: strings.TrimSpace(config.Version),
		httpClient: httpClient, minInterval: config.MinInterval, maxAttempts: config.MaxAttempts,
	}, nil
}

func (c *Client) Get(ctx context.Context, path string) (json.RawMessage, error) {
	return c.request(ctx, http.MethodGet, path, nil)
}

func (c *Client) Post(ctx context.Context, path string, input any) (json.RawMessage, error) {
	body, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	return c.request(ctx, http.MethodPost, path, body)
}

func (c *Client) request(ctx context.Context, method, path string, body []byte) (json.RawMessage, error) {
	if !strings.HasPrefix(path, "/") {
		return nil, errors.New("Fangjian request path must be absolute")
	}
	var lastErr error
	for attempt := 1; attempt <= c.maxAttempts; attempt++ {
		if err := c.waitRateLimit(ctx); err != nil {
			return nil, err
		}
		request, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Authorization", c.authorization)
		request.Header.Set("ak", c.ak)
		request.Header.Set("Version", c.version)
		request.Header.Set("Accept", "application/json")
		request.Header.Set("Referer", "https://servicewechat.com/wx06cb5b3a684f53de/0/page-frame.html")
		request.Header.Set("User-Agent", "Mozilla/5.0 MicroMessenger")
		if method == http.MethodPost {
			request.Header.Set("Content-Type", "application/json")
		}
		response, err := c.httpClient.Do(request)
		if err != nil {
			lastErr = err
			if !retryableNetworkError(err) || attempt == c.maxAttempts {
				return nil, fmt.Errorf("Fangjian request failed: %w", err)
			}
			if err := waitRetry(ctx, attempt); err != nil {
				return nil, err
			}
			continue
		}
		responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
		closeErr := response.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("Fangjian response read failed: %w", readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("Fangjian response close failed: %w", closeErr)
		}
		if len(responseBody) > maxResponseBytes {
			return nil, errors.New("Fangjian response exceeds 16 MiB")
		}
		if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
			return nil, fmt.Errorf("Fangjian authentication failed with HTTP %d", response.StatusCode)
		}
		if response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500 {
			lastErr = fmt.Errorf("Fangjian temporary HTTP status %d", response.StatusCode)
			if attempt < c.maxAttempts {
				if err := waitRetry(ctx, attempt); err != nil {
					return nil, err
				}
				continue
			}
			return nil, lastErr
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return nil, fmt.Errorf("Fangjian HTTP status %d", response.StatusCode)
		}
		if !json.Valid(responseBody) {
			return nil, errors.New("Fangjian response is not valid JSON")
		}
		return append(json.RawMessage(nil), responseBody...), nil
	}
	return nil, lastErr
}

func (c *Client) waitRateLimit(ctx context.Context) error {
	c.mu.Lock()
	wait := c.minInterval - time.Since(c.lastRequest)
	if wait < 0 {
		wait = 0
	}
	c.lastRequest = time.Now().Add(wait)
	c.mu.Unlock()
	if wait == 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryableNetworkError(err error) bool {
	var networkErr net.Error
	return errors.As(err, &networkErr) && (networkErr.Timeout() || networkErr.Temporary())
}

func waitRetry(ctx context.Context, attempt int) error {
	delay := time.Duration(attempt*attempt) * 250 * time.Millisecond
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
