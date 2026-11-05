package apipersist

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// DefaultServerIDFileName is the name of the server ID file when no path is specified.
const DefaultServerIDFileName = "mediamtx-server-id"

// ServerIDFilePath returns the path of the server ID file.
// When fpath is empty, the file is placed next to the configuration file,
// or in the working directory if there's no configuration file.
func ServerIDFilePath(fpath string, confPath string) string {
	if fpath != "" {
		return fpath
	}
	if confPath != "" {
		return filepath.Join(filepath.Dir(confPath), DefaultServerIDFileName)
	}
	return DefaultServerIDFileName
}

// LoadOrCreateServerID reads the server ID from a file.
// If the file does not exist, a new random ID is generated and saved.
func LoadOrCreateServerID(fpath string) (string, error) {
	byts, err := os.ReadFile(fpath)
	if err == nil {
		id, err2 := uuid.Parse(strings.TrimSpace(string(byts)))
		if err2 != nil {
			return "", fmt.Errorf("invalid server ID in '%s': %w", fpath, err2)
		}
		return id.String(), nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	dir := filepath.Dir(fpath)
	if dir != "" {
		err = os.MkdirAll(dir, 0o755)
		if err != nil {
			return "", err
		}
	}

	id := uuid.New().String()

	err = os.WriteFile(fpath, []byte(id+"\n"), 0o600)
	if err != nil {
		return "", fmt.Errorf("unable to save server ID in '%s': %w", fpath, err)
	}

	return id, nil
}
