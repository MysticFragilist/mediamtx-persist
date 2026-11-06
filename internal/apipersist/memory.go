package apipersist

import (
	"context"
	"maps"
	"sync"

	"github.com/bluenviron/mediamtx/internal/conf"
)

// MemoryDB is an in-memory database that survives the stores opened on it.
// It is meant for tests.
type MemoryDB struct {
	mutex sync.Mutex
	rows  map[string][]byte

	// FailWrites makes Upsert and Delete return an error.
	FailWrites error
}

// Open returns a store backed by the database.
func (db *MemoryDB) Open() Store {
	return &memoryStore{db: db}
}

// Rows returns the raw JSON of the stored paths.
func (db *MemoryDB) Rows() map[string][]byte {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	ret := make(map[string][]byte)
	maps.Copy(ret, db.rows)
	return ret
}

type memoryStore struct {
	db *MemoryDB
}

func (s *memoryStore) Load(_ context.Context) (map[string]*conf.OptionalPath, error) {
	s.db.mutex.Lock()
	defer s.db.mutex.Unlock()

	ret := make(map[string]*conf.OptionalPath)
	for name, raw := range s.db.rows {
		p, err := decodePath(raw)
		if err != nil {
			return nil, err
		}
		ret[name] = p
	}
	return ret, nil
}

func (s *memoryStore) Upsert(_ context.Context, name string, p *conf.OptionalPath) error {
	raw, err := encodePath(p)
	if err != nil {
		return err
	}

	s.db.mutex.Lock()
	defer s.db.mutex.Unlock()

	if s.db.FailWrites != nil {
		return s.db.FailWrites
	}

	if s.db.rows == nil {
		s.db.rows = make(map[string][]byte)
	}
	s.db.rows[name] = raw
	return nil
}

func (s *memoryStore) Delete(_ context.Context, name string) error {
	s.db.mutex.Lock()
	defer s.db.mutex.Unlock()

	if s.db.FailWrites != nil {
		return s.db.FailWrites
	}

	delete(s.db.rows, name)
	return nil
}

func (s *memoryStore) Close() {}
