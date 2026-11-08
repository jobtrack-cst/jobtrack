package domain

import "time"

// TimeRange is the period of GetSummary and GetInternalSummary. Start is
// inclusive and End exclusive, compared with the envelope occurred_at.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// NewTimeRange needs both bounds, because the contract has no open-ended
// summary. A nil time means the request field was not set. Equal times are
// rejected because the contract says start must be before end.
func NewTimeRange(start, end *time.Time) (TimeRange, error) {
	if start == nil || end == nil {
		return TimeRange{}, ErrInvalidTimestamp
	}
	if !start.Before(*end) {
		return TimeRange{}, ErrInvalidTimeRange
	}
	return TimeRange{Start: start.UTC(), End: end.UTC()}, nil
}

// QueryBounds rounds both bounds up to a whole microsecond, the precision
// PostgreSQL stores. For stored values this keeps the comparison exact: an
// event at 10:00:00.000001 is before a start of 10:00:00.0000015, and after
// rounding it is still before 10:00:00.000002. The driver then never has to
// drop nanoseconds.
func (r TimeRange) QueryBounds() (start, end time.Time) {
	return ceilMicrosecond(r.Start), ceilMicrosecond(r.End)
}

// Truncate counts from the zero time, which is on a whole second, so this is
// also correct for times before 1970.
func ceilMicrosecond(t time.Time) time.Time {
	down := t.Truncate(time.Microsecond)
	if down.Equal(t) {
		return t
	}
	return down.Add(time.Microsecond)
}
