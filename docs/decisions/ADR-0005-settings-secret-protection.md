# ADR-0005: Settings secret protection with modular Scanner pipeline

## Status

Accepted

## Context

Airlock's original security model encrypted `.env` file secrets and passed them through a transparent decryption proxy. However, a review (2026-03-27) identified three gaps:

1. **Workspace `.env` exposure.** The `.env` file specified via `--env` was encrypted to a temporary `env.enc`, but the original plaintext file remained accessible at `/workspace/.env` because the entire workspace directory is bind-mounted read-write. The agent could bypass encryption by reading the file directly.

2. **Settings file secrets.** `~/.claude/settings.json` and project-level `.claude/settings.json` can contain plaintext API tokens in `env` blocks (both top-level and per-MCP-server). These were mounted into the container with no encryption, exposing tokens for Slack, GitHub, Stripe, and any other MCP server configured with secrets.

3. **Anthropic API passthrough.** The default `passthrough_hosts` list (`api.anthropic.com`, `auth.anthropic.com`) meant Anthropic API traffic was never intercepted by the proxy. If `ANTHROPIC_API_KEY` was stored in a settings file's `env` block, encrypting it would break authentication because the proxy would not decrypt the ciphertext in transit.

4. **No response visibility.** The proxy only logged outbound requests. There was no audit trail for response traffic from external APIs.

These gaps meant the security model had a smaller effective coverage than documented.

## Decision

### 1. Shadow bind mounts for plaintext files

Use Docker file-level bind mounts to overlay plaintext files with their encrypted counterparts. When the agent reads the file, it sees `ENC[age:...]` ciphertext instead of the original plaintext. This applies to `.env` files inside the workspace and to all processed settings files.

### 2. Modular Scanner interface

Introduce a `Scanner` interface (`internal/secrets/scanner.go`) that abstracts secret discovery and encryption per config format. Each format is a separate implementation:

- `EnvScanner` -- handles `.env` files (wraps existing encryption logic)
- `ClaudeScanner` -- handles `.claude/settings.json` and `settings.local.json` (global and project-level)

Future scanners (OpenAI, Slack, etc.) implement the same interface without touching existing code.

`ScanAll` orchestrates all registered scanners, merging their shadow mounts and proxy mappings into a unified result.

### 3. Heuristic secret detection

Rather than encrypting all values (which could cause false-positive replacements for short values like `"1"`), secrets are identified heuristically:

- **Key name signals**: contains `token`, `key`, `secret`, `password`, `credential`, or `auth` (case-insensitive)
- **Value prefix signals**: matches known patterns like `sk-`, `xoxb-`, `ghp_`, `AKIA`, `eyJ`, etc.
- **Exclusions**: values shorter than 8 characters, booleans, file paths, URLs

Conservative approach: when no signal matches, the value is left as plaintext. Users can put ambiguous secrets in `.env` files where all values are encrypted unconditionally.

### 4. Passthrough removal

The default `passthrough_hosts` changes from `["api.anthropic.com", "auth.anthropic.com"]` to `[]` (empty). All outbound traffic now goes through the decryption proxy by default. This enables encrypting `ANTHROPIC_API_KEY` and any other secret in top-level `env` blocks.

A `--passthrough-hosts` CLI flag on `run` and `start` commands allows runtime override for users who want to skip decryption for specific hosts.

### 5. Response audit logging

The proxy now logs response metadata (status code, content type, size) for all traffic including former passthrough hosts. Response body content is never logged.

## Consequences

### Easier

- **Broader secret coverage**: Settings files, `.env` files, and top-level env vars are all protected through a single pipeline
- **Extensibility**: Adding support for new config formats requires only a new `Scanner` implementation
- **Audit trail**: Both request and response traffic are logged, giving enterprise security teams full visibility
- **GUI integration**: `--passthrough-hosts` flag allows the GUI Settings UI to control proxy behavior at runtime

### Harder

- **Non-HTTP MCP servers**: MCP servers that use secrets for non-HTTP operations (database connections, local auth) receive `ENC[age:...]` values and fail. This is a known limitation documented in the security model.
- **Heuristic false negatives**: Secrets with unusual naming or format may not be detected. Users should use `.env` files for such cases.
- **JSON formatting**: Processed settings files may have different whitespace than originals (cosmetic only).

## Alternatives Considered

### Encrypt all env values unconditionally

Simpler but dangerous. Short values like `"1"` or `"oauth2"` would be encrypted, and the proxy would replace any occurrence of these common strings in request bodies, causing false-positive replacements in unrelated traffic.

### Strip env blocks entirely (don't encrypt, just remove)

Simpler but breaks MCP servers. Without env vars, MCP servers that need API tokens would fail to start.

### User-annotated secrets (manual marking)

Most accurate but high friction. Users would need to tag each secret in their settings files. Rejected in favor of the heuristic approach which covers the majority of cases automatically.

### Copy entire ~/.claude/ directory to tmpDir

Would avoid file-level shadow mounts but requires copying the entire directory (potentially large with plugin caches). Shadow mounts are more efficient and more precise.

## Revision (2026-04-02)

The Scanner pipeline has been extended with a third scanner type: `FileScanner`. This scanner handles user-registered secret files in 6 formats (dotenv, JSON, YAML, INI, properties, plain text) configured in `.airlock/config.yaml`. It implements the same `Scanner` interface alongside `EnvScanner` and `ClaudeScanner`. See [ADR-0008](ADR-0008-multi-format-secrets.md) for the full design.
