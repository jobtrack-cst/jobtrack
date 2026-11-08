package domain

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const MaxLocationCharacters = 100

type Alert struct {
	ID, ApplicantID      string
	CategoryIDs, TagIDs  []string
	Location             string
	Names                Names
	CreatedAt, UpdatedAt time.Time
}
type Names struct{ Categories, Tags map[string]string }

func NormalizeFilters(categories, tags []string, location string) ([]string, []string, error) {
	if len(categories) > 20 || len(tags) > 20 || !utf8.ValidString(location) || strings.ContainsRune(location, 0) || utf8.RuneCountInString(location) > MaxLocationCharacters {
		return nil, nil, ErrInvalid
	}
	normalize := func(ids []string) ([]string, error) {
		out := make([]string, 0, len(ids))
		for _, raw := range ids {
			id, err := NormalizeID(raw, ErrInvalid)
			if err != nil {
				return nil, err
			}
			out = append(out, id)
		}
		sort.Strings(out)
		return slices.Compact(out), nil
	}
	c, err := normalize(categories)
	if err != nil {
		return nil, nil, err
	}
	t, err := normalize(tags)
	if err != nil {
		return nil, nil, err
	}
	if len(c) == 0 && len(t) == 0 && location == "" {
		return nil, nil, ErrInvalid
	}
	return c, t, nil
}
func (n Names) Validate(categories, tags []string) error {
	for _, id := range categories {
		if _, ok := n.Categories[id]; !ok {
			return ErrInvalid
		}
	}
	for _, id := range tags {
		if _, ok := n.Tags[id]; !ok {
			return ErrInvalid
		}
	}
	return nil
}

type Page struct {
	Size   int
	Offset int64
}

func ParsePage(size int32, token string) (Page, error) {
	if size < 0 {
		return Page{}, ErrPagination
	}
	if size == 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	var offset int64
	if token != "" {
		for _, c := range token {
			if c < '0' || c > '9' {
				return Page{}, ErrPagination
			}
		}
		var err error
		offset, err = strconv.ParseInt(token, 10, 64)
		if err != nil {
			return Page{}, ErrPagination
		}
	}
	return Page{int(size), offset}, nil
}
