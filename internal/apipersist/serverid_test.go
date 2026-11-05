package apipersist

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestServerIDFilePath(t *testing.T) {
	require.Equal(t, "/custom/id", ServerIDFilePath("/custom/id", "/etc/mediamtx/mediamtx.yml"))
	require.Equal(t, filepath.Join("/etc/mediamtx", DefaultServerIDFileName),
		ServerIDFilePath("", "/etc/mediamtx/mediamtx.yml"))
	require.Equal(t, DefaultServerIDFileName, ServerIDFilePath("", ""))
}

func TestLoadOrCreateServerID(t *testing.T) {
	fpath := filepath.Join(t.TempDir(), "sub", "server-id")

	id1, err := LoadOrCreateServerID(fpath)
	require.NoError(t, err)
	_, err = uuid.Parse(id1)
	require.NoError(t, err)

	id2, err := LoadOrCreateServerID(fpath)
	require.NoError(t, err)
	require.Equal(t, id1, id2)
}

func TestLoadOrCreateServerIDInvalid(t *testing.T) {
	fpath := filepath.Join(t.TempDir(), "server-id")
	require.NoError(t, os.WriteFile(fpath, []byte("not-a-uuid"), 0o600))

	_, err := LoadOrCreateServerID(fpath)
	require.Error(t, err)
}
