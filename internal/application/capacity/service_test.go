package capacity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sine-io/propulse/internal/application/user"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
)

func testAssumptions() domaincapacity.Assumptions {
	return domaincapacity.Assumptions{
		RuleVersion:   "2026.07.14",
		EffectiveDate: "2026-07-14",
		RuleSource:    "test capacity rules",
		Loan: domaincapacity.LoanParams{
			AnnualInterestRate: 0.039,
			LoanTermMonths:     360,
			RepaymentMethod:    domaincapacity.RepaymentEqualInstallment,
		},
		LoanSource: "test loan defaults",
		LoanOrigin: domaincapacity.OriginConfiguredDefault,
		CityPolicy: domaincapacity.CityPolicy{
			City:            "测试市",
			PolicyName:      "测试首付政策",
			DownPaymentRate: 0.35,
			EffectiveDate:   "2026-07-14",
			Source:          "测试政策来源",
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
}

func TestCreateCalculationPersistsComputedResult(t *testing.T) {
	repo := &memoryCalculationRepository{
		nextID:    "calc_123",
		createdAt: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC),
	}
	service := NewService(repo, testAssumptions(), repo.now, repo.newID)

	record, err := service.CreateCalculation(context.Background(), CreateCalculationCommand{
		Input: domaincapacity.HousingCapacityInput{
			CashOnHand:                150,
			OldHomeValue:              320,
			OldLoanBalance:            80,
			MonthlyIncome:             3.5,
			CurrentMonthlyMortgage:    0,
			AcceptableMonthlyMortgage: 1.5,
			TargetTotalPrice:          500,
			RenovationBudget:          40,
			TransactionCosts:          18,
			TransitionRentCost:        5,
		},
	})
	if err != nil {
		t.Fatalf("CreateCalculation() error = %v", err)
	}

	if record.ID != "calc_123" {
		t.Fatalf("record.ID = %q, want calc_123", record.ID)
	}
	if record.Result.PressureLevel != domaincapacity.PressureStrained {
		t.Fatalf("record.Result.PressureLevel = %q", record.Result.PressureLevel)
	}
	if record.Result.Strategy != "先卖后买或同步推进" {
		t.Fatalf("record.Result.Strategy = %q", record.Result.Strategy)
	}
	if record.Result.TraceabilityStatus != domaincapacity.TraceabilityComplete || record.Result.AppliedAssumptions == nil {
		t.Fatalf("traceability = %q/%#v", record.Result.TraceabilityStatus, record.Result.AppliedAssumptions)
	}
	if len(repo.records) != 1 {
		t.Fatalf("saved records = %d, want 1", len(repo.records))
	}
}

func TestCreateCalculationRejectsInvalidDomainInputBeforePersistence(t *testing.T) {
	repo := &memoryCalculationRepository{}
	service := NewService(repo, testAssumptions(), time.Now, func() string { return "unused" })

	_, err := service.CreateCalculation(context.Background(), CreateCalculationCommand{
		Input: domaincapacity.HousingCapacityInput{MonthlyIncome: 0, TargetTotalPrice: 550},
	})
	if !errors.Is(err, domaincapacity.ErrInvalidInput) {
		t.Fatalf("CreateCalculation() error = %v, want ErrInvalidInput", err)
	}
	if len(repo.records) != 0 {
		t.Fatalf("saved records = %d, want 0", len(repo.records))
	}
}

func TestGetAssumptionsReturnsInjectedRuleSet(t *testing.T) {
	repo := &memoryCalculationRepository{}
	want := testAssumptions()
	service := NewService(repo, want, time.Now, func() string { return "unused" })

	got, err := service.GetAssumptions(context.Background(), GetAssumptionsQuery{})
	if err != nil {
		t.Fatalf("GetAssumptions() error = %v", err)
	}
	if got.Legacy != want {
		t.Fatalf("GetAssumptions() = %#v, want %#v", got, want)
	}
}

func TestGetAssumptionsResolvesEffectivePolicyOptions(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	policyRepo := &memoryPolicyRepository{effective: applicationTestPolicy()}
	service := NewServiceWithPolicies(&memoryCalculationRepository{}, policyRepo, testAssumptions(), func() time.Time { return now }, func() string { return "unused" })

	view, err := service.GetAssumptions(context.Background(), GetAssumptionsQuery{
		City: " 天津 ", HomePurchaseOrder: domaincapacity.HomeSecond, LoanTermMonths: 60,
	})
	if err != nil {
		t.Fatalf("GetAssumptions() error = %v", err)
	}
	if policyRepo.findCity != "天津" || !policyRepo.findAt.Equal(now) {
		t.Fatalf("policy lookup = %q/%v", policyRepo.findCity, policyRepo.findAt)
	}
	if view.Policy == nil || view.Policy.Version != "tianjin-test" || len(view.LoanOptions) != 3 {
		t.Fatalf("view policy/options = %#v", view)
	}
	if got := *view.LoanOptions[1].ProvidentAnnualInterestRate; got != 0.02525 {
		t.Fatalf("second provident 60-month rate = %v, want 0.02525", got)
	}
	if view.Legacy.RuleVersion != "tianjin-test" || view.Legacy.RuleSource != "天津测试政策" ||
		view.Legacy.Loan.AnnualInterestRate != 0.041 || view.Legacy.Loan.LoanTermMonths != 60 ||
		view.Legacy.LoanSource != "https://example.com/policy" {
		t.Fatalf("legacy policy projection = %#v", view.Legacy)
	}
	if view.Legacy.CityPolicy.City != "天津" || view.Legacy.CityPolicy.DownPaymentRate != 0.25 ||
		view.Legacy.CityPolicy.Source != "https://example.com/policy" {
		t.Fatalf("legacy city policy projection = %#v", view.Legacy.CityPolicy)
	}
	if view.Disclaimer != domaincapacity.BudgetEstimateDisclaimer {
		t.Fatalf("disclaimer = %q", view.Disclaimer)
	}
}

func TestCreateCalculationUsesPolicyAndPersistsVersionedBreakdown(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	calculationRepo := &memoryCalculationRepository{nextID: "calc_policy", createdAt: now}
	policyRepo := &memoryPolicyRepository{effective: applicationTestPolicy()}
	service := NewServiceWithPolicies(calculationRepo, policyRepo, testAssumptions(), calculationRepo.now, calculationRepo.newID)

	record, err := service.CreateCalculation(context.Background(), CreateCalculationCommand{
		UserID: "user-1",
		Input:  applicationPolicyInput(),
	})
	if err != nil {
		t.Fatalf("CreateCalculation() error = %v", err)
	}
	if record.ID != "calc_policy" || record.Result.PolicyVersion == nil || record.Result.PolicyVersion.Version != "tianjin-test" {
		t.Fatalf("versioned record = %#v", record)
	}
	if record.Result.LoanBreakdown == nil || record.Result.TaxBreakdown == nil || record.Result.Disclaimer == "" {
		t.Fatalf("calculation breakdown = %#v", record.Result)
	}
}

func TestCreatePolicyVersionTrimsAndAppendsThroughRepository(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	policyRepo := &memoryPolicyRepository{}
	service := NewServiceWithPolicies(&memoryCalculationRepository{}, policyRepo, testAssumptions(), func() time.Time { return now }, func() string { return "policy-id" })
	policy := applicationTestPolicy()
	policy.ID = ""
	policy.City = " 天津 "
	policy.Version = " future-version "
	policy.Name = " 未来政策 "
	policy.Sources[0].Title = " 来源标题 "

	created, err := service.CreatePolicyVersion(context.Background(), CreatePolicyVersionCommand{Policy: policy})
	if err != nil {
		t.Fatalf("CreatePolicyVersion() error = %v", err)
	}
	if created.ID != "policy-id" || created.City != "天津" || created.Version != "future-version" || created.Name != "未来政策" {
		t.Fatalf("created policy = %#v", created)
	}
	if created.Sources[0].Title != "来源标题" || !created.CreatedAt.Equal(now) {
		t.Fatalf("created metadata = %#v", created)
	}
}

func TestCreatePolicyVersionRejectsIncompleteSourceBeforeRepository(t *testing.T) {
	policyRepo := &memoryPolicyRepository{}
	service := NewServiceWithPolicies(&memoryCalculationRepository{}, policyRepo, testAssumptions(), time.Now, func() string { return "policy-id" })
	policy := applicationTestPolicy()
	policy.Sources[0].URL = ""

	_, err := service.CreatePolicyVersion(context.Background(), CreatePolicyVersionCommand{Policy: policy})
	if !errors.Is(err, ErrInvalidPolicy) || policyRepo.createCalls != 0 {
		t.Fatalf("CreatePolicyVersion() error/calls = %v/%d, want ErrInvalidPolicy/0", err, policyRepo.createCalls)
	}
}

func TestGetCalculationReturnsStoredRecord(t *testing.T) {
	repo := &memoryCalculationRepository{
		records: map[string]CalculationRecord{
			"calc_123": {
				ID: "calc_123",
				Input: domaincapacity.HousingCapacityInput{
					CashOnHand: 10,
				},
				Result: domaincapacity.HousingCapacityResult{
					PressureLevel: domaincapacity.PressureSafe,
					Strategy:      "可以同步推进",
				},
			},
		},
	}
	service := NewService(repo, testAssumptions(), time.Now, func() string { return "unused" })

	record, err := service.GetCalculation(context.Background(), GetCalculationQuery{ID: "calc_123"})
	if err != nil {
		t.Fatalf("GetCalculation() error = %v", err)
	}
	if record.ID != "calc_123" {
		t.Fatalf("record.ID = %q, want calc_123", record.ID)
	}
	if record.Result.Strategy != "可以同步推进" {
		t.Fatalf("record.Result.Strategy = %q", record.Result.Strategy)
	}
}

func TestGetCalculationReturnsNotFound(t *testing.T) {
	repo := &memoryCalculationRepository{records: map[string]CalculationRecord{}}
	service := NewService(repo, testAssumptions(), time.Now, func() string { return "unused" })

	_, err := service.GetCalculation(context.Background(), GetCalculationQuery{ID: "missing"})
	if !errors.Is(err, ErrCalculationNotFound) {
		t.Fatalf("GetCalculation() error = %v, want ErrCalculationNotFound", err)
	}
}

func TestLatestCalculationReturnsNewestRecordForUser(t *testing.T) {
	repo := &memoryCalculationRepository{
		records: map[string]CalculationRecord{
			"older": {
				ID:        "older",
				UserID:    user.SingleUserID,
				CreatedAt: time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC),
			},
			"newer": {
				ID:        "newer",
				UserID:    user.SingleUserID,
				CreatedAt: time.Date(2026, 7, 9, 11, 0, 0, 0, time.UTC),
			},
			"other": {
				ID:        "other",
				UserID:    "other-user",
				CreatedAt: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewService(repo, testAssumptions(), time.Now, func() string { return "unused" })

	record, err := service.LatestCalculation(context.Background(), LatestCalculationQuery{UserID: user.SingleUserID})
	if err != nil {
		t.Fatalf("LatestCalculation() error = %v", err)
	}
	if record.ID != "newer" {
		t.Fatalf("record.ID = %q, want newer", record.ID)
	}
}

func TestLatestCalculationReturnsNotFound(t *testing.T) {
	repo := &memoryCalculationRepository{records: map[string]CalculationRecord{}}
	service := NewService(repo, testAssumptions(), time.Now, func() string { return "unused" })

	_, err := service.LatestCalculation(context.Background(), LatestCalculationQuery{UserID: user.SingleUserID})
	if !errors.Is(err, ErrCalculationNotFound) {
		t.Fatalf("LatestCalculation() error = %v, want ErrCalculationNotFound", err)
	}
}

type memoryCalculationRepository struct {
	records   map[string]CalculationRecord
	nextID    string
	createdAt time.Time
}

type memoryPolicyRepository struct {
	effective   domaincapacity.HousingPolicyVersion
	items       []domaincapacity.HousingPolicyVersion
	findCity    string
	findAt      time.Time
	createCalls int
}

func (m *memoryPolicyRepository) FindEffective(_ context.Context, city string, asOf time.Time) (domaincapacity.HousingPolicyVersion, error) {
	m.findCity = city
	m.findAt = asOf
	if m.effective.ID == "" {
		return domaincapacity.HousingPolicyVersion{}, ErrPolicyNotFound
	}
	return m.effective, nil
}

func (m *memoryPolicyRepository) List(context.Context, string) ([]domaincapacity.HousingPolicyVersion, error) {
	return append([]domaincapacity.HousingPolicyVersion(nil), m.items...), nil
}

func (m *memoryPolicyRepository) Create(_ context.Context, policy domaincapacity.HousingPolicyVersion) (domaincapacity.HousingPolicyVersion, error) {
	m.createCalls++
	m.items = append(m.items, policy)
	return policy, nil
}

func applicationTestPolicy() domaincapacity.HousingPolicyVersion {
	return domaincapacity.HousingPolicyVersion{
		ID: "policy-1", City: "天津", Version: "tianjin-test", Name: "天津测试政策",
		EffectiveFrom: "2026-01-01", Enabled: true,
		Rules: domaincapacity.HousingPolicyRules{
			DownPayment: domaincapacity.DownPaymentRules{
				CommercialFirst: 0.15, CommercialSecond: 0.25, ProvidentFirst: 0.20,
				ProvidentSecond: 0.30, CombinedFirst: 0.18, CombinedSecond: 0.28,
			},
			Interest: domaincapacity.InterestRateRules{
				CommercialFirst: 0.031, CommercialSecond: 0.041,
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
			Code: "source", Title: "测试来源", Issuer: "测试机构",
			URL: "https://example.com/policy", EffectiveDate: "2026-01-01",
		}},
	}
}

func applicationPolicyInput() domaincapacity.HousingCapacityInput {
	return domaincapacity.HousingCapacityInput{
		CashOnHand: 150, OldHomeValue: 320, OldLoanBalance: 80, MonthlyIncome: 3.5,
		CurrentMonthlyMortgage: 0, AcceptableMonthlyMortgage: 1.5, TargetTotalPrice: 500,
		RenovationBudget: 40, TransitionRentCost: 5,
		TransactionScenario: &domaincapacity.TransactionScenario{
			City: "天津", HomePurchaseOrder: domaincapacity.HomeFirst,
			TargetHomeType: domaincapacity.TargetHomeResale, TargetHomeAreaSQM: 120,
			OldHomeHoldingYears: 5, OldHomeOnlyFamilyHome: true, OldHomeOriginalPrice: 200,
			TaxBurdenMode: domaincapacity.TaxBurdenStatutory,
		},
		LoanPlan: &domaincapacity.LoanPlan{
			Type: domaincapacity.LoanCommercial, TotalLoanAmount: 400, LoanTermMonths: 360,
			RepaymentMethod: domaincapacity.RepaymentEqualInstallment,
		},
	}
}

func (m *memoryCalculationRepository) Save(_ context.Context, record CalculationRecord) (CalculationRecord, error) {
	if m.records == nil {
		m.records = map[string]CalculationRecord{}
	}
	m.records[record.ID] = record
	return record, nil
}

func (m *memoryCalculationRepository) Find(_ context.Context, id string) (CalculationRecord, error) {
	record, ok := m.records[id]
	if !ok {
		return CalculationRecord{}, ErrCalculationNotFound
	}
	return record, nil
}

func (m *memoryCalculationRepository) FindLatestByUser(_ context.Context, userID string) (CalculationRecord, error) {
	var latest CalculationRecord
	for _, record := range m.records {
		if record.UserID != userID {
			continue
		}
		if latest.ID == "" || record.CreatedAt.After(latest.CreatedAt) {
			latest = record
		}
	}
	if latest.ID == "" {
		return CalculationRecord{}, ErrCalculationNotFound
	}
	return latest, nil
}

func (m *memoryCalculationRepository) now() time.Time {
	return m.createdAt
}

func (m *memoryCalculationRepository) newID() string {
	return m.nextID
}
