package asset

import (
	"context"
	"strings"

	"github.com/google/uuid"
	domainasset "github.com/sine-io/propulse/internal/domain/asset"
)

type GetAssetQuery struct {
	UserID string
	ID     string
}

func (s *Service) GetAsset(ctx context.Context, query GetAssetQuery) (domainasset.Asset, error) {
	userID := strings.TrimSpace(query.UserID)
	id, err := uuid.Parse(strings.TrimSpace(query.ID))
	if err != nil || userID == "" {
		return domainasset.Asset{}, ErrInvalidCommand
	}
	return s.repo.Find(ctx, userID, id.String())
}

type ListAssetsQuery struct {
	UserID   string
	Page     int
	PageSize int
}

func (s *Service) ListAssets(ctx context.Context, query ListAssetsQuery) (Page, error) {
	userID := strings.TrimSpace(query.UserID)
	if userID == "" || query.Page < 0 || query.PageSize < 0 {
		return Page{}, ErrInvalidCommand
	}
	page := query.Page
	if page == 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize == 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		return Page{}, ErrInvalidCommand
	}
	items, total, err := s.repo.List(ctx, userID, pageSize, (page-1)*pageSize)
	if err != nil {
		return Page{}, err
	}
	return Page{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}
