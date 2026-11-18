package domain

import (
	"slices"
	"sort"
)

// PublishedJob contains only fields needed by subscription matching.
type PublishedJob struct {
	ID, CategoryID, Location string
	TagIDs                   []string
}

type Match struct{ AlertID, ApplicantID string }

// Matches combines the three filters with AND; empty filters are unrestricted.
func (a Alert) Matches(job PublishedJob) bool {
	if len(a.CategoryIDs) > 0 && !slices.Contains(a.CategoryIDs, job.CategoryID) {
		return false
	}
	if a.Location != "" && a.Location != job.Location {
		return false
	}
	if len(a.TagIDs) == 0 {
		return true
	}
	for _, tag := range job.TagIDs {
		if slices.Contains(a.TagIDs, tag) {
			return true
		}
	}
	return false
}

// PlanMatches preserves each matching subscription in deterministic alert order.
func PlanMatches(job PublishedJob, candidates []Alert) []Match {
	matches := []Match{}
	for _, a := range candidates {
		if a.Matches(job) {
			matches = append(matches, Match{AlertID: a.ID, ApplicantID: a.ApplicantID})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].AlertID < matches[j].AlertID })
	return matches
}

// MatchApplicants sends one notification per applicant, regardless of alert count.
func MatchApplicants(matches []Match) []string {
	applicants := make([]string, 0, len(matches))
	for _, m := range matches {
		applicants = append(applicants, m.ApplicantID)
	}
	sort.Strings(applicants)
	return slices.Compact(applicants)
}
