package model

import (
	"net/http"
	"strconv"
)

const (
	_defaultPageNum  = 0
	_defaultPageSize = 10
)

// Pagination captures all info needed for pagination.
// If Size is negative, the result is an unlimited size.
type Pagination struct {
	Page int
	Size int
}

func (p *Pagination) getSize() int {
	size := p.Size
	if size == 0 {
		size = _defaultPageSize
	}

	return size
}

func (p *Pagination) Offset() int {
	if p.getSize() < 0 {
		return p.Page
	}

	return p.Page * p.getSize()
}

func (p *Pagination) Limit() int {
	return p.getSize()
}

func GetPagination(r *http.Request) *Pagination {
	pageNum, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("size"))
	return &Pagination{Page: pageNum, Size: pageSize}
}
