package app

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestReadinessCheckerReturnsNilWhenDependenciesAreReady(t *testing.T) {
	calls := []string{}
	checker := NewReadinessChecker(
		&sqlPingerStub{ping: recordReadinessPing(&calls, "sql", nil)},
		&pgxPingerStub{ping: recordReadinessPing(&calls, "pgx", nil)},
		&redisPingerStub{ping: recordReadinessPing(&calls, "redis", nil)},
		"access-token",
	)

	if err := checker.Check(context.Background()); err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
	assertReadinessCalls(t, calls, []string{"sql", "pgx", "redis"})
}

func TestReadinessCheckerRejectsWhitespaceAccessTokenBeforePingingDependencies(t *testing.T) {
	calls := []string{}
	checker := NewReadinessChecker(
		&sqlPingerStub{ping: recordReadinessPing(&calls, "sql", nil)},
		&pgxPingerStub{ping: recordReadinessPing(&calls, "pgx", nil)},
		&redisPingerStub{ping: recordReadinessPing(&calls, "redis", nil)},
		" \t\n ",
	)

	err := checker.Check(context.Background())
	if err == nil || err.Error() != "access token is not configured" {
		t.Fatalf("Check() error = %v, want access token configuration error", err)
	}
	assertReadinessCalls(t, calls, nil)
}

func TestReadinessCheckerRejectsEmptyAccessToken(t *testing.T) {
	checker := NewReadinessChecker(
		&sqlPingerStub{ping: func(context.Context) error { return nil }},
		&pgxPingerStub{ping: func(context.Context) error { return nil }},
		&redisPingerStub{ping: func(context.Context) error { return nil }},
		"",
	)

	if err := checker.Check(context.Background()); err == nil || err.Error() != "access token is not configured" {
		t.Fatalf("Check() error = %v, want access token configuration error", err)
	}
}

func TestReadinessCheckerWrapsDependencyErrorsAndShortCircuits(t *testing.T) {
	tests := []struct {
		name      string
		failure   string
		wantCalls []string
		wantText  string
	}{
		{name: "database sql", failure: "sql", wantCalls: []string{"sql"}, wantText: "sql ping"},
		{name: "pgx", failure: "pgx", wantCalls: []string{"sql", "pgx"}, wantText: "pgx ping"},
		{name: "redis", failure: "redis", wantCalls: []string{"sql", "pgx", "redis"}, wantText: "redis ping"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dependencyErr := errors.New(tt.failure + " unavailable")
			calls := []string{}
			checker := NewReadinessChecker(
				&sqlPingerStub{ping: recordReadinessPing(&calls, "sql", readinessFailure(tt.failure, "sql", dependencyErr))},
				&pgxPingerStub{ping: recordReadinessPing(&calls, "pgx", readinessFailure(tt.failure, "pgx", dependencyErr))},
				&redisPingerStub{ping: recordReadinessPing(&calls, "redis", readinessFailure(tt.failure, "redis", dependencyErr))},
				"access-token",
			)

			err := checker.Check(context.Background())
			if !errors.Is(err, dependencyErr) {
				t.Fatalf("errors.Is(%v, dependencyErr) = false", err)
			}
			if got, want := err.Error(), fmt.Sprintf("%s: %s unavailable", tt.wantText, tt.failure); got != want {
				t.Fatalf("Check() error = %q, want %q", got, want)
			}
			assertReadinessCalls(t, calls, tt.wantCalls)
		})
	}
}

func readinessFailure(failure, dependency string, err error) error {
	if failure == dependency {
		return err
	}
	return nil
}

func recordReadinessPing(calls *[]string, name string, err error) func(context.Context) error {
	return func(context.Context) error {
		*calls = append(*calls, name)
		return err
	}
}

func assertReadinessCalls(t *testing.T, got, want []string) {
	t.Helper()
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("ping calls = %v, want %v", got, want)
	}
}

type sqlPingerStub struct {
	ping func(context.Context) error
}

func (s *sqlPingerStub) PingContext(ctx context.Context) error {
	return s.ping(ctx)
}

type pgxPingerStub struct {
	ping func(context.Context) error
}

func (s *pgxPingerStub) Ping(ctx context.Context) error {
	return s.ping(ctx)
}

type redisPingerStub struct {
	ping func(context.Context) error
}

func (s *redisPingerStub) Ping(ctx context.Context) error {
	return s.ping(ctx)
}
