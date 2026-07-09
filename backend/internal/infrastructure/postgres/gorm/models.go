package gormrepo

import (
	"encoding/json"
	"time"
)

type CapacityCalculationModel struct {
	ID        string          `gorm:"column:id;type:uuid;primaryKey"`
	Input     json.RawMessage `gorm:"column:input;type:jsonb;not null"`
	Result    json.RawMessage `gorm:"column:result;type:jsonb;not null"`
	CreatedAt time.Time       `gorm:"column:created_at;autoCreateTime"`
}

func (CapacityCalculationModel) TableName() string {
	return "capacity_calculations"
}
