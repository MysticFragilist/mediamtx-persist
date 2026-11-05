// Package apipersist contains storage backends that persist paths created through the Control API.
package apipersist

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluenviron/mediamtx/internal/conf"
)

// Store persists path configurations created through the Control API.
// Every row is scoped by the server ID the store was opened with,
// so that multiple servers can share the same database.
type Store interface {
	// ServerID returns the ID of the server this store is scoped to.
	ServerID() string

	// Load returns all paths stored for the server.
	Load(ctx context.Context) (map[string]*conf.OptionalPath, error)

	// Upsert inserts or replaces a path.
	Upsert(ctx context.Context, name string, p *conf.OptionalPath) error

	// Delete removes a path. Deleting a path that is not stored is not an error.
	Delete(ctx context.Context, name string) error

	// Close closes the store.
	Close()
}

// New opens a store with the given driver.
func New(ctx context.Context, driver string, connection string, serverID string) (Store, error) {
	switch driver {
	case "postgres":
		return newPostgres(ctx, connection, serverID)

	default:
		return nil, fmt.Errorf("unsupported persistence driver %q", driver)
	}
}

func encodePath(p *conf.OptionalPath) ([]byte, error) {
	return p.MarshalJSON()
}

func decodePath(b []byte) (*conf.OptionalPath, error) {
	var p conf.OptionalPath
	err := p.UnmarshalJSON(b)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ErrWrite is returned (wrapped) when a change cannot be saved into the store.
var ErrWrite = errors.New("unable to save into the persistence database")
