package neighborhood

import "context"

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
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
