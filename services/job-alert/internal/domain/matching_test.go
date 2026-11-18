package domain_test

import (
	"github.com/jobtrack-cst/jobtrack/services/job-alert/internal/domain"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAlert_MatchesCombinesFiltersAndAllowsCategorylessJobs(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		categories, tags          []string
		location                  string
		categorized, categoryless bool
	}{
		{"category", []string{"category"}, nil, "", true, false},
		{"tag", nil, []string{"other", "tag"}, "", true, true},
		{"location", nil, nil, "Thimphu", true, true},
		{"category and tag", []string{"category"}, []string{"tag"}, "", true, false},
		{"category and location", []string{"category"}, nil, "Thimphu", true, false},
		{"tag and location", nil, []string{"tag"}, "Thimphu", true, true},
		{"all", []string{"category"}, []string{"tag"}, "Thimphu", true, false},
		{"category miss", []string{"other"}, []string{"tag"}, "Thimphu", false, false},
		{"tag miss", nil, []string{"other"}, "Thimphu", false, false},
		{"location case", nil, []string{"tag"}, "thimphu", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := domain.Alert{CategoryIDs: tc.categories, TagIDs: tc.tags, Location: tc.location}
			job := domain.PublishedJob{CategoryID: "category", TagIDs: []string{"tag"}, Location: "Thimphu"}
			require.Equal(t, tc.categorized, a.Matches(job))
			job.CategoryID = ""
			require.Equal(t, tc.categoryless, a.Matches(job))
			job.TagIDs = nil
			if len(tc.tags) > 0 {
				require.False(t, a.Matches(job))
			}
		})
	}
}
func TestMatching_PreservesAllAlertsAndNotifiesUniqueApplicantsDeterministically(t *testing.T) {
	candidates := []domain.Alert{
		{ID: "c", ApplicantID: "b", Location: "Thimphu"},
		{ID: "a", ApplicantID: "a", Location: "Thimphu"},
		{ID: "d", ApplicantID: "a", Location: "Paro"},
		{ID: "b", ApplicantID: "a", TagIDs: []string{"tag"}},
	}
	job := domain.PublishedJob{Location: "Thimphu", TagIDs: []string{"tag"}}
	matches := domain.PlanMatches(job, candidates)
	require.Equal(t, []domain.Match{{AlertID: "a", ApplicantID: "a"}, {AlertID: "b", ApplicantID: "a"}, {AlertID: "c", ApplicantID: "b"}}, matches)
	require.Equal(t, []string{"a", "b"}, domain.MatchApplicants(matches))
	require.Equal(t, "c", candidates[0].ID, "planning must not reorder repository data")
}
