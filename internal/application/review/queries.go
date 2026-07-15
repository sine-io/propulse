package review

import "context"

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type GetNoteQuery struct {
	ID string
}

func (s *Service) GetNote(ctx context.Context, query GetNoteQuery) (Note, error) {
	id, err := normalizeRequiredUUID(query.ID, ErrInvalidNoteID)
	if err != nil {
		return Note{}, err
	}
	if s.userID == "" {
		return Note{}, ErrInvalidNote
	}
	return s.repo.FindNote(ctx, s.userID, id)
}

type ListNotesQuery struct {
	Page     int
	PageSize int
}

type NotesPage struct {
	Items    []Note
	Total    int
	Page     int
	PageSize int
}

func (s *Service) ListNotes(ctx context.Context, query ListNotesQuery) (NotesPage, error) {
	if s.userID == "" {
		return NotesPage{}, ErrInvalidNote
	}
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	maxInt := int(^uint(0) >> 1)
	if page-1 > maxInt/pageSize {
		return NotesPage{}, ErrInvalidPagination
	}

	result, err := s.repo.ListNotes(ctx, ListNotesInput{
		UserID: s.userID,
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	})
	if err != nil {
		return NotesPage{}, err
	}
	if result.Items == nil {
		result.Items = []Note{}
	}
	return NotesPage{
		Items:    result.Items,
		Total:    result.Total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
