package domain_test

import (
	"strings"
	"testing"

	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"github.com/stretchr/testify/require"
)

const id = "11111111-1111-1111-1111-111111111111"

func TestFilters_BoundariesAndNormalization(t *testing.T) {
	for _, tc := range []struct {
		name string
		c, t []string
		l    string
		bad  bool
	}{
		{"empty", nil, nil, "", true}, {"location", nil, nil, "Thimphu", false}, {"exact name", nil, nil, " Thimphu ", false},
		{"max location", nil, nil, strings.Repeat("ཀ", 100), false}, {"long location", nil, nil, strings.Repeat("ཀ", 101), true},
		{"NUL", nil, nil, "a\x00", true}, {"invalid UTF8", nil, nil, string([]byte{255}), true},
		{"bad category", []string{"bad"}, nil, "", true}, {"bad tag", nil, []string{"bad"}, "", true},
		{"20 categories", repeat(id, 20), nil, "", false}, {"21 categories", repeat(id, 21), nil, "", true},
		{"20 tags", nil, repeat(id, 20), "", false}, {"21 tags", nil, repeat(id, 21), "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := domain.NormalizeFilters(tc.c, tc.t, tc.l)
			if tc.bad {
				require.ErrorIs(t, err, domain.ErrInvalid)
			} else {
				require.NoError(t, err)
			}
		})
	}
	c, tags, err := domain.NormalizeFilters([]string{"AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA", id, id}, []string{id, id}, "")
	require.NoError(t, err)
	require.Equal(t, []string{id, "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"}, c)
	require.Equal(t, []string{id}, tags)
}
func repeat(v string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = v
	}
	return out
}
func TestCaller_RequiresApplicant(t *testing.T) {
	for _, tc := range []struct {
		name, id, role string
		err            error
	}{
		{"applicant", id, "applicant", nil}, {"employer", id, "employer", domain.ErrPermissionDenied}, {"admin", id, "admin", domain.ErrPermissionDenied}, {"unknown", id, "other", domain.ErrUnauthenticated}, {"missing", "", "", domain.ErrUnauthenticated}, {"bad ID", "bad", "applicant", domain.ErrUnauthenticated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := (domain.Caller{UserID: tc.id, Role: tc.role}).RequireRole("applicant")
			require.ErrorIs(t, err, tc.err)
		})
	}
}
func TestPagination_Boundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		size  int32
		token string
		want  domain.Page
		bad   bool
	}{
		{"default", 0, "", domain.Page{Size: 20}, false}, {"cap", 101, "20", domain.Page{Size: 100, Offset: 20}, false}, {"negative", -1, "", domain.Page{}, true}, {"negative offset", 1, "-1", domain.Page{}, true}, {"plus", 1, "+1", domain.Page{}, true}, {"space", 1, " 1", domain.Page{}, true}, {"text", 1, "x", domain.Page{}, true}, {"overflow", 1, "9223372036854775808", domain.Page{}, true}, {"max", 1, "9223372036854775807", domain.Page{Size: 1, Offset: 9223372036854775807}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := domain.ParsePage(tc.size, tc.token)
			if tc.bad {
				require.ErrorIs(t, err, domain.ErrPagination)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.want, p)
			}
		})
	}
}

func TestResolvedNames_RequiresEverySuppliedID(t *testing.T) {
	for _, tc := range []struct {
		name             string
		names            domain.Names
		categories, tags []string
		bad              bool
	}{
		{"category present", domain.Names{Categories: map[string]string{id: "Engineering"}}, []string{id}, nil, false},
		{"tag present", domain.Names{Tags: map[string]string{id: "Go"}}, nil, []string{id}, false},
		{"category omitted", domain.Names{}, []string{id}, nil, true},
		{"tag omitted", domain.Names{}, nil, []string{id}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.names.Validate(tc.categories, tc.tags)
			if tc.bad {
				require.ErrorIs(t, err, domain.ErrInvalid)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
