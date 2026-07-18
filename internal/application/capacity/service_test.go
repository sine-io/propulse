package capacity

import (
	"context"
	"errors"
	"sort"
	"strings"
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
				ID: "calc_123", UserID: user.SingleUserID,
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

	record, err := service.GetCalculation(context.Background(), GetCalculationQuery{UserID: user.SingleUserID, ID: "calc_123"})
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

	_, err := service.GetCalculation(context.Background(), GetCalculationQuery{UserID: user.SingleUserID, ID: "missing"})
	if !errors.Is(err, ErrCalculationNotFound) {
		t.Fatalf("GetCalculation() error = %v, want ErrCalculationNotFound", err)
	}
}

func TestGetCalculationIsIsolatedByUser(t *testing.T) {
	repo := &memoryCalculationRepository{records: map[string]CalculationRecord{
		"calc-private": {ID: "calc-private", UserID: "other-user"},
	}}
	service := NewService(repo, testAssumptions(), time.Now, nil)

	_, err := service.GetCalculation(context.Background(), GetCalculationQuery{
		UserID: user.SingleUserID, ID: "calc-private",
	})
	if !errors.Is(err, ErrCalculationNotFound) {
		t.Fatalf("GetCalculation() error = %v, want ErrCalculationNotFound", err)
	}
}

func TestListCalculationsDefaultsFiltersAndPaginatesNewestFirst(t *testing.T) {
	newer := calculationHistoryRecord("calc-newer", user.SingleUserID, time.Date(2026, 7, 17, 9, 0, 0, 0, time.UTC), "海河花园", "3室2厅", "现住房")
	older := calculationHistoryRecord("calc-older", user.SingleUserID, time.Date(2026, 7, 16, 9, 0, 0, 0, time.UTC), "梅江花园", "2室1厅", "")
	other := calculationHistoryRecord("calc-other", "other-user", time.Date(2026, 7, 18, 9, 0, 0, 0, time.UTC), "海河花园", "3室2厅", "别人的房")
	repo := &memoryCalculationRepository{records: map[string]CalculationRecord{
		newer.ID: newer, older.ID: older, other.ID: other,
	}}
	service := NewService(repo, testAssumptions(), time.Now, nil)

	page, err := service.ListCalculations(context.Background(), ListCalculationsQuery{
		UserID: " " + user.SingleUserID + " ", Page: 2, PageSize: 1,
	})
	if err != nil {
		t.Fatalf("ListCalculations() error = %v", err)
	}
	if page.Total != 2 || page.Page != 2 || page.PageSize != 1 || len(page.Items) != 1 || page.Items[0].ID != older.ID {
		t.Fatalf("page = %#v, want second user-owned record", page)
	}
	if repo.listFilter.UserID != user.SingleUserID {
		t.Fatalf("repository user = %q, want trimmed user", repo.listFilter.UserID)
	}
}

func TestListCalculationsSearchesSnapshotAndDateFields(t *testing.T) {
	record := calculationHistoryRecord("calc-history-123", user.SingleUserID, time.Date(2026, 7, 17, 9, 30, 0, 0, time.UTC), "海河花园", "3室2厅", "现住房")
	repo := &memoryCalculationRepository{records: map[string]CalculationRecord{record.ID: record}}
	service := NewService(repo, testAssumptions(), time.Now, nil)

	for _, keyword := range []string{"history-123", "海河", "3室", "现住", "2026-07-17"} {
		page, err := service.ListCalculations(context.Background(), ListCalculationsQuery{
			UserID: user.SingleUserID, Query: " " + keyword + " ",
		})
		if err != nil {
			t.Fatalf("ListCalculations(%q) error = %v", keyword, err)
		}
		if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != record.ID {
			t.Fatalf("ListCalculations(%q) = %#v, want matching record", keyword, page)
		}
	}
}

