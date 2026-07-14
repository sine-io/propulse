package config

import (
	"errors"
	"testing"
)

func setValidCapacityPolicyEnv(t *testing.T) {
	t.Helper()
	t.Setenv("PROPULSE_CAPACITY_POLICY_CITY", "测试市")
	t.Setenv("PROPULSE_CAPACITY_POLICY_NAME", "测试首付政策")
	t.Setenv("PROPULSE_CAPACITY_POLICY_DOWN_PAYMENT_RATE", "0.35")
	t.Setenv("PROPULSE_CAPACITY_POLICY_EFFECTIVE_DATE", "2026-07-14")
	t.Setenv("PROPULSE_CAPACITY_POLICY_SOURCE", "测试政策来源")
}

func TestLoadUsesDocumentedDefaults(t *testing.T) {
	setValidCapacityPolicyEnv(t)
	t.Setenv("PROPULSE_HTTP_ADDR", "")
	t.Setenv("PROPULSE_DATABASE_URL", "")
	t.Setenv("PROPULSE_REDIS_ADDR", "")
	t.Setenv("PROPULSE_ACCESS_TOKEN", "")
	t.Setenv("PROPULSE_ADMIN_API_TOKEN", "")
	t.Setenv("PROPULSE_LOG_LEVEL", "")
	t.Setenv("PROPULSE_LOG_PRETTY", "")
	t.Setenv("PROPULSE_SCHEDULER_INTERVAL", "")
	t.Setenv("PROPULSE_USER_ID", "propulse-user")

	cfg, err := Load("serve")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.UserID != "propulse-user" {
		t.Fatalf("UserID = %q, want propulse-user", cfg.UserID)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL == "" {
		t.Fatal("DatabaseURL must have a local postgres default")
	}
	if cfg.RedisAddr != "127.0.0.1:6379" {
		t.Fatalf("RedisAddr = %q, want 127.0.0.1:6379", cfg.RedisAddr)
	}
	if cfg.AccessToken != "" {
		t.Fatalf("AccessToken = %q, want empty default", cfg.AccessToken)
	}
	if cfg.Log.Level != "info" {
		t.Fatalf("Log.Level = %q, want info", cfg.Log.Level)
	}
	if cfg.Log.Pretty {
		t.Fatal("Log.Pretty must default to false")
	}
	if cfg.SchedulerInterval.String() != "1h0m0s" {
		t.Fatalf("SchedulerInterval = %s, want 1h0m0s", cfg.SchedulerInterval)
	}
	if cfg.MetricAlgorithmVersion != "market-metrics/2026.07.14.1" {
		t.Fatalf("MetricAlgorithmVersion = %q", cfg.MetricAlgorithmVersion)
	}
	if cfg.AlternativeComparisonRuleVersion != "alternative-comparison/2026.07.14.1" {
		t.Fatalf("AlternativeComparisonRuleVersion = %q", cfg.AlternativeComparisonRuleVersion)
	}
}

func TestLoadReadsAccessToken(t *testing.T) {
	setValidCapacityPolicyEnv(t)
	t.Setenv("PROPULSE_USER_ID", "propulse-user")
	t.Setenv("PROPULSE_ACCESS_TOKEN", "secret-token")
	t.Setenv("PROPULSE_ADMIN_API_TOKEN", "legacy-token")

	cfg, err := Load("api")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AccessToken != "secret-token" {
		t.Fatalf("AccessToken = %q, want secret-token", cfg.AccessToken)
	}
}

func TestLoadDoesNotAcceptLegacyAdminToken(t *testing.T) {
	setValidCapacityPolicyEnv(t)
	t.Setenv("PROPULSE_USER_ID", "propulse-user")
	t.Setenv("PROPULSE_ACCESS_TOKEN", "")
	t.Setenv("PROPULSE_ADMIN_API_TOKEN", "legacy-token")

	cfg, err := Load("worker")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AccessToken != "" {
		t.Fatalf("AccessToken = %q, want empty", cfg.AccessToken)
	}
}

func TestLoadParsesSchedulerInterval(t *testing.T) {
	setValidCapacityPolicyEnv(t)
	t.Setenv("PROPULSE_USER_ID", "propulse-user")
	t.Setenv("PROPULSE_SCHEDULER_INTERVAL", "10s")

	cfg, err := Load("scheduler")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SchedulerInterval.String() != "10s" {
		t.Fatalf("SchedulerInterval = %s, want 10s", cfg.SchedulerInterval)
	}
}

func TestLoadRejectsInvalidSchedulerInterval(t *testing.T) {
	setValidCapacityPolicyEnv(t)
	t.Setenv("PROPULSE_USER_ID", "propulse-user")
	t.Setenv("PROPULSE_SCHEDULER_INTERVAL", "sometimes")

	_, err := Load("scheduler")
	if err == nil {
		t.Fatal("Load() error = nil, want invalid interval error")
	}
}

func TestLoadRequiresUserIDForRuntimeModes(t *testing.T) {
	for _, mode := range []string{"serve", "api", "worker", "scheduler"} {
		t.Run(mode, func(t *testing.T) {
			setValidCapacityPolicyEnv(t)
			t.Setenv("PROPULSE_USER_ID", "")

			_, err := Load(mode)
			if !errors.Is(err, ErrMissingUserID) {
				t.Fatalf("Load(%q) error = %v, want ErrMissingUserID", mode, err)
			}
		})
	}
}

func TestLoadMigrationsOnlyRequireDatabaseConfiguration(t *testing.T) {
	for _, mode := range []string{"migrate up", "migrate down"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("PROPULSE_DATABASE_URL", "postgres://migration-db")
			t.Setenv("PROPULSE_USER_ID", "")
			t.Setenv("PROPULSE_SCHEDULER_INTERVAL", "not-a-duration")
			t.Setenv("PROPULSE_CAPACITY_POLICY_CITY", "")
			t.Setenv("PROPULSE_CAPACITY_POLICY_DOWN_PAYMENT_RATE", "not-a-number")

			cfg, err := Load(mode)
			if err != nil {
				t.Fatalf("Load(%q) error = %v", mode, err)
			}
			if cfg.DatabaseURL != "postgres://migration-db" {
				t.Fatalf("DatabaseURL = %q, want postgres://migration-db", cfg.DatabaseURL)
			}
			if cfg.Mode != mode {
				t.Fatalf("Mode = %q, want %q", cfg.Mode, mode)
			}
			if cfg.UserID != "" {
				t.Fatalf("UserID = %q, want empty for migration mode", cfg.UserID)
			}
		})
	}
}

