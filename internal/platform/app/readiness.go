package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type SQLPinger interface {
	PingContext(context.Context) error
}

type PGXPinger interface {
	Ping(context.Context) error
}

type RedisPinger interface {
	Ping(context.Context) error
}

type readinessChecker struct {
	sql         SQLPinger
	pgx         PGXPinger
	redis       RedisPinger
	accessToken string
}

func NewReadinessChecker(sql SQLPinger, pgx PGXPinger, redis RedisPinger, accessToken string) readinessChecker {
	return readinessChecker{
		sql:         sql,
		pgx:         pgx,
		redis:       redis,
		accessToken: accessToken,
	}
}

func (c readinessChecker) Check(ctx context.Context) error {
	if strings.TrimSpace(c.accessToken) == "" {
		return errors.New("access token is not configured")
	}
	if err := c.sql.PingContext(ctx); err != nil {
		return fmt.Errorf("sql ping: %w", err)
	}
	if err := c.pgx.Ping(ctx); err != nil {
		return fmt.Errorf("pgx ping: %w", err)
	}
	if err := c.redis.Ping(ctx); err != nil {
		return fmt.Errorf("redis ping: %w", err)
	}
	return nil
}
