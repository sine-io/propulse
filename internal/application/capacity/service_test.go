package capacity

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sine-io/propulse/internal/application/user"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
)

func TestCreateCalculationPersistsComputedResult(t *testing.T) {
	repo := &memoryCalculationRepository{
		nextID:    "calc_123",
		createdAt: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
	}
	service := NewService(repo, repo.now, repo.newID)

	record, err := service.CreateCalculation(context.Background(), CreateCalculationCommand{
		Input: domaincapacity.HousingCapacityInput{
			CashOnHand:                150,
			OldHomeValue:              320,
			OldLoanBalance:            80,
			MonthlyIncome:             3.5,
			CurrentMonthlyMortgage:    0,
			AcceptableMonthlyMortgage: 1.5,
			TargetTotalPrice:          550,
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
	if len(repo.records) != 1 {
		t.Fatalf("saved records = %d, want 1", len(repo.records))
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
	service := NewService(repo, time.Now, func() string { return "unused" })

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
	service := NewService(repo, time.Now, func() string { return "unused" })

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
	service := NewService(repo, time.Now, func() string { return "unused" })

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
	service := NewService(repo, time.Now, func() string { return "unused" })

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
