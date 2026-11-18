package domain

// RemovalPublishesClose reports whether a moderation removal must publish
// job.closed. Only a published job is known to job-search and saved-jobs, and
// a closed job already sent job.closed, so analytics would count it twice.
func RemovalPublishesClose(previous Status) bool {
	return previous == StatusPublished
}
