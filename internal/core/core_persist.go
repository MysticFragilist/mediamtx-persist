package core

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/bluenviron/mediamtx/internal/apipersist"
	"github.com/bluenviron/mediamtx/internal/conf"
	"github.com/bluenviron/mediamtx/internal/logger"
)

const persistOpTimeout = 10 * time.Second

// newPersistStore opens a persistence store. It can be overridden in tests.
var newPersistStore = func(ctx context.Context, c *conf.Conf, serverID string) (apipersist.Store, error) {
	return apipersist.New(ctx, c.APIPersistDriver, c.APIPersistConnection, serverID)
}

type persistKey struct {
	driver       string
	connection   string
	serverIDFile string
}

func persistKeyOf(c *conf.Conf) persistKey {
	return persistKey{
		driver:       c.APIPersistDriver,
		connection:   c.APIPersistConnection,
		serverIDFile: c.APIPersistServerIDFile,
	}
}

func (p *Core) closePersistStore() {
	if p.persistStore != nil {
		p.persistStore.Close()
		p.persistStore = nil
	}
}

// ensurePersistStore makes sure that the persistence store reflects the given configuration,
// opening, closing or reopening it when needed.
func (p *Core) ensurePersistStore(c *conf.Conf) error {
	if !c.APIPersist {
		if p.persistStore != nil {
			p.Log(logger.Info, "API persistence disabled")
		}
		p.closePersistStore()
		return nil
	}

	key := persistKeyOf(c)
	if p.persistStore != nil && key == p.persistKey {
		return nil
	}

	p.closePersistStore()

	serverIDFile := apipersist.ServerIDFilePath(c.APIPersistServerIDFile, p.confPath)
	serverID, err := apipersist.LoadOrCreateServerID(serverIDFile)
	if err != nil {
		return fmt.Errorf("API persistence: %w", err)
	}

	ctx, cancel := context.WithTimeout(p.ctx, persistOpTimeout)
	defer cancel()

	store, err := newPersistStore(ctx, c, serverID)
	if err != nil {
		return fmt.Errorf("API persistence: %w", err)
	}

	p.persistStore = store
	p.persistKey = key

	p.Log(logger.Info, "API persistence enabled (driver %s, server ID %s, ID file '%s')",
		c.APIPersistDriver, serverID, serverIDFile)

	return nil
}

// mergePersistedPaths returns a copy of c that contains the paths stored in the persistence store.
// Paths that are already defined in c (i.e. in the configuration file) take precedence.
func (p *Core) mergePersistedPaths(c *conf.Conf) (*conf.Conf, error) {
	if p.persistStore == nil {
		return c, nil
	}

	ctx, cancel := context.WithTimeout(p.ctx, persistOpTimeout)
	defer cancel()

	stored, err := p.persistStore.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("API persistence: unable to load paths: %w", err)
	}

	names := make([]string, 0, len(stored))
	for name := range stored {
		if _, ok := c.OptionalPaths[name]; ok {
			p.Log(logger.Warn, "API persistence: path '%s' is defined in the configuration file, "+
				"ignoring the persisted version", name)
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	if len(names) == 0 {
		return c, nil
	}

	// fast path: add all paths and validate once (AddPath validates the whole configuration).
	merged := c.Clone()
	if merged.OptionalPaths == nil {
		merged.OptionalPaths = make(map[string]*conf.OptionalPath)
	}
	last := names[len(names)-1]
	for _, name := range names[:len(names)-1] {
		merged.OptionalPaths[name] = stored[name]
	}
	err = merged.AddPath(last, stored[last])
	// slow path: add paths one by one, skipping invalid ones.
	if err != nil {
		merged = c.Clone()
		for _, name := range names {
			tmp := merged.Clone()
			err = tmp.AddPath(name, stored[name])
			if err != nil {
				p.Log(logger.Warn, "API persistence: skipping invalid persisted path '%s': %v", name, err)
				continue
			}
			merged = tmp
		}
	}

	restored := len(merged.OptionalPaths) - len(c.OptionalPaths)
	p.Log(logger.Info, "API persistence: restored %d path(s)", restored)

	return merged, nil
}

// persistPath saves the state of a path after an API request.
func (p *Core) persistPath(name string, newConf *conf.Conf) error {
	if p.persistStore == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(p.ctx, persistOpTimeout)
	defer cancel()

	var err error
	if pathConf, ok := newConf.OptionalPaths[name]; ok {
		err = p.persistStore.Upsert(ctx, name, pathConf)
	} else {
		err = p.persistStore.Delete(ctx, name)
	}
	if err != nil {
		return fmt.Errorf("%w: path '%s': %w", apipersist.ErrWrite, name, err)
	}

	return nil
}
