package capacity

import (
	"context"
	"errors"
	"time"

	domaincapacity "github.com/sine-io/propulse/backend/internal/domain/capacity"
)

var ErrCalculationNotFound = errors.New("capacity calculation not found")

type CalculationRepository interface {
	Save(ctx context.Context, record CalculationRecord) (CalculationRecord, error)
	Find(ctx context.Context, id string) (CalculationRecord, error)
	FindLatestByUser(ctx context.Context, userID string) (CalculationRecord, error)
}

type CalculationRecord struct {
	ID        string
	UserID    string
	Input     domaincapacity.HousingCapacityInput
	Result    domaincapacity.HousingCapacityResult
	CreatedAt time.Time
}
