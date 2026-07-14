package neighborhood

import (
	"context"
	"strings"
	"time"
)

type Service struct {
	repo             Repository
	algorithmVersion string
	now              func() time.Time
}

func NewService(repo Repository) *Service {
	return NewServiceWithMetricConfig(repo, "", time.Now)
}

func NewServiceWithMetricConfig(repo Repository, algorithmVersion string, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, algorithmVersion: strings.TrimSpace(algorithmVersion), now: now}
}

type CreateNeighborhoodCommand struct {
	Name         string
	Area         string
	TargetLayout string
}

func (s *Service) CreateNeighborhood(ctx context.Context, command CreateNeighborhoodCommand) (Neighborhood, error) {
	return s.repo.CreateNeighborhood(ctx, CreateNeighborhoodInput{
		Name:         command.Name,
		Area:         command.Area,
		TargetLayout: command.TargetLayout,
	})
}

type AddWatchlistItemCommand struct {
	UserID         string
	NeighborhoodID string
}

func (s *Service) AddWatchlistItem(ctx context.Context, command AddWatchlistItemCommand) (WatchlistItem, error) {
	return s.repo.AddWatchlistItem(ctx, command.UserID, command.NeighborhoodID)
}
