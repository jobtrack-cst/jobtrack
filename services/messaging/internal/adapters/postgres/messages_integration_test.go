//go:build integration

package postgres_test

import (
	"github.com/google/uuid"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/messaging/internal/domain"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

func TestMessages_PersistenceOrderingAndPagination(t *testing.T) {
	ctx, pool := startParticipantsDB(t)
	application, applicant, employer := uuid.NewString(), uuid.NewString(), uuid.NewString()
	_, err := pool.Exec(ctx, `INSERT INTO application_participants VALUES($1,$2,$3)`, application, applicant, employer)
	require.NoError(t, err)
	repo := postgres.NewMessages(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	ids := []string{"00000000-0000-4000-8000-000000000003", "00000000-0000-4000-8000-000000000001", "00000000-0000-4000-8000-000000000002"}
	for _, id := range ids {
		m, err := repo.Create(ctx, domain.Message{ID: id, ApplicationID: application, SenderID: applicant, RecipientID: employer, Content: "界", CreatedAt: now, UpdatedAt: now})
		require.NoError(t, err)
		require.Equal(t, now, m.CreatedAt)
	}
	for _, tc := range []struct {
		offset      int64
		size, count int
	}{{0, 2, 2}, {2, 2, 1}, {3, 2, 0}, {99, 20, 0}} {
		items, total, err := repo.List(ctx, application, domain.Page{Size: tc.size, Offset: tc.offset})
		require.NoError(t, err)
		require.Equal(t, int64(3), total)
		require.Len(t, items, tc.count)
		if len(items) > 0 {
			require.Equal(t, []string{ids[1], ids[2], ids[0]}[tc.offset], items[0].ID)
		}
	}
	items, total, err := repo.List(ctx, uuid.NewString(), domain.Page{Size: 20})
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, total)
	earlier := now.Add(-time.Hour)
	first, err := repo.Create(ctx, domain.Message{ID: uuid.NewString(), ApplicationID: application, SenderID: employer, RecipientID: applicant, Content: "earlier", CreatedAt: earlier, UpdatedAt: earlier})
	require.NoError(t, err)
	ordered, _, err := repo.List(ctx, application, domain.Page{Size: 20})
	require.NoError(t, err)
	require.Equal(t, first.ID, ordered[0].ID)
	_, err = repo.Create(ctx, first)
	require.Error(t, err)
	for _, tc := range []struct {
		name, content string
		valid         bool
	}{{"empty", "", false}, {"one", "x", true}, {"2000", strings.Repeat("界", 2000), true}, {"2001", strings.Repeat("界", 2001), false}} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := repo.Create(ctx, domain.Message{ID: uuid.NewString(), ApplicationID: application, SenderID: employer, RecipientID: applicant, Content: tc.content, CreatedAt: now.Add(time.Second), UpdatedAt: now.Add(time.Second)})
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
	_, err = repo.Create(ctx, domain.Message{ID: uuid.NewString(), ApplicationID: uuid.NewString(), SenderID: applicant, RecipientID: employer, Content: "hello", CreatedAt: now, UpdatedAt: now})
	require.Error(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO application_participants VALUES($1,$2,$3)`, application, applicant, employer)
	require.Error(t, err)
	pool.Close()
	_, _, err = repo.List(ctx, application, domain.Page{Size: 20})
	require.EqualError(t, err, "messaging database operation failed")
}
