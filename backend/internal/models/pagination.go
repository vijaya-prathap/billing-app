package models

const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

type Pagination struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

func NewPagination(page, limit int) Pagination {
	if page < 1 {
		page = DefaultPage
	}
	if limit < 1 {
		limit = DefaultLimit
	}
	if limit > MaxLimit {
		limit = MaxLimit
	}
	return Pagination{Page: page, Limit: limit}
}

func (p Pagination) Offset() int {
	return (p.Page - 1) * p.Limit
}

type PaginatedResponse[T any] struct {
	Data       []T   `json:"data"`
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func NewPaginatedResponse[T any](data []T, p Pagination, total int64) *PaginatedResponse[T] {
	if data == nil {
		data = []T{}
	}
	totalPages := int((total + int64(p.Limit) - 1) / int64(p.Limit))
	return &PaginatedResponse[T]{
		Data:       data,
		Page:       p.Page,
		Limit:      p.Limit,
		Total:      total,
		TotalPages: totalPages,
	}
}