func TestListCalculationsSupportsLegacyRecordsWithoutSelectionSnapshots(t *testing.T) {
	record := CalculationRecord{
		ID: "legacy", UserID: user.SingleUserID,
		Input:     domaincapacity.HousingCapacityInput{TargetTotalPrice: 420},
		Result:    domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureStrained},
		CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	service := NewService(&memoryCalculationRepository{records: map[string]CalculationRecord{record.ID: record}}, testAssumptions(), time.Now, nil)

	page, err := service.ListCalculations(context.Background(), ListCalculationsQuery{UserID: user.SingleUserID})
	if err != nil {
		t.Fatalf("ListCalculations() error = %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].TargetNeighborhoodName != "" || page.Items[0].TargetLayout != "" || page.Items[0].OldHomeName != "" {
		t.Fatalf("legacy summary = %#v", page.Items)
	}
}

func TestListCalculationsReturnsEmptyPageAndRepositoryErrors(t *testing.T) {
	repo := &memoryCalculationRepository{records: map[string]CalculationRecord{}}
	service := NewService(repo, testAssumptions(), time.Now, nil)
	page, err := service.ListCalculations(context.Background(), ListCalculationsQuery{UserID: user.SingleUserID})
	if err != nil || page.Total != 0 || len(page.Items) != 0 || page.Page != 1 || page.PageSize != 20 {
		t.Fatalf("empty page/error = %#v/%v", page, err)
	}

	repositoryErr := errors.New("repository unavailable")
	repo.listErr = repositoryErr
	_, err = service.ListCalculations(context.Background(), ListCalculationsQuery{UserID: user.SingleUserID})
	if !errors.Is(err, repositoryErr) {
		t.Fatalf("ListCalculations() error = %v, want repository error", err)
	}
}

func TestListCalculationsRejectsInvalidPagination(t *testing.T) {
	service := NewService(&memoryCalculationRepository{}, testAssumptions(), time.Now, nil)
	for _, query := range []ListCalculationsQuery{
		{UserID: ""},
		{UserID: user.SingleUserID, Page: -1},
		{UserID: user.SingleUserID, PageSize: -1},
		{UserID: user.SingleUserID, PageSize: 101},
	} {
		if _, err := service.ListCalculations(context.Background(), query); !errors.Is(err, ErrInvalidCalculationQuery) {
			t.Fatalf("ListCalculations(%#v) error = %v, want ErrInvalidCalculationQuery", query, err)
		}
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

func calculationHistoryRecord(id, userID string, createdAt time.Time, neighborhood, layout, oldHomeName string) CalculationRecord {
	oldHome := &OldHomeSelectionSnapshot{Mode: OldHomeNone, ConfirmedAt: createdAt}
	if oldHomeName != "" {
		oldHome.Mode = OldHomeAsset
		oldHome.AssetName = oldHomeName
	}
	return CalculationRecord{
		ID: id, UserID: userID, CreatedAt: createdAt,
		Input:  domaincapacity.HousingCapacityInput{TargetTotalPrice: 500},
		Result: domaincapacity.HousingCapacityResult{PressureLevel: domaincapacity.PressureSafe},
		SelectionContext: &SelectionContext{
			OldHome: oldHome,
			TargetHome: &TargetHomeSelectionSnapshot{Property: SelectionPropertySnapshot{
				NeighborhoodName: neighborhood, Layout: layout,
			}},
		},
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
	records    map[string]CalculationRecord
	nextID     string
	createdAt  time.Time
	listErr    error
	listFilter CalculationListFilter
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

func (m *memoryCalculationRepository) FindByUser(_ context.Context, userID, id string) (CalculationRecord, error) {
	record, ok := m.records[id]
	if !ok || record.UserID != userID {
		return CalculationRecord{}, ErrCalculationNotFound
	}
	return record, nil
}

func (m *memoryCalculationRepository) ListByUser(_ context.Context, filter CalculationListFilter) (CalculationHistoryPage, error) {
	m.listFilter = filter
	if m.listErr != nil {
		return CalculationHistoryPage{}, m.listErr
	}
	records := make([]CalculationRecord, 0, len(m.records))
	keyword := strings.ToLower(filter.Query)
	for _, record := range m.records {
		if record.UserID != filter.UserID {
			continue
		}
		summary := testCalculationSummary(record)
		searchable := strings.ToLower(strings.Join([]string{
			summary.ID, summary.TargetNeighborhoodName, summary.TargetLayout, summary.OldHomeName,
			summary.CreatedAt.Format("2006-01-02 15:04:05"),
		}, " "))
		if keyword == "" || strings.Contains(searchable, keyword) {
			records = append(records, record)
		}
	}
	sort.Slice(records, func(i, j int) bool {
		if records[i].CreatedAt.Equal(records[j].CreatedAt) {
			return records[i].ID > records[j].ID
		}
		return records[i].CreatedAt.After(records[j].CreatedAt)
	})
	total := int64(len(records))
	start := (filter.Page - 1) * filter.PageSize
	if start > len(records) {
		start = len(records)
	}
	end := min(start+filter.PageSize, len(records))
	items := make([]CalculationSummary, 0, end-start)
	for _, record := range records[start:end] {
		items = append(items, testCalculationSummary(record))
	}
	return CalculationHistoryPage{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func testCalculationSummary(record CalculationRecord) CalculationSummary {
	summary := CalculationSummary{
		ID: record.ID, CreatedAt: record.CreatedAt, PressureLevel: record.Result.PressureLevel,
		TargetTotalPrice: record.Input.TargetTotalPrice,
	}
	if record.SelectionContext == nil {
		return summary
	}
	if target := record.SelectionContext.TargetHome; target != nil {
		summary.TargetNeighborhoodName = target.Property.NeighborhoodName
		summary.TargetLayout = target.Property.Layout
	}
	if oldHome := record.SelectionContext.OldHome; oldHome != nil && oldHome.Mode == OldHomeAsset {
		summary.OldHomeName = oldHome.AssetName
	}
	return summary
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
