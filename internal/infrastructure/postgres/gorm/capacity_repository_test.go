package gormrepo

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	appcapacity "github.com/sine-io/propulse/internal/application/capacity"
	domaincapacity "github.com/sine-io/propulse/internal/domain/capacity"
	migraterunner "github.com/sine-io/propulse/internal/infrastructure/migrate"
)

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
	record, err := repo.Save(ctx, appcapacity.CalculationRecord{
		ID: uuid.NewString(),
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
		Result: domaincapacity.HousingCapacityResult{
			PressureLevel: domaincapacity.PressureStrained,
			Strategy:      "先卖后买或同步推进",
		},
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	found, err := repo.Find(ctx, record.ID)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if found.Result.PressureLevel != domaincapacity.PressureStrained {
		t.Fatalf("found.Result.PressureLevel = %q, want strained", found.Result.PressureLevel)
	}
}
