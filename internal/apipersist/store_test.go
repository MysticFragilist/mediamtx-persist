package apipersist

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/bluenviron/mediamtx/internal/conf"
)

func mustPath(t *testing.T, js string) *conf.OptionalPath {
	t.Helper()
	var p conf.OptionalPath
	require.NoError(t, p.UnmarshalJSON([]byte(js)))
	return &p
}

func pathJSON(t *testing.T, p *conf.OptionalPath) string {
	t.Helper()
	byts, err := p.MarshalJSON()
	require.NoError(t, err)
	return string(byts)
}

func testStore(t *testing.T, open func(serverID string) Store) {
	t.Helper()

	ctx := context.Background()

	serverA := uuid.New().String()
	serverB := uuid.New().String()

	a := open(serverA)
	defer a.Close()
	b := open(serverB)
	defer b.Close()

	defer func() {
		for _, s := range []Store{a, b} {
			paths, err := s.Load(ctx)
			require.NoError(t, err)
			for name := range paths {
				require.NoError(t, s.Delete(ctx, name))
			}
		}
	}()

	require.Equal(t, serverA, a.ServerID())

	paths, err := a.Load(ctx)
	require.NoError(t, err)
	require.Empty(t, paths)

	require.NoError(t, a.Upsert(ctx, "cam1", mustPath(t, `{"source":"rtsp://host1/stream"}`)))
	require.NoError(t, a.Upsert(ctx, "cam2", mustPath(t, `{"source":"rtsp://host2/stream"}`)))
	require.NoError(t, b.Upsert(ctx, "cam1", mustPath(t, `{"source":"rtsp://other/stream"}`)))

	// update
	require.NoError(t, a.Upsert(ctx, "cam2", mustPath(t, `{"source":"rtsp://host2/stream","sourceOnDemand":true}`)))

	paths, err = a.Load(ctx)
	require.NoError(t, err)
	require.Len(t, paths, 2)
	require.JSONEq(t, `{"source":"rtsp://host1/stream"}`, pathJSON(t, paths["cam1"]))
	require.JSONEq(t, `{"source":"rtsp://host2/stream","sourceOnDemand":true}`, pathJSON(t, paths["cam2"]))

	// delete, including a missing path
	require.NoError(t, a.Delete(ctx, "cam1"))
	require.NoError(t, a.Delete(ctx, "missing"))

	paths, err = a.Load(ctx)
	require.NoError(t, err)
	require.Len(t, paths, 1)
	require.Contains(t, paths, "cam2")

	// other server is isolated
	paths, err = b.Load(ctx)
	require.NoError(t, err)
	require.Len(t, paths, 1)
	require.JSONEq(t, `{"source":"rtsp://other/stream"}`, pathJSON(t, paths["cam1"]))
}

func TestMemoryStore(t *testing.T) {
	db := &MemoryDB{}
	testStore(t, db.Open)
}

// TestPostgresStore requires a PostgreSQL server.
// Set MTX_TEST_POSTGRES to a connection string to run it, for instance:
// MTX_TEST_POSTGRES=postgres://postgres:pw@localhost:5432/postgres?sslmode=disable
func TestPostgresStore(t *testing.T) {
	dsn := os.Getenv("MTX_TEST_POSTGRES")
	if dsn == "" {
		t.Skip("MTX_TEST_POSTGRES not set")
	}

	testStore(t, func(serverID string) Store {
		s, err := New(context.Background(), "postgres", dsn, serverID)
		require.NoError(t, err)
		return s
	})
}

func TestNewUnsupportedDriver(t *testing.T) {
	_, err := New(context.Background(), "mysql", "dsn", "id")
	require.EqualError(t, err, `unsupported persistence driver "mysql"`)
}
