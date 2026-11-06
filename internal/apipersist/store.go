// Package apipersist contains storage backends that persist paths created through the Control API.
package apipersist

import (
	"context"
	"errors"
	"fmt"

	"github.com/bluenviron/mediamtx/internal/conf"
)

// Store persists path configurations created through the Control API.
// A store is meant to be used by a single server (the origin, in a read replicas setup).
type Store interface {
	// Load returns all stored paths.
	Load(ctx context.Context) (map[string]*conf.OptionalPath, error)

	// Upsert inserts or replaces a path.
	Upsert(ctx context.Context, name string, p *conf.OptionalPath) error

	// Delete removes a path. Deleting a path that is not stored is not an error.
	Delete(ctx context.Context, name string) error

	// Close closes the store.
	Close()
}

// New opens a store with the given driver.
func New(ctx context.Context, driver string, connection string) (Store, error) {
	switch driver {
	case "postgres":
		return newPostgres(ctx, connection)

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
