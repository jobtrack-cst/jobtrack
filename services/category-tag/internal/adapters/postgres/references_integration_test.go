//go:build integration

package postgres_test

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/jobtrack-cst/jobtrack/pkg/db"
	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/adapters/postgres"
	"github.com/jobtrack-cst/jobtrack/services/category-tag/internal/domain"
)

// Seeded IDs other services can rely on (JT-39 patterned UUIDs).
const (
	catSoftware = "10000000-0000-4000-8000-000000000001"
	catTourism  = "10000000-0000-4000-8000-000000000012"
	tagGo       = "20000000-0000-4000-8000-000000000001"
	tagGRPC     = "20000000-0000-4000-8000-000000000015"
	locThimphu  = "30000000-0000-4000-8000-000000000014"
	locRemote   = "30000000-0000-4000-8000-000000000021"
	unknownID   = "00000000-0000-4000-8000-000000000000"
)

// startReferenceDB follows the job-posting pattern (JT-38): a real Postgres
// with all migrations applied, run twice to prove they are safe on every startup.
func startReferenceDB(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	container, err := pgcontainer.Run(ctx, "postgres:18-alpine", pgcontainer.WithDatabase("category_tag_test"), pgcontainer.WithUsername("category_tag_test"), pgcontainer.WithPassword("synthetic-test-password"), pgcontainer.BasicWaitStrategies())
	if container != nil {
		t.Cleanup(func() {
			cleanupCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			if err := container.Terminate(cleanupCtx); err != nil {
				t.Error("terminate test PostgreSQL failed")
			}
		})
	}
	if err != nil {
		t.Fatal("start test PostgreSQL failed")
	}
	url, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal("get test connection string failed")
	}
	pool, err := db.OpenPool(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	for range 2 {
		if err := db.Migrate(ctx, pool, filepath.Join("..", "..", "..", "migrations")); err != nil {
			t.Fatal(err)
		}
	}
	return ctx, pool
}

// listAll reads every page and checks total_count on each one.
func listAll(t *testing.T, ctx context.Context, repo *postgres.References, kind domain.Kind, limit int) []domain.Item {
	t.Helper()
	var all []domain.Item
	for offset := 0; ; offset += limit {
		page := domain.Page{Limit: limit, Offset: offset}
		items, total, err := repo.List(ctx, kind, page)
		require.NoError(t, err)
		all = append(all, items...)
		if page.NextPageToken(total) == "" {
			require.Len(t, all, total)
			return all
		}
		require.Len(t, items, limit)
	}
}

func names(items []domain.Item) []string {
	out := make([]string, len(items))
	for i, item := range items {
		out[i] = item.Name
	}
	return out
}

func TestReferences_Integration_SeedDataLoads(t *testing.T) {
	ctx, pool := startReferenceDB(t)
	repo := postgres.NewReferences(pool)

	for _, tt := range []struct {
		kind      domain.Kind
		count     int
		firstName string
		lastName  string
		known     domain.Item
	}{
		{domain.KindCategory, 12, "Business and Finance", "Software Development", domain.Item{ID: catTourism, Name: "Hospitality and Tourism"}},
		{domain.KindTag, 30, "Accounting", "UI Design", domain.Item{ID: tagGRPC, Name: "gRPC"}},
		{domain.KindLocation, 21, "Bumthang", "Zhemgang", domain.Item{ID: locRemote, Name: "Remote"}},
	} {
		t.Run(tt.kind.String(), func(t *testing.T) {
			// A page size of 7 crosses several page boundaries for every list.
			all := listAll(t, ctx, repo, tt.kind, 7)
			require.Len(t, all, tt.count)
			got := names(all)
			assert.Equal(t, tt.firstName, got[0])
			assert.Equal(t, tt.lastName, got[len(got)-1])
			assert.True(t, sort.SliceIsSorted(got, func(i, j int) bool {
				return strings.ToLower(got[i]) < strings.ToLower(got[j])
			}), "names not sorted ignoring case: %v", got)
			assert.Contains(t, all, tt.known)
			for _, item := range all {
				assert.True(t, domain.IsUUID(item.ID), item.ID)
				assert.Equal(t, strings.ToLower(item.ID), item.ID)
				assert.Equal(t, strings.TrimSpace(item.Name), item.Name)
				assert.LessOrEqual(t, len([]rune(item.Name)), domain.MaxNameLength)
			}
		})
	}

	t.Run("gRPC sorts between Go and Java", func(t *testing.T) {
		items, total, err := repo.List(ctx, domain.KindTag, domain.Page{Limit: 3, Offset: 10})
		require.NoError(t, err)
		assert.Equal(t, 30, total)
		assert.Equal(t, []string{"Go", "gRPC", "Java"}, names(items))
	})

	t.Run("offset past the end returns no items but the total", func(t *testing.T) {
		items, total, err := repo.List(ctx, domain.KindLocation, domain.Page{Limit: 20, Offset: 100})
		require.NoError(t, err)
		assert.Empty(t, items)
		assert.NotNil(t, items)
		assert.Equal(t, 21, total)
	})

	t.Run("unknown kind", func(t *testing.T) {
		_, _, err := repo.List(ctx, domain.KindUnspecified, domain.Page{Limit: 20})
		require.Error(t, err)
	})
}

func TestReferences_Integration_Resolve(t *testing.T) {
	ctx, pool := startReferenceDB(t)
	repo := postgres.NewReferences(pool)

	for _, tt := range []struct {
		name string
		kind domain.Kind
		ids  []string
		want []domain.Item
	}{
		{"categories sorted by name, unknown left out", domain.KindCategory, []string{catSoftware, unknownID, catTourism},
			[]domain.Item{{ID: catTourism, Name: "Hospitality and Tourism"}, {ID: catSoftware, Name: "Software Development"}}},
		{"tags", domain.KindTag, []string{tagGRPC, tagGo},
			[]domain.Item{{ID: tagGo, Name: "Go"}, {ID: tagGRPC, Name: "gRPC"}}},
		// A tag ID is not a category: each kind reads only its own table.
		{"ID of another kind is unknown", domain.KindCategory, []string{tagGo, locThimphu}, []domain.Item{}},
		{"only unknown IDs", domain.KindTag, []string{unknownID}, []domain.Item{}},
		{"no IDs", domain.KindTag, nil, []domain.Item{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.Resolve(ctx, tt.kind, tt.ids)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// The checks the PR notes as field limits are enforced by the database,
// because rows only ever come from migrations.
func TestReferences_Integration_ConstraintsEnforceNameRules(t *testing.T) {
	ctx, pool := startReferenceDB(t)
	for _, tt := range []struct {
		name  string
		table string
		value string
		ok    bool
	}{
		{"100 characters", "tags", strings.Repeat("a", 100), true},
		{"101 characters", "tags", strings.Repeat("b", 101), false},
		{"empty", "categories", "", false},
		{"leading space", "locations", " Gelephu", false},
		{"control character", "tags", "Bad\tName", false},
		{"duplicate ignoring case", "locations", "thimphu", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			idColumn := map[string]string{"categories": "category_id", "tags": "tag_id", "locations": "location_id"}[tt.table]
			tx, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = tx.Rollback(ctx) }()
			_, err = tx.Exec(ctx, `INSERT INTO `+tt.table+` (`+idColumn+`, name) VALUES ($1, $2)`, unknownID, tt.value)
			if tt.ok {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}
		})
	}
}