func TestLoadMapsCapacityAssumptions(t *testing.T) {
	setValidCapacityPolicyEnv(t)
	t.Setenv("PROPULSE_USER_ID", "propulse-user")

	cfg, err := Load("api")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	assumptions := cfg.CapacityAssumptions
	if assumptions.RuleVersion != "2026.07.14" || assumptions.EffectiveDate != "2026-07-14" {
		t.Fatalf("rule metadata = %q/%q", assumptions.RuleVersion, assumptions.EffectiveDate)
	}
	if assumptions.CityPolicy.City != "测试市" || assumptions.CityPolicy.PolicyName != "测试首付政策" ||
		assumptions.CityPolicy.DownPaymentRate != 0.35 || assumptions.CityPolicy.Source != "测试政策来源" {
		t.Fatalf("CityPolicy = %#v", assumptions.CityPolicy)
	}
	if assumptions.Loan.AnnualInterestRate != 0.039 || assumptions.Loan.LoanTermMonths != 360 ||
		assumptions.Loan.RepaymentMethod != "equal_installment" {
		t.Fatalf("Loan = %#v", assumptions.Loan)
	}
}

func TestLoadRequiresEveryCapacityPolicySetting(t *testing.T) {
	keys := []string{
		"PROPULSE_CAPACITY_POLICY_CITY",
		"PROPULSE_CAPACITY_POLICY_NAME",
		"PROPULSE_CAPACITY_POLICY_DOWN_PAYMENT_RATE",
		"PROPULSE_CAPACITY_POLICY_EFFECTIVE_DATE",
		"PROPULSE_CAPACITY_POLICY_SOURCE",
	}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			setValidCapacityPolicyEnv(t)
			t.Setenv("PROPULSE_USER_ID", "propulse-user")
			t.Setenv(key, " ")

			_, err := Load("serve")
			if !errors.Is(err, ErrMissingCapacityPolicy) {
				t.Fatalf("Load() error = %v, want ErrMissingCapacityPolicy", err)
			}
		})
	}
}

func TestLoadRejectsInvalidCapacityPolicy(t *testing.T) {
	tests := map[string]struct {
		key   string
		value string
	}{
		"non numeric rate": {key: "PROPULSE_CAPACITY_POLICY_DOWN_PAYMENT_RATE", value: "many"},
		"zero rate":        {key: "PROPULSE_CAPACITY_POLICY_DOWN_PAYMENT_RATE", value: "0"},
		"rate one":         {key: "PROPULSE_CAPACITY_POLICY_DOWN_PAYMENT_RATE", value: "1"},
		"negative rate":    {key: "PROPULSE_CAPACITY_POLICY_DOWN_PAYMENT_RATE", value: "-0.1"},
		"invalid date":     {key: "PROPULSE_CAPACITY_POLICY_EFFECTIVE_DATE", value: "2026/07/14"},
		"future date":      {key: "PROPULSE_CAPACITY_POLICY_EFFECTIVE_DATE", value: "2999-01-01"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			setValidCapacityPolicyEnv(t)
			t.Setenv("PROPULSE_USER_ID", "propulse-user")
			t.Setenv(test.key, test.value)

			_, err := Load("api")
			if !errors.Is(err, ErrInvalidCapacityPolicy) {
				t.Fatalf("Load() error = %v, want ErrInvalidCapacityPolicy", err)
			}
		})
	}
}
