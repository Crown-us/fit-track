package httpapi

import (
	"net/http"
	"strconv"

	"github.com/Crown-us/fit-track/internal/domain"
)

type pageMeta struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	Total  int `json:"total"`
}

type listBody[T any] struct {
	Data       []T      `json:"data"`
	Pagination pageMeta `json:"pagination"`
}

func writeList[T any](w http.ResponseWriter, r *http.Request, items []T, page domain.Page, total int) {
	writeJSON(w, r, http.StatusOK, listBody[T]{
		Data:       items,
		Pagination: pageMeta{Limit: page.Limit, Offset: page.Offset, Total: total},
	})
}

func pathID(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.ErrNotFound
	}
	return id, nil
}

func parsePage(r *http.Request) (domain.Page, error) {
	page := domain.Page{Limit: domain.DefaultPageLimit}
	var v domain.Validator
	query := r.URL.Query()
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		v.Check(err == nil && limit >= 1 && limit <= domain.MaxPageLimit, "limit", "must be between 1 and 100")
		page.Limit = limit
	}
	if raw := query.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		v.Check(err == nil && offset >= 0, "offset", "must be 0 or greater")
		page.Offset = offset
	}
	return page, v.Err()
}
