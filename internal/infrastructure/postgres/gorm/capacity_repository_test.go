package gormrepo

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	migraterunner "github.com/sine-io/propulse/internal/infrastructure/migrate"
)

func repositoryTestAssumptions() domaincapacity.Assumptions {
	return domaincapacity.Assumptions{
		RuleVersion: "2026.07.14", EffectiveDate: "2026-07-14", RuleSource: "test rule source",
		Loan: domaincapacity.LoanParams{
			AnnualInterestRate: 0.039, LoanTermMonths: 360, RepaymentMethod: domaincapacity.RepaymentEqualInstallment,
		},
		LoanSource: "test loan source", LoanOrigin: domaincapacity.OriginConfiguredDefault,
		CityPolicy: domaincapacity.CityPolicy{
			City: "测试市", PolicyName: "测试政策", DownPaymentRate: 0.35,
			EffectiveDate: "2026-07-14", Source: "测试来源", Origin: domaincapacity.OriginConfiguredDefault,
		},
		ReserveMonths: 6,
		PressureThresholds: domaincapacity.PressureThresholds{
			SafeRatio: 0.35, StrainedRatio: 0.45, DangerRatio: 0.55, DangerMultiplier: 1.15,
		},
		OldHomeShareThreshold: 0.5,
	}
}

func repositoryTestInput() domaincapacity.HousingCapacityInput {
	return domaincapacity.HousingCapacityInput{
		CashOnHand: 150, OldHomeValue: 320, OldLoanBalance: 80, MonthlyIncome: 3.5,
		CurrentMonthlyMortgage: 0, AcceptableMonthlyMortgage: 1.5, TargetTotalPrice: 550,
		RenovationBudget: 40, TransactionCosts: 18, TransitionRentCost: 5,
		LoanOverride: &domaincapacity.LoanParams{
			AnnualInterestRate: 0.041, LoanTermMonths: 240, RepaymentMethod: domaincapacity.RepaymentEqualPrincipal,
		},
		CityPolicyOverride: &domaincapacity.CityPolicy{
			City: "覆盖市", PolicyName: "覆盖政策", DownPaymentRate: 0.4,
			EffectiveDate: "2026-07-01", Source: "用户政策来源",
		},
	}
}

func TestCapacityPersistenceRoundTripsCompleteCalculation(t *testing.T) {
	input := repositoryTestInput()
	result, err := domaincapacity.Calculate(input, repositoryTestAssumptions(), time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	inputJSON, err := json.Marshal(newCapacityPersistenceInput(input))
	if err != nil {
		t.Fatalf("Marshal(input) error = %v", err)
	}
	resultJSON, err := json.Marshal(newCapacityPersistenceResult(result))
	if err != nil {
		t.Fatalf("Marshal(result) error = %v", err)
	}

	record, err := capacityCalculationFromModel(CapacityCalculationModel{ID: "calc_complete", Input: inputJSON, Result: resultJSON})
	if err != nil {
		t.Fatalf("capacityCalculationFromModel() error = %v", err)
	}
	if !reflect.DeepEqual(record.Input, input) {
		t.Fatalf("Input = %#v, want %#v", record.Input, input)
	}
	if !reflect.DeepEqual(record.Result, result) {
		t.Fatalf("Result = %#v, want %#v", record.Result, result)
	}
}

func TestCapacityPersistenceMarksOldJSONAsLegacyWithoutGuessingAssumptions(t *testing.T) {
	record, err := capacityCalculationFromModel(CapacityCalculationModel{
		ID:     "calc_legacy",
		Input:  json.RawMessage(`{"cashOnHand":150,"monthlyIncome":3.5,"targetTotalPrice":500}`),
		Result: json.RawMessage(`{"safeTotalPrice":510,"pressureLevel":"safe","strategy":"可以同步推进"}`),
	})
	if err != nil {
		t.Fatalf("capacityCalculationFromModel() error = %v", err)
	}
	if record.Result.TraceabilityStatus != domaincapacity.TraceabilityLegacyUnversioned {
		t.Fatalf("TraceabilityStatus = %q, want legacy_unversioned", record.Result.TraceabilityStatus)
	}
	if record.Result.AppliedAssumptions != nil || record.Result.RuleVersion != "" || record.Result.EffectiveDate != "" {
		t.Fatalf("legacy trace metadata = %#v, want no inferred assumptions", record.Result)
	}
}

func TestCapacityRepositoryPersistsAndFindsCalculations(t *testing.T) {
	databaseURL := os.Getenv("PROPULSE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("PROPULSE_TEST_DATABASE_URL is not set")
	}

	ctx := context.Background()

	if err := migraterunner.Run(ctx, databaseURL, "up"); err != nil {
		t.Fatalf("Run(up) error = %v", err)
	}

	db, sqlDB, err := Open(databaseURL)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	repo := NewCapacityRepository(db)
	input := repositoryTestInput()
	result, err := domaincapacity.Calculate(input, repositoryTestAssumptions(), time.Now())
	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}
	record, err := repo.Save(ctx, appcapacity.CalculationRecord{
		ID: uuid.NewString(), UserID: "repository-test-user", Input: input, Result: result, CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	found, err := repo.Find(ctx, record.ID)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if !reflect.DeepEqual(found.Input, input) || !reflect.DeepEqual(found.Result, result) {
		t.Fatalf("found calculation lost traceability: %#v", found)
	}
	latest, err := repo.FindLatestByUser(ctx, "repository-test-user")
	if err != nil {
		t.Fatalf("FindLatestByUser() error = %v", err)
	}
	if latest.ID != record.ID || !reflect.DeepEqual(latest.Result, result) {
		t.Fatalf("latest = %#v, want calculation %q", latest, record.ID)
	}
}
