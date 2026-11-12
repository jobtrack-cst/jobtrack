package domain_test

import (
	"strings"
	"testing"
	"time"

	"github.com/jobtrack-cst/jobtrack/services/applicant-profile/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// now fixes the current year at 2026: start_year may be 1950..2027 and
// end_year 0 or start_year..2036.
var now = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)

func edu(start, end int32) domain.Education {
	return domain.Education{Institution: "College of Science and Technology", Qualification: "BE Software Engineering", StartYear: start, EndYear: end}
}

func validInput() domain.ProfileInput {
	return domain.ProfileInput{
		Summary:   "Final-year student.\nInterested in backend work.",
		Education: []domain.Education{edu(2023, 2027)},
		Skills:    []string{"Go", "PostgreSQL"},
	}
}

func n(count int, s string) []string {
	out := make([]string, count)
	for i := range out {
		out[i] = s + strings.Repeat("x", i)
	}
	return out
}

func TestNewProfileContent(t *testing.T) {
	with := func(change func(*domain.ProfileInput)) domain.ProfileInput {
		in := validInput()
		change(&in)
		return in
	}
	tests := []struct {
		name    string
		in      domain.ProfileInput
		wantErr error
	}{
		{"valid", validInput(), nil},
		{"everything empty", domain.ProfileInput{}, nil},

		// Summary: 0 to 2000 characters after trimming; line breaks allowed.
		{"summary 2000", with(func(in *domain.ProfileInput) { in.Summary = strings.Repeat("a", 2000) }), nil},
		{"summary 2001", with(func(in *domain.ProfileInput) { in.Summary = strings.Repeat("a", 2001) }), domain.ErrInvalidSummary},
		{"summary 2000 multibyte", with(func(in *domain.ProfileInput) { in.Summary = strings.Repeat("ཀ", 2000) }), nil},
		{"summary 2000 after trimming", with(func(in *domain.ProfileInput) { in.Summary = "  " + strings.Repeat("a", 2000) + "\n" }), nil},
		{"summary tab and CRLF", with(func(in *domain.ProfileInput) { in.Summary = "a\tb\r\nc" }), nil},
		{"summary NUL", with(func(in *domain.ProfileInput) { in.Summary = "a\x00b" }), domain.ErrInvalidSummary},
		{"summary invalid UTF-8", with(func(in *domain.ProfileInput) { in.Summary = "a\xffb" }), domain.ErrInvalidSummary},

		// Education count.
		{"education 20 entries", with(func(in *domain.ProfileInput) {
			in.Education = make([]domain.Education, 20)
			for i := range in.Education {
				in.Education[i] = edu(2010, 2014)
			}
		}), nil},
		{"education 21 entries", with(func(in *domain.ProfileInput) { in.Education = make([]domain.Education, 21) }), domain.ErrTooManyEducation},

		// Institution and qualification: 1 to 150, one line.
		{"institution 150", with(func(in *domain.ProfileInput) { in.Education[0].Institution = strings.Repeat("a", 150) }), nil},
		{"institution 151", with(func(in *domain.ProfileInput) { in.Education[0].Institution = strings.Repeat("a", 151) }), domain.ErrInvalidInstitution},
		{"institution empty", with(func(in *domain.ProfileInput) { in.Education[0].Institution = "" }), domain.ErrInvalidInstitution},
		{"institution whitespace", with(func(in *domain.ProfileInput) { in.Education[0].Institution = "   " }), domain.ErrInvalidInstitution},
		{"institution line break", with(func(in *domain.ProfileInput) { in.Education[0].Institution = "a\nb" }), domain.ErrInvalidInstitution},
		{"qualification 1", with(func(in *domain.ProfileInput) { in.Education[0].Qualification = "B" }), nil},
		{"qualification 150 multibyte", with(func(in *domain.ProfileInput) { in.Education[0].Qualification = strings.Repeat("é", 150) }), nil},
		{"qualification 151", with(func(in *domain.ProfileInput) { in.Education[0].Qualification = strings.Repeat("a", 151) }), domain.ErrInvalidQualification},
		{"qualification empty", with(func(in *domain.ProfileInput) { in.Education[0].Qualification = "" }), domain.ErrInvalidQualification},

		// start_year: 1950 to current year + 1.
		{"start 1950", with(func(in *domain.ProfileInput) { in.Education[0] = edu(1950, 0) }), nil},
		{"start 1949", with(func(in *domain.ProfileInput) { in.Education[0] = edu(1949, 0) }), domain.ErrInvalidStartYear},
		{"start 2027 (next year)", with(func(in *domain.ProfileInput) { in.Education[0] = edu(2027, 0) }), nil},
		{"start 2028", with(func(in *domain.ProfileInput) { in.Education[0] = edu(2028, 0) }), domain.ErrInvalidStartYear},
		{"start missing", with(func(in *domain.ProfileInput) { in.Education[0] = edu(0, 0) }), domain.ErrInvalidStartYear},

		// end_year: 0 (ongoing) or start_year to current year + 10.
		{"end 0 ongoing", with(func(in *domain.ProfileInput) { in.Education[0] = edu(2024, 0) }), nil},
		{"end equals start", with(func(in *domain.ProfileInput) { in.Education[0] = edu(2024, 2024) }), nil},
		{"end before start", with(func(in *domain.ProfileInput) { in.Education[0] = edu(2024, 2023) }), domain.ErrInvalidEndYear},
		{"end 2036", with(func(in *domain.ProfileInput) { in.Education[0] = edu(2024, 2036) }), nil},
		{"end 2037", with(func(in *domain.ProfileInput) { in.Education[0] = edu(2024, 2037) }), domain.ErrInvalidEndYear},
		{"end negative", with(func(in *domain.ProfileInput) { in.Education[0] = edu(2024, -1) }), domain.ErrInvalidEndYear},

		// Skills: up to 50, each 1 to 50, no duplicates ignoring case.
		{"skills 50", with(func(in *domain.ProfileInput) { in.Skills = n(50, "s") }), nil},
		{"skills 51", with(func(in *domain.ProfileInput) { in.Skills = n(51, "s") }), domain.ErrTooManySkills},
		{"skill 50 characters", with(func(in *domain.ProfileInput) { in.Skills = []string{strings.Repeat("a", 50)} }), nil},
		{"skill 51 characters", with(func(in *domain.ProfileInput) { in.Skills = []string{strings.Repeat("a", 51)} }), domain.ErrInvalidSkill},
		{"skill empty", with(func(in *domain.ProfileInput) { in.Skills = []string{"Go", ""} }), domain.ErrInvalidSkill},
		{"skill whitespace", with(func(in *domain.ProfileInput) { in.Skills = []string{" "} }), domain.ErrInvalidSkill},
		{"skill control character", with(func(in *domain.ProfileInput) { in.Skills = []string{"Go\x07"} }), domain.ErrInvalidSkill},
		{"duplicate skill, other case and spaces", with(func(in *domain.ProfileInput) { in.Skills = []string{"Go", " go "} }), domain.ErrDuplicateSkill},
		{"duplicate skill, multibyte case", with(func(in *domain.ProfileInput) { in.Skills = []string{"Élan", "élan"} }), domain.ErrDuplicateSkill},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewProfileContent(tt.in, now)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.ProfileContent{}, got)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, got.Education)
			assert.NotNil(t, got.Skills)
			assert.Len(t, got.Education, len(tt.in.Education))
			assert.Len(t, got.Skills, len(tt.in.Skills))
		})
	}
}

