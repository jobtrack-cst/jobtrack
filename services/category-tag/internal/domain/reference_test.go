package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/domain"
)

const (
	idA = "10000000-0000-4000-8000-000000000001"
	idB = "10000000-0000-4000-8000-000000000002"
)

func manyIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = idA
	}
	return ids
}

func TestResolveIDs(t *testing.T) {
	for _, tt := range []struct {
		name    string
		in      []string
		want    []string
		wantErr error
	}{
		{"nil", nil, []string{}, nil},
		{"one ID", []string{idA}, []string{idA}, nil},
		{"order kept", []string{idB, idA}, []string{idB, idA}, nil},
		{"duplicates removed", []string{idA, idB, idA}, []string{idA, idB}, nil},
		{"uppercase lowercased and deduplicated", []string{strings.ToUpper(idA), idA}, []string{idA}, nil},
		{"malformed dropped", []string{"not-a-uuid", "", idA, idA + " "}, []string{idA}, nil},
		{"only malformed", []string{"x"}, []string{}, nil},
		{"100 IDs allowed", manyIDs(100), []string{idA}, nil},
		{"101 IDs rejected", manyIDs(101), nil, domain.ErrTooManyIDs},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.ResolveIDs(tt.in)
			assert.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestKind_String(t *testing.T) {
	for _, tt := range []struct {
		kind domain.Kind
		want string
	}{
		{domain.KindCategory, "category"},
		{domain.KindTag, "tag"},
		{domain.KindLocation, "location"},
		{domain.KindUnspecified, ""},
	} {
		assert.Equal(t, tt.want, tt.kind.String())
	}
}
