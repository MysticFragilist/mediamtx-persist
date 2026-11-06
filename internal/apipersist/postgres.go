package apipersist

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	// register the pgx database/sql driver.
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/bluenviron/mediamtx/internal/conf"
)

const postgresTable = "mediamtx_paths"

const postgresConnectTimeout = 10 * time.Second

type postgres struct {
	db *sql.DB
}

func newPostgres(ctx context.Context, connection string) (*postgres, error) {
	db, err := sql.Open("pgx", connection)
	if err != nil {
		return nil, err
	}

	ctx2, cancel := context.WithTimeout(ctx, postgresConnectTimeout)
	defer cancel()

	err = db.PingContext(ctx2)
	if err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("unable to connect to the persistence database: %w", err)
	}

	_, err = db.ExecContext(ctx2, `CREATE TABLE IF NOT EXISTS `+postgresTable+` (
		name       TEXT        PRIMARY KEY,
		conf       JSONB       NOT NULL,
		updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`)
	if err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("unable to initialize the persistence database: %w", err)
	}

	return &postgres{db: db}, nil
}

func (s *postgres) Load(ctx context.Context) (map[string]*conf.OptionalPath, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, conf FROM `+postgresTable)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck

	ret := make(map[string]*conf.OptionalPath)

	for rows.Next() {
		var name string
		var raw []byte
		err = rows.Scan(&name, &raw)
		if err != nil {
			return nil, err
		}

		var p *conf.OptionalPath
		p, err = decodePath(raw)
		if err != nil {
			return nil, fmt.Errorf("unable to decode stored path %q: %w", name, err)
		}

		ret[name] = p
	}

	return ret, rows.Err()
}

func (s *postgres) Upsert(ctx context.Context, name string, p *conf.OptionalPath) error {
	raw, err := encodePath(p)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx,
		`INSERT INTO `+postgresTable+` (name, conf, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (name) DO UPDATE SET conf = EXCLUDED.conf, updated_at = now()`,
		name, string(raw))
	return err
}

func (s *postgres) Delete(ctx context.Context, name string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM `+postgresTable+` WHERE name = $1`, name)
	return err
}

func (s *postgres) Close() {
	s.db.Close() //nolint:errcheck
}