func TestNewProfileContent_TrimsAndKeepsOrder(t *testing.T) {
	got, err := domain.NewProfileContent(domain.ProfileInput{
		Summary: "  Hello  ",
		Education: []domain.Education{
			{Institution: " RUB ", Qualification: " BSc ", StartYear: 2019, EndYear: 2022},
			{Institution: "CST", Qualification: "BE", StartYear: 2023},
		},
		Skills: []string{" Go ", "React", "SQL"},
	}, now)
	require.NoError(t, err)
	assert.Equal(t, domain.ProfileContent{
		Summary: "Hello",
		Education: []domain.Education{
			{Institution: "RUB", Qualification: "BSc", StartYear: 2019, EndYear: 2022},
			{Institution: "CST", Qualification: "BE", StartYear: 2023, EndYear: 0},
		},
		Skills: []string{"Go", "React", "SQL"},
	}, got)
}

func TestNewProfileContent_ErrorNamesTheEntry(t *testing.T) {
	_, err := domain.NewProfileContent(domain.ProfileInput{
		Education: []domain.Education{edu(2019, 2022), edu(2024, 2020)},
	}, now)
	require.ErrorIs(t, err, domain.ErrInvalidEndYear)
	assert.Contains(t, err.Error(), "education[1]")

	_, err = domain.NewProfileContent(domain.ProfileInput{Skills: []string{"Go", "Rust", "go"}}, now)
	require.ErrorIs(t, err, domain.ErrDuplicateSkill)
	assert.Contains(t, err.Error(), "skills[2]")
}

func TestNewProfileContent_YearLimitsFollowTheClock(t *testing.T) {
	in := domain.ProfileInput{Education: []domain.Education{edu(2028, 0)}}
	_, err := domain.NewProfileContent(in, now)
	require.ErrorIs(t, err, domain.ErrInvalidStartYear)
	_, err = domain.NewProfileContent(in, now.AddDate(1, 0, 0))
	require.NoError(t, err, "next year 2028 is allowed once the clock reaches 2027")
}
