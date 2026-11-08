package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jobtrack-cst/jobtrack/services/analytics/internal/domain"
)

func TestNewTimeRange(t *testing.T) {
	t1 := time.Date(2026, 11, 8, 10, 0, 0, 0, time.FixedZone("BTT", 6*3600))
	justAfter := t1.Add(time.Nanosecond)
	before := t1.Add(-time.Hour)
	epoch := time.Unix(0, 0)
	tests := []struct {
		name       string
		start, end *time.Time
		wantErr    error
	}{
		{"one hour", &before, &t1, nil},
		{"start one nanosecond before end", &t1, &justAfter, nil},
		{"Unix epoch start is a set bound", &epoch, &t1, nil},
		{"start equal to end", &t1, &t1, domain.ErrInvalidTimeRange},
		{"start after end", &t1, &before, domain.ErrInvalidTimeRange},
		{"missing start", nil, &t1, domain.ErrInvalidTimestamp},
		{"missing end", &t1, nil, domain.ErrInvalidTimestamp},
		{"both missing", nil, nil, domain.ErrInvalidTimestamp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.NewTimeRange(tt.start, tt.end)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, domain.TimeRange{}, got)
				return
			}
			require.NoError(t, err)
			assert.True(t, tt.start.Equal(got.Start))
			assert.True(t, tt.end.Equal(got.End))
			assert.Equal(t, time.UTC, got.Start.Location())
			assert.Equal(t, time.UTC, got.End.Location())
		})
	}
}

func TestTimeRange_QueryBounds(t *testing.T) {
	base := time.Date(2026, 11, 8, 4, 0, 0, 123456000, time.UTC)
	tests := []struct {
		name               string
		start, end         time.Time
		wantStart, wantEnd time.Time
	}{
		{"whole microseconds are kept", base, base.Add(time.Hour), base, base.Add(time.Hour)},
		{"1 extra nanosecond rounds up", base.Add(time.Nanosecond), base.Add(999 * time.Nanosecond), base.Add(time.Microsecond), base.Add(time.Microsecond)},
		{"whole seconds are kept", base.Truncate(time.Second), base.Truncate(time.Second).Add(time.Second), base.Truncate(time.Second), base.Truncate(time.Second).Add(time.Second)},
		{
			"before 1970 rounds up too",
			time.Date(1969, 12, 31, 23, 59, 59, 999999500, time.UTC), time.Unix(0, 0).UTC(),
			time.Unix(0, 0).UTC(), time.Unix(0, 0).UTC(),
		},
		{
			"latest valid timestamp",
			base, time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC),
			base, time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := domain.TimeRange{Start: tt.start, End: tt.end}.QueryBounds()
			assert.True(t, tt.wantStart.Equal(gotStart), "start %v, want %v", gotStart, tt.wantStart)
			assert.True(t, tt.wantEnd.Equal(gotEnd), "end %v, want %v", gotEnd, tt.wantEnd)
		})
	}
}
