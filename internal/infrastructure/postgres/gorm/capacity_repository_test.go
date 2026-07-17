package gormrepo

import (
	"context"
	"encoding/json"
	"errors"
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

func TestCapacityPersistenceRoundTripsVersionedPolicyCalculation(t *testing.T) {
	input := domaincapacity.HousingCapacityInput{
		CashOnHand: 150, OldHomeValue: 320, OldLoanBalance: 80, MonthlyIncome: 3.5,
		CurrentMonthlyMortgage: 0, AcceptableMonthlyMortgage: 1.5, TargetTotalPrice: 500,
		RenovationBudget: 40, TransitionRentCost: 5,
		TransactionScenario: &domaincapacity.TransactionScenario{
			City: "天津", HomePurchaseOrder: domaincapacity.HomeFirst, TargetHomeType: domaincapacity.TargetHomeResale,
			TargetHomeAreaSQM: 120, OldHomeHoldingYears: 5, OldHomeOnlyFamilyHome: true,
			OldHomeOriginalPrice: 200, TaxBurdenMode: domaincapacity.TaxBurdenStatutory,
		},
		LoanPlan: &domaincapacity.LoanPlan{
			Type: domaincapacity.LoanCommercial, TotalLoanAmount: 400, LoanTermMonths: 360,
			RepaymentMethod: domaincapacity.RepaymentEqualInstallment,
		},
	}
	policy := repositoryTestPolicy("天津", "policy-round-trip", "2026-01-01")
	asOf := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	result, err := domaincapacity.CalculateWithPolicy(input, repositoryTestAssumptions(), policy, asOf)
	if err != nil {
		t.Fatalf("CalculateWithPolicy() error = %v", err)
	}
	inputJSON, err := json.Marshal(newCapacityPersistenceInput(input))
	if err != nil {
		t.Fatal(err)
	}
	resultJSON, err := json.Marshal(newCapacityPersistenceResult(result))
	if err != nil {
		t.Fatal(err)
	}
	record, err := capacityCalculationFromModel(CapacityCalculationModel{ID: "calc_policy", Input: inputJSON, Result: resultJSON})
	if err != nil {
		t.Fatal(err)
	}
	// An empty override list is omitted in JSON and returns as nil; both mean no user override.
	result.ManualOverrides = nil
	if !reflect.DeepEqual(record.Input, input) || !reflect.DeepEqual(record.Result, result) {
		t.Fatalf("versioned calculation lost traceability: %#v", record)
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

func TestCapacityPersistenceRoundTripsIndependentSelectionContext(t *testing.T) {
	confirmedAt := time.Date(2026, 7, 17, 8, 0, 0, 0, time.UTC)
	contextSnapshot := appcapacity.SelectionContext{
		OldHome: &appcapacity.OldHomeSelectionSnapshot{Mode: appcapacity.OldHomeNone, ConfirmedAt: confirmedAt},
		TargetHome: &appcapacity.TargetHomeSelectionSnapshot{
			Property: appcapacity.SelectionPropertySnapshot{
				NeighborhoodID: "22222222-2222-4222-8222-222222222222", NeighborhoodName: "海河花园",
				City: "天津", District: "河西区", Layout: "3室2厅", AreaSQM: 118,
			},
			ConfirmedPurchasePriceWan: 480, PriceDifferenceWan: -20, ConfirmedAt: confirmedAt,
		},
	}
	encoded, err := json.Marshal(contextSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	record, err := capacityCalculationFromModel(CapacityCalculationModel{
		ID: "calc-selection", Input: json.RawMessage(`{"monthlyIncome":3.5,"targetTotalPrice":480}`),
		Result: json.RawMessage(`{"pressureLevel":"safe"}`), SelectionContext: encoded,
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.SelectionContext == nil || !reflect.DeepEqual(*record.SelectionContext, contextSnapshot) {
		t.Fatalf("SelectionContext = %#v, want %#v", record.SelectionContext, contextSnapshot)
	}
}

func TestCapacityPolicyPersistenceRoundTripsRulesAndSources(t *testing.T) {
	policy := repositoryTestPolicy("测试市", "policy-v1", "2026-01-01")
	model, err := capacityPolicyModel(policy)
	if err != nil {
		t.Fatalf("capacityPolicyModel() error = %v", err)
	}
	got, err := capacityPolicyFromModel(model)
	if err != nil {
		t.Fatalf("capacityPolicyFromModel() error = %v", err)
	}
	if !reflect.DeepEqual(got, policy) {
		t.Fatalf("policy round trip = %#v, want %#v", got, policy)
	}
}

func TestCapacityPolicyRepositoryAppendsFutureVersionAndClosesPredecessor(t *testing.T) {
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
	t.Cleanup(func() { _ = sqlDB.Close() })
	repo := NewCapacityPolicyRepository(db)
	city := "仓储测试-" + uuid.NewString()

	first := repositoryTestPolicy(city, "v1", "2026-01-01")
	if _, err := repo.Create(ctx, first); err != nil {
		t.Fatalf("Create(first) error = %v", err)
	}
	second := repositoryTestPolicy(city, "v2", "2027-01-01")
	if _, err := repo.Create(ctx, second); err != nil {
		t.Fatalf("Create(second) error = %v", err)
	}

	oldEffective, err := repo.FindEffective(ctx, city, time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC))
	if err != nil || oldEffective.Version != "v1" || oldEffective.EffectiveTo == nil || *oldEffective.EffectiveTo != "2027-01-01" {
		t.Fatalf("old effective policy = %#v, %v", oldEffective, err)
	}
	newEffective, err := repo.FindEffective(ctx, city, time.Date(2027, 1, 1, 12, 0, 0, 0, time.UTC))
	if err != nil || newEffective.Version != "v2" {
		t.Fatalf("new effective policy = %#v, %v", newEffective, err)
	}

	overlap := repositoryTestPolicy(city, "overlap", "2026-06-01")
	if _, err := repo.Create(ctx, overlap); !errors.Is(err, appcapacity.ErrPolicyConflict) {
		t.Fatalf("Create(overlap) error = %v, want ErrPolicyConflict", err)
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

func repositoryTestPolicy(city, version, effectiveFrom string) domaincapacity.HousingPolicyVersion {
	return domaincapacity.HousingPolicyVersion{
		ID: uuid.NewString(), City: city, Version: version, Name: "仓储测试政策",
		EffectiveFrom: effectiveFrom, Enabled: true, CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Rules: domaincapacity.HousingPolicyRules{
			DownPayment: domaincapacity.DownPaymentRules{
				CommercialFirst: 0.15, CommercialSecond: 0.15, ProvidentFirst: 0.20,
				ProvidentSecond: 0.20, CombinedFirst: 0.20, CombinedSecond: 0.20,
			},
			Interest: domaincapacity.InterestRateRules{
				CommercialFirst: 0.031, CommercialSecond: 0.031,
				ProvidentFirstUpToFiveYears: 0.021, ProvidentFirstOverFiveYears: 0.026,
				ProvidentSecondUpToFiveYears: 0.02525, ProvidentSecondOverFiveYears: 0.03075,
			},
			Tax: domaincapacity.TaxRules{
				DeedFirstUpToAreaRate: 0.01, DeedFirstOverAreaRate: 0.015,
				DeedSecondUpToAreaRate: 0.01, DeedSecondOverAreaRate: 0.02,
				DeedAreaThresholdSQM: 140, VATRate: 0.03, VATExemptHoldingYears: 2,
				VATSurchargeRate: 0.06, IncomeTaxGainRate: 0.20, IncomeTaxAssessedRate: 0.01,
				IncomeTaxExemptHoldingYears: 5,
			},
		},
		Sources: []domaincapacity.PolicySource{{
			Code: "official", Title: "官方来源", Issuer: "主管部门",
			URL: "https://example.com/policy", EffectiveDate: effectiveFrom,
		}},
	}
}
