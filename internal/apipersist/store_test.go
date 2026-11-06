package apipersist

import (
	"context"
	"os"
	"testing"

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

func testStore(t *testing.T, open func() Store) {
	t.Helper()

	ctx := context.Background()

	s := open()

	defer func() {
		paths, err := s.Load(ctx)
		require.NoError(t, err)
		for name := range paths {
			require.NoError(t, s.Delete(ctx, name))
		}
		s.Close()
	}()

	paths, err := s.Load(ctx)
	require.NoError(t, err)
	require.Empty(t, paths)

	require.NoError(t, s.Upsert(ctx, "cam1", mustPath(t, `{"source":"rtsp://host1/stream"}`)))
	require.NoError(t, s.Upsert(ctx, "cam2", mustPath(t, `{"source":"rtsp://host2/stream"}`)))

	// update
	require.NoError(t, s.Upsert(ctx, "cam2", mustPath(t, `{"source":"rtsp://host2/stream","sourceOnDemand":true}`)))

	paths, err = s.Load(ctx)
	require.NoError(t, err)
	require.Len(t, paths, 2)
	require.JSONEq(t, `{"source":"rtsp://host1/stream"}`, pathJSON(t, paths["cam1"]))
	require.JSONEq(t, `{"source":"rtsp://host2/stream","sourceOnDemand":true}`, pathJSON(t, paths["cam2"]))

	// delete, including a missing path
	require.NoError(t, s.Delete(ctx, "cam1"))
	require.NoError(t, s.Delete(ctx, "missing"))

	// data survives reopening the store
	s2 := open()
	defer s2.Close()

	paths, err = s2.Load(ctx)
	require.NoError(t, err)
	require.Len(t, paths, 1)
	require.Contains(t, paths, "cam2")
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

	testStore(t, func() Store {
		s, err := New(context.Background(), "postgres", dsn)
		require.NoError(t, err)
		return s
	})
}

func TestNewUnsupportedDriver(t *testing.T) {
	_, err := New(context.Background(), "mysql", "dsn")
	require.EqualError(t, err, `unsupported persistence driver "mysql"`)
}
