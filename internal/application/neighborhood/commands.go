package neighborhood

import (
	"context"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
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
	Name             string
	City             string
	Area             string
	AvailableLayouts []string
}

func (s *Service) CreateNeighborhood(ctx context.Context, command CreateNeighborhoodCommand) (Neighborhood, error) {
	name := strings.TrimSpace(command.Name)
	city := strings.TrimSpace(command.City)
	area := strings.TrimSpace(command.Area)
	layouts := normalizeLayouts(command.AvailableLayouts)
	if name == "" || utf8.RuneCountInString(name) > 256 ||
		city == "" || utf8.RuneCountInString(city) > 128 ||
		area == "" || utf8.RuneCountInString(area) > 128 ||
		len(layouts) == 0 || slices.ContainsFunc(layouts, func(layout string) bool {
		return utf8.RuneCountInString(layout) > 64
	}) {
		return Neighborhood{}, ErrInvalidNeighborhood
	}
	return s.repo.CreateNeighborhood(ctx, CreateNeighborhoodInput{
		Name:             name,
		City:             city,
		Area:             area,
		AvailableLayouts: layouts,
	})
}

type AddWatchlistItemCommand struct {
	UserID         string
	NeighborhoodID string
	TargetLayout   string
}

func (s *Service) AddWatchlistItem(ctx context.Context, command AddWatchlistItemCommand) (WatchlistItem, error) {
	parsedNeighborhoodID, err := uuid.Parse(strings.TrimSpace(command.NeighborhoodID))
	if err != nil {
		return WatchlistItem{}, ErrInvalidNeighborhoodID
	}
	targetLayout := strings.TrimSpace(command.TargetLayout)
	if targetLayout == "" {
		return WatchlistItem{}, ErrInvalidTargetLayout
	}
	neighborhood, err := s.repo.GetNeighborhood(ctx, parsedNeighborhoodID.String())
	if err != nil {
		return WatchlistItem{}, err
	}
	if !slices.Contains(neighborhood.AvailableLayouts, targetLayout) {
		return WatchlistItem{}, ErrInvalidTargetLayout
	}
	return s.repo.AddWatchlistItem(ctx, strings.TrimSpace(command.UserID), parsedNeighborhoodID.String(), targetLayout)
}

func normalizeLayouts(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		layout := strings.TrimSpace(value)
		if layout == "" {
			continue
		}
		if _, ok := seen[layout]; ok {
			continue
		}
		seen[layout] = struct{}{}
		result = append(result, layout)
	}
	slices.Sort(result)
	return result
}
