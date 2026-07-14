package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
)

const (
	defaultHTTPAddr          = ":8080"
	defaultDatabaseURL       = "postgres://propulse:propulse@127.0.0.1:5432/propulse?sslmode=disable"
	defaultRedisAddr         = "127.0.0.1:6379"
	defaultLogLevel          = "info"
	defaultSchedulerInterval = time.Hour
	capacityRuleVersion      = "2026.07.14"
	capacityEffectiveDate    = "2026-07-14"
	capacityRuleSource       = "propulse capacity rule set"
	capacityLoanSource       = "propulse configured loan defaults"
)

type Config struct {
	HTTPAddr            string
	DatabaseURL         string
	RedisAddr           string
	AccessToken         string
	UserID              string
	Mode                string
	SchedulerInterval   time.Duration
	CapacityAssumptions domaincapacity.Assumptions
	Log                 LogConfig
}

type LogConfig struct {
	Level  string
	Pretty bool
}

var (
	ErrMissingUserID         = errors.New("PROPULSE_USER_ID is required")
	ErrMissingCapacityPolicy = errors.New("capacity policy configuration is required")
	ErrInvalidCapacityPolicy = errors.New("capacity policy configuration is invalid")
)

func Load(mode string) (Config, error) {
	cfg := Config{
		HTTPAddr:          getEnv("PROPULSE_HTTP_ADDR", defaultHTTPAddr),
		DatabaseURL:       getEnv("PROPULSE_DATABASE_URL", defaultDatabaseURL),
		RedisAddr:         getEnv("PROPULSE_REDIS_ADDR", defaultRedisAddr),
		AccessToken:       getEnv("PROPULSE_ACCESS_TOKEN", ""),
		Mode:              mode,
		SchedulerInterval: defaultSchedulerInterval,
		Log: LogConfig{
			Level:  getEnv("PROPULSE_LOG_LEVEL", defaultLogLevel),
			Pretty: getEnv("PROPULSE_LOG_PRETTY", "") == "true",
		},
	}

	// Migration commands only need the database connection. Runtime-only
	// configuration must not prevent schema administration or CI setup.
	if mode == "migrate up" || mode == "migrate down" {
		return cfg, nil
	}

	schedulerInterval, err := parseDurationEnv("PROPULSE_SCHEDULER_INTERVAL", defaultSchedulerInterval)
	if err != nil {
		return Config{}, err
	}
	cfg.SchedulerInterval = schedulerInterval

	// Runtime modes require an explicit stable identity and never fall back to a
	// shared account (#36 / SYS-001.1).
	cfg.UserID = getEnv("PROPULSE_USER_ID", "")
	if cfg.UserID == "" {
		return Config{}, ErrMissingUserID
	}

	assumptions, err := loadCapacityAssumptions(time.Now())
	if err != nil {
		return Config{}, err
	}
	cfg.CapacityAssumptions = assumptions

	return cfg, nil
}

func loadCapacityAssumptions(asOf time.Time) (domaincapacity.Assumptions, error) {
	city, err := requiredEnv("PROPULSE_CAPACITY_POLICY_CITY")
	if err != nil {
		return domaincapacity.Assumptions{}, err
	}
	policyName, err := requiredEnv("PROPULSE_CAPACITY_POLICY_NAME")
	if err != nil {
		return domaincapacity.Assumptions{}, err
	}
	rateText, err := requiredEnv("PROPULSE_CAPACITY_POLICY_DOWN_PAYMENT_RATE")
	if err != nil {
		return domaincapacity.Assumptions{}, err
	}
	effectiveDate, err := requiredEnv("PROPULSE_CAPACITY_POLICY_EFFECTIVE_DATE")
	if err != nil {
		return domaincapacity.Assumptions{}, err
	}
	source, err := requiredEnv("PROPULSE_CAPACITY_POLICY_SOURCE")
	if err != nil {
		return domaincapacity.Assumptions{}, err
	}

	downPaymentRate, err := strconv.ParseFloat(rateText, 64)
	if err != nil {
		return domaincapacity.Assumptions{}, fmt.Errorf("%w: PROPULSE_CAPACITY_POLICY_DOWN_PAYMENT_RATE", ErrInvalidCapacityPolicy)
	}

	assumptions := domaincapacity.Assumptions{
		RuleVersion:   capacityRuleVersion,
		EffectiveDate: capacityEffectiveDate,
		RuleSource:    capacityRuleSource,
		Loan: domaincapacity.LoanParams{
			AnnualInterestRate: 0.039,
			LoanTermMonths:     360,
			RepaymentMethod:    domaincapacity.RepaymentEqualInstallment,
		},
		LoanSource: capacityLoanSource,
		LoanOrigin: domaincapacity.OriginConfiguredDefault,
		CityPolicy: domaincapacity.CityPolicy{
			City:            city,
			PolicyName:      policyName,
			DownPaymentRate: downPaymentRate,
			EffectiveDate:   effectiveDate,
			Source:          source,
			Origin:          domaincapacity.OriginConfiguredDefault,
		},
		ReserveMonths: 6,
		PressureThresholds: domaincapacity.PressureThresholds{
			SafeRatio:        0.35,
			StrainedRatio:    0.45,
			DangerRatio:      0.55,
			DangerMultiplier: 1.15,
		},
		OldHomeShareThreshold: 0.5,
	}
	if err := assumptions.ValidateAt(asOf); err != nil {
		return domaincapacity.Assumptions{}, fmt.Errorf("%w: %w", ErrInvalidCapacityPolicy, err)
	}
	return assumptions, nil
}

func requiredEnv(key string) (string, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return "", fmt.Errorf("%w: %s", ErrMissingCapacityPolicy, key)
	}
	return value, nil
}

func parseDurationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := getEnv(key, "")
	if value == "" {
		return fallback, nil
	}

	return time.ParseDuration(value)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
