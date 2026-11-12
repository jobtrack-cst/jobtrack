package domain

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Limits agreed in JT-50, in characters after trimming. The migration
// enforces the same values, so update both together.
const (
	MaxSummaryLength       = 2000
	MaxEducationEntries    = 20
	MaxEducationTextLength = 150
	MaxSkills              = 50
	MaxSkillLength         = 50
	MinStartYear           = 1950
	// MaxStartYearAhead and MaxEndYearAhead are counted from the current year.
	MaxStartYearAhead = 1
	MaxEndYearAhead   = 10
)

// Education is one entry. EndYear 0 means the study is ongoing.
type Education struct {
	Institution   string
	Qualification string
	StartYear     int32
	EndYear       int32
}

// ProfileInput is the raw content of a create or update request.
type ProfileInput struct {
	Summary   string
	Education []Education
	Skills    []string
}

// ProfileContent is validated, trimmed content. Lists are never nil and keep
// the order they were sent in.
type ProfileContent struct {
	Summary   string
	Education []Education
	Skills    []string
}

// Profile is one applicant's stored profile. ApplicantID is the user ID.
type Profile struct {
	ApplicantID string
	ProfileContent
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewProfileContent applies the JT-50 rules. now supplies the current year
// for the year limits.
func NewProfileContent(in ProfileInput, now time.Time) (ProfileContent, error) {
	summary, ok := cleanText(in.Summary, 0, MaxSummaryLength, true)
	if !ok {
		return ProfileContent{}, ErrInvalidSummary
	}
	if len(in.Education) > MaxEducationEntries {
		return ProfileContent{}, ErrTooManyEducation
	}
	year := int32(now.UTC().Year())
	education := make([]Education, 0, len(in.Education))
	for i, e := range in.Education {
		clean, err := cleanEducation(e, year)
		if err != nil {
			return ProfileContent{}, fmt.Errorf("education[%d]: %w", i, err)
		}
		education = append(education, clean)
	}
	if len(in.Skills) > MaxSkills {
		return ProfileContent{}, ErrTooManySkills
	}
	skills := make([]string, 0, len(in.Skills))
	seen := make(map[string]bool, len(in.Skills))
	for i, raw := range in.Skills {
		skill, ok := cleanText(raw, 1, MaxSkillLength, false)
		if !ok {
			return ProfileContent{}, fmt.Errorf("skills[%d]: %w", i, ErrInvalidSkill)
		}
		key := strings.ToLower(skill)
		if seen[key] {
			return ProfileContent{}, fmt.Errorf("skills[%d]: %w", i, ErrDuplicateSkill)
		}
		seen[key] = true
		skills = append(skills, skill)
	}
	return ProfileContent{Summary: summary, Education: education, Skills: skills}, nil
}

func cleanEducation(e Education, year int32) (Education, error) {
	institution, ok := cleanText(e.Institution, 1, MaxEducationTextLength, false)
	if !ok {
		return Education{}, ErrInvalidInstitution
	}
	qualification, ok := cleanText(e.Qualification, 1, MaxEducationTextLength, false)
	if !ok {
		return Education{}, ErrInvalidQualification
	}
	if e.StartYear < MinStartYear || e.StartYear > year+MaxStartYearAhead {
		return Education{}, ErrInvalidStartYear
	}
	if e.EndYear != 0 && (e.EndYear < e.StartYear || e.EndYear > year+MaxEndYearAhead) {
		return Education{}, ErrInvalidEndYear
	}
	return Education{Institution: institution, Qualification: qualification, StartYear: e.StartYear, EndYear: e.EndYear}, nil
}

// cleanText trims s and checks its length in characters. It refuses invalid
// UTF-8 and control characters; multiline allows line breaks and tabs, which
// only the summary needs.
func cleanText(s string, minLen, maxLen int, multiline bool) (string, bool) {
	if !utf8.ValidString(s) {
		return "", false
	}
	for _, r := range s {
		if unicode.IsControl(r) && !(multiline && (r == '\n' || r == '\r' || r == '\t')) {
			return "", false
		}
	}
	s = strings.TrimSpace(s)
	n := utf8.RuneCountInString(s)
	return s, n >= minLen && n <= maxLen
}
