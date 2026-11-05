package apipersist

import (
	"context"
	"maps"
	"sync"

	"github.com/bluenviron/mediamtx/internal/conf"
)

// MemoryDB is an in-memory database that can be shared by multiple MemoryStores.
// It is meant for tests.
type MemoryDB struct {
	mutex sync.Mutex
	rows  map[string]map[string][]byte

	// FailWrites makes Upsert and Delete return an error.
	FailWrites error
}

// Open returns a store scoped to the given server ID.
func (db *MemoryDB) Open(serverID string) Store {
	return &memoryStore{db: db, serverID: serverID}
}

// Rows returns the raw JSON of the paths stored for a server.
func (db *MemoryDB) Rows(serverID string) map[string][]byte {
	db.mutex.Lock()
	defer db.mutex.Unlock()

	ret := make(map[string][]byte)
	maps.Copy(ret, db.rows[serverID])
	return ret
}

type memoryStore struct {
	db       *MemoryDB
	serverID string
}

func (s *memoryStore) ServerID() string {
	return s.serverID
}

func (s *memoryStore) Load(_ context.Context) (map[string]*conf.OptionalPath, error) {
	s.db.mutex.Lock()
	defer s.db.mutex.Unlock()

	ret := make(map[string]*conf.OptionalPath)
	for name, raw := range s.db.rows[s.serverID] {
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
		s.db.rows = make(map[string]map[string][]byte)
	}
	if s.db.rows[s.serverID] == nil {
		s.db.rows[s.serverID] = make(map[string][]byte)
	}
	s.db.rows[s.serverID][name] = raw
	return nil
}

func (s *memoryStore) Delete(_ context.Context, name string) error {
	s.db.mutex.Lock()
	defer s.db.mutex.Unlock()

	if s.db.FailWrites != nil {
		return s.db.FailWrites
	}

	delete(s.db.rows[s.serverID], name)
	return nil
}

func (s *memoryStore) Close() {}
