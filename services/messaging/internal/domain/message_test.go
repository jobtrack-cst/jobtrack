package domain_test

import (
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestContent_Boundaries(t *testing.T) {
	for _, tc := range []struct {
		name, raw, want string
		valid           bool
	}{{"empty", "", "", false}, {"whitespace", " \n\t ", "", false}, {"one", " 界 ", "界", true}, {"2000", strings.Repeat("界", 2000), strings.Repeat("界", 2000), true}, {"2001", strings.Repeat("界", 2001), "", false}, {"null", "a\x00b", "", false}, {"invalid UTF8", string([]byte{255}), "", false}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.ValidateContent(tc.raw)
			require.Equal(t, tc.want, got)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, domain.ErrInvalidContent)
			}
		})
	}
}
func TestPage_Boundaries(t *testing.T) {
	for _, tc := range []struct {
		name  string
		size  int32
		token string
		want  domain.Page
		valid bool
	}{{"default", 0, "", domain.Page{20, 0}, true}, {"one", 1, "0", domain.Page{1, 0}, true}, {"cap", 101, "20", domain.Page{100, 20}, true}, {"maximum", 100, "100", domain.Page{100, 100}, true}, {"negative size", -1, "", domain.Page{}, false}, {"negative offset", 1, "-1", domain.Page{}, false}, {"plus", 1, "+1", domain.Page{}, false}, {"space", 1, " 1", domain.Page{}, false}, {"decimal", 1, "1.2", domain.Page{}, false}, {"overflow", 1, "9223372036854775808", domain.Page{}, false}} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := domain.ParsePage(tc.size, tc.token)
			require.Equal(t, tc.want, p)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, domain.ErrInvalidPagination)
			}
		})
	}
}
func TestCaller_Validation(t *testing.T) {
	for _, tc := range []struct {
		id, role string
		valid    bool
	}{{"AAAAAAAA-1111-4111-8111-111111111111", "applicant", true}, {"aaaaaaaa-1111-4111-8111-111111111111", "employer", true}, {"aaaaaaaa-1111-4111-8111-111111111111", "admin", true}, {"invalid", "applicant", false}, {"aaaaaaaa-1111-4111-8111-111111111111", "other", false}} {
		c, err := domain.NewCaller(tc.id, tc.role)
		if tc.valid {
			require.NoError(t, err)
			require.Equal(t, strings.ToLower(tc.id), c.UserID)
		} else {
			require.ErrorIs(t, err, domain.ErrUnauthenticated)
		}
	}
}
