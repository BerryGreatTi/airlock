# ADR-0008: Multi-Format Secret File Management

## Status
Accepted

## Context
Airlock originally supported only `.env` files for secret encryption (via `EnvScanner`). Users need to encrypt secrets in JSON (GCP service account keys), YAML (Kubernetes secrets, Helm values), INI (AWS credentials), Properties (Java configs), and plain text (PEM keys, certificates).

## Decision

### Format support
Support 6 formats via a `FileParser` interface with per-format implementations: dotenv, JSON, YAML, INI, properties, plain text (whole-file). Format detection is extension-based (`.json` -> JSON, `.yaml`/`.yml` -> YAML, etc.) with explicit `--format` override.

### Key path notation
Use `/` as the key path separator for nested structures: `db/password`, `servers/0/host`. This avoids escaping issues with literal dots in key names (e.g., `spring.datasource.password` is a single key, not nested).

### Selective encryption
Users choose which keys to encrypt via `--keys k1,k2`, `--all`, or `--auto` (heuristic). The `EncryptSelected()` function handles per-key encryption. Selected keys are persisted in `.airlock/config.yaml` as `encrypt_keys`.

### Single source of truth
`.airlock/config.yaml` stores the list of registered secret files (`secret_files` field). The GUI reads/writes via CLI commands -- it does not maintain its own registry.

### Atomic writes
All `Write` methods use temp-file-then-rename to prevent data loss on crash during in-place encryption/decryption.

### Proxy unchanged
The proxy's `mapping.json` format and `ENC[age:...]` string replacement are format-agnostic. No proxy changes needed.

## Alternatives Considered

- **SOPS integration**: Would provide proven multi-format support but adds a heavyweight dependency and different encryption format.
- **Dot notation for paths**: Simpler but collides with literal dots in key names (e.g., Java properties).
- **GUI-side file registry**: Would duplicate state between Swift persistence and `.airlock/config.yaml`. Rejected per D1.

## Consequences
- 6 new parsers + `FileScanner` in `internal/secrets/`
- 7 new CLI commands under `airlock secret`
- GUI Secrets tab redesigned with file sidebar + multi-select table
- New dependency: `gopkg.in/ini.v1`
- `EnvScanner` preserved for `--env` flag backward compatibility
