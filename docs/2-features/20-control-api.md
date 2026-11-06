# Control API

The server can be queried and controlled with an API, that can be enabled by toggling the `api` parameter in the configuration:

```yml
api: yes
```

To obtain a list of active paths, run:

```
curl http://127.0.0.1:9997/v3/paths/list
```

The control API is documented in the [Control API Reference page](../6-references/2-control-api.md) and in the [OpenAPI / Swagger file](https://github.com/bluenviron/mediamtx/blob/{version_tag}/api/openapi.yaml).

Be aware that by default the Control API is accessible by localhost only; to increase visibility or enforce authentication, check [Authentication](05-authentication.md).

## Persisting API changes

By default, paths created, edited or deleted through the Control API (`/v3/config/paths/add`, `patch`, `replace`, `delete`) are kept in memory only, and are lost when the server is restarted. The server can save them into a database and restore them at startup, so that, after a restart or a power cycle, it reconnects to the same sources with the same settings without the need to send the API requests again:

```yml
api: yes
apiPersist: yes
apiPersistDriver: postgres
apiPersistConnection: postgres://user:pass@db.example.com:5432/mediamtx?sslmode=disable
```

The only available driver is `postgres` (PostgreSQL). At startup, the server connects to the database and creates the following table if it does not exist:

```sql
CREATE TABLE mediamtx_paths (
  name       TEXT        PRIMARY KEY,
  conf       JSONB       NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Every API request that changes a path is saved into the database before being applied. If the database cannot be written, the request fails with status code 500 and the change is not applied, so that the running configuration and the database are always consistent. Only the fields that were explicitly set are stored, therefore `pathDefaults` keep applying to restored paths.

A database is meant to be used by a single server. In a [read replicas](18-scalability.md#read-replicas) setup, enable persistence on the origin instance only, which is the one that receives API requests and publishers; read replicas don't need it, since they use a static proxy configuration. Two servers that share the same database overwrite each other's paths: if you run multiple independent servers, give each one its own database.

### Behavior and limitations

- Only paths are persisted. Changes to global settings, path defaults and internal users performed through the API are not persisted.
- The configuration file has priority: if a stored path has the same name as a path defined in the configuration file, the stored version is ignored and a warning is printed. Deleting through the API a path that is defined in the configuration file is not persistent.
- When the configuration file is edited and reloaded, persisted paths are merged again. If the database is not reachable at that moment, the reload is skipped and the current configuration is kept.
- If the database is not reachable at startup, the server exits with an error.
- Path settings, including credentials such as `readPass` and `publishPass`, are stored in plain text in the database: restrict access to it accordingly. The connection string is redacted in the output of `/v3/config/global/get`.
