package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/bluenviron/mediamtx/internal/apipersist"
	"github.com/bluenviron/mediamtx/internal/conf"
)

func useMemoryPersist(t *testing.T, db *apipersist.MemoryDB) {
	t.Helper()

	prev := newPersistStore
	newPersistStore = func(_ context.Context, _ *conf.Conf) (apipersist.Store, error) {
		return db.Open(), nil
	}
	t.Cleanup(func() { newPersistStore = prev })
}

func persistConf(extra string) string {
	return "api: yes\n" +
		"rtsp: no\n" +
		"rtmp: no\n" +
		"hls: no\n" +
		"webrtc: no\n" +
		"srt: no\n" +
		"moq: no\n" +
		"apiPersist: yes\n" +
		"apiPersistConnection: memory\n" +
		extra
}

func persistRequest(t *testing.T, hc *http.Client, method string, ur string, body any) int {
	t.Helper()

	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}

	req, err := http.NewRequest(method, ur, &buf)
	require.NoError(t, err)

	res, err := hc.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()

	return res.StatusCode
}

// persistListPaths returns the configured paths, as name -> source.
func persistListPaths(t *testing.T, hc *http.Client) map[string]string {
	t.Helper()

	var out struct {
		Items []struct {
			Name           string `json:"name"`
			Source         string `json:"source"`
			SourceOnDemand bool   `json:"sourceOnDemand"`
		} `json:"items"`
	}
	httpRequest(t, hc, http.MethodGet, "http://localhost:9997/v3/config/paths/list", nil, &out)

	ret := make(map[string]string)
	for _, item := range out.Items {
		v := item.Source
		if item.SourceOnDemand {
			v += " (on demand)"
		}
		ret[item.Name] = v
	}
	return ret
}

func runPersistInstance(t *testing.T, cnf string, cb func(hc *http.Client)) {
	t.Helper()

	p, ok := newInstance(t, cnf)
	require.Equal(t, true, ok)
	defer p.Close()

	tr := &http.Transport{}
	defer tr.CloseIdleConnections()
	hc := &http.Client{Transport: tr, Timeout: 10 * time.Second}

	cb(hc)
}

func TestAPIPersistRestore(t *testing.T) {
	db := &apipersist.MemoryDB{}
	useMemoryPersist(t, db)

	cnf := persistConf("")

	// first run: create three cameras, patch one, replace one, delete one.
	runPersistInstance(t, cnf, func(hc *http.Client) {
		for _, name := range []string{"cam1", "cam2", "cam3", "cam4"} {
			require.Equal(t, http.StatusOK, persistRequest(t, hc, http.MethodPost,
				"http://localhost:9997/v3/config/paths/add/"+name,
				map[string]any{"source": "rtsp://" + name + ".local/stream"}))
		}

		require.Equal(t, http.StatusOK, persistRequest(t, hc, http.MethodPatch,
			"http://localhost:9997/v3/config/paths/patch/cam2",
			map[string]any{"sourceOnDemand": true}))

		require.Equal(t, http.StatusOK, persistRequest(t, hc, http.MethodPost,
			"http://localhost:9997/v3/config/paths/replace/cam3",
			map[string]any{"source": "rtsp://replaced.local/stream"}))

		require.Equal(t, http.StatusOK, persistRequest(t, hc, http.MethodDelete,
			"http://localhost:9997/v3/config/paths/delete/cam4", nil))
	})

	require.Len(t, db.Rows(), 3)

	expected := map[string]string{
		"cam1": "rtsp://cam1.local/stream",
		"cam2": "rtsp://cam2.local/stream (on demand)",
		"cam3": "rtsp://replaced.local/stream",
	}

	// second run (simulated restart): paths are restored.
	runPersistInstance(t, cnf, func(hc *http.Client) {
		require.Equal(t, expected, persistListPaths(t, hc))
	})

	// restored paths can still be changed, and changes are persisted.
	runPersistInstance(t, cnf, func(hc *http.Client) {
		require.Equal(t, http.StatusOK, persistRequest(t, hc, http.MethodDelete,
			"http://localhost:9997/v3/config/paths/delete/cam1", nil))
	})
	require.Len(t, db.Rows(), 2)
}

func TestAPIPersistConfigFileWins(t *testing.T) {
	db := &apipersist.MemoryDB{}
	useMemoryPersist(t, db)

	runPersistInstance(t, persistConf(""), func(hc *http.Client) {
		for _, name := range []string{"cam1", "cam2"} {
			require.Equal(t, http.StatusOK, persistRequest(t, hc, http.MethodPost,
				"http://localhost:9997/v3/config/paths/add/"+name,
				map[string]any{"source": "rtsp://api.local/" + name}))
		}
	})

	runPersistInstance(t, persistConf("paths:\n"+
		"  cam1:\n"+
		"    source: rtsp://yaml.local/cam1\n"), func(hc *http.Client) {
		require.Equal(t, map[string]string{
			"cam1": "rtsp://yaml.local/cam1",
			"cam2": "rtsp://api.local/cam2",
		}, persistListPaths(t, hc))
	})
}

func TestAPIPersistWriteError(t *testing.T) {
	db := &apipersist.MemoryDB{FailWrites: errors.New("database down")}
	useMemoryPersist(t, db)

	runPersistInstance(t, persistConf(""), func(hc *http.Client) {
		require.Equal(t, http.StatusInternalServerError, persistRequest(t, hc, http.MethodPost,
			"http://localhost:9997/v3/config/paths/add/cam1",
			map[string]any{"source": "rtsp://cam1.local/stream"}))

		// the path must not be applied, to keep memory and database consistent.
		require.Empty(t, persistListPaths(t, hc))
	})
}

func TestAPIPersistFileReload(t *testing.T) {
	db := &apipersist.MemoryDB{}
	useMemoryPersist(t, db)

	dir := t.TempDir()
	confPath := filepath.Join(dir, "mediamtx.yml")

	require.NoError(t, os.WriteFile(confPath, []byte(persistConf("")), 0o644))

	p, ok := New([]string{confPath})
	require.Equal(t, true, ok)
	defer p.Close()

	tr := &http.Transport{}
	defer tr.CloseIdleConnections()
	hc := &http.Client{Transport: tr, Timeout: 10 * time.Second}

	require.Equal(t, http.StatusOK, persistRequest(t, hc, http.MethodPost,
		"http://localhost:9997/v3/config/paths/add/cam1",
		map[string]any{"source": "rtsp://cam1.local/stream"}))

	// edit the configuration file: the API path must survive the reload.
	require.NoError(t, os.WriteFile(confPath, []byte(persistConf("paths:\n"+
		"  yamlcam:\n"+
		"    source: rtsp://yaml.local/stream\n")), 0o644))

	require.Eventually(t, func() bool {
		res, err := hc.Get("http://localhost:9997/v3/config/paths/get/yamlcam")
		if err != nil {
			return false
		}
		res.Body.Close()
		return res.StatusCode == http.StatusOK
	}, 10*time.Second, 100*time.Millisecond)

	require.Equal(t, map[string]string{
		"cam1":    "rtsp://cam1.local/stream",
		"yamlcam": "rtsp://yaml.local/stream",
	}, persistListPaths(t, hc))
}
