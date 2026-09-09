// Package pagination parses the shared list-query parameters
// (page, pageSize, sort) and builds the shared page envelope.
package pagination

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPageSize = 20
	MaxPageSize     = 200
)

type Query struct {
	Page     int
	PageSize int
	SortBy   string
	SortDesc bool
}

func (q Query) Offset() int { return (q.Page - 1) * q.PageSize }
func (q Query) Limit() int  { return q.PageSize }

// Parse reads page/pageSize/sort from the query string. sort is "field" or "-field".
func Parse(c *gin.Context) Query {
	q := Query{Page: 1, PageSize: DefaultPageSize}
	if v, err := strconv.Atoi(c.Query("page")); err == nil && v > 0 {
		q.Page = v
	}
	if v, err := strconv.Atoi(c.Query("pageSize")); err == nil && v > 0 {
		q.PageSize = min(v, MaxPageSize)
	}
	if s := strings.TrimSpace(c.Query("sort")); s != "" {
		if strings.HasPrefix(s, "-") {
			q.SortDesc = true
			s = s[1:]
		}
		q.SortBy = s
	}
	return q
}

// Page is the list response shape.
type Page[T any] struct {
	Items    []T   `json:"items"`
	Total    int64 `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
}

func NewPage[T any](items []T, total int64, q Query) Page[T] {
	if items == nil {
		items = []T{}
	}
	return Page[T]{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}
}
