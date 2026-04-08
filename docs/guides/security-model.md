# Security Model

## Threat Model

Airlock protects against a specific threat: **an AI coding agent accidentally or maliciously leaking secrets that it has access to.** The agent is not inherently malicious, but it operates autonomously and may:

- Include API keys in prompts sent to LLM providers
- Hardcode credentials in generated source code
- Push secrets to public repositories
- Write secrets to log files or terminal output

Airlock assumes the host machine is trusted. The age private key resides on the host and is never exposed to the container.

## Defense Layers

### Layer 1: Container Isolation

The agent runs inside a Docker container with minimal privileges:
- `--cap-drop=ALL` removes all Linux capabilities
- Each workspace directory is mounted read-write at `/workspace/<project-name>` (using the directory basename). This gives each workspace a distinct path inside the container, so Claude Code maintains separate project histories in the shared volume.
- `~/.claude/` state is stored in a persistent Docker named volume (`airlock-claude-home`) mounted read-write at `/home/airlock/.claude`. This enables OAuth persistence, session history, and cross-workspace context. The volume is independent from the host's `~/.claude` (see [ADR-0006](../decisions/ADR-0006-writable-claude-volume.md)).
- Sensitive files (`.env`, `settings.json`) are shadow-mounted with encrypted versions. Shadow mounts (file-level bind mounts) take precedence over the volume mount, so the agent reads only `ENC[age:...]` ciphertext at those specific paths even though the underlying volume is writable.
- No direct network access (internal Docker network)
- The `airlock` container user runs as UID 1001 / GID 1001, explicitly pinned in the Dockerfile for volume ownership consistency.

**Blast radius:** Damage is limited to the mounted workspace directory and the persistent `.claude` volume. The volume stores only Claude Code state (history, sessions, OAuth tokens) -- no host filesystem access beyond the workspace.

**Shadow mount security on writable volume:** Shadow mounts only overlay the specific files that were scanned for secrets (`settings.json`, `settings.local.json`). Other files written to the volume by the agent (e.g., session data, history) are not shadowed. See [glossary/shadow-mount](../glossary/shadow-mount.md) for details.

### Layer 2: Secret Encryption

All secrets are encrypted with [age](https://age-encryption.org/) (X25519) before entering the container. A modular Scanner pipeline discovers and encrypts secrets across multiple config formats:

| Source | Scanner | Scope |
|--------|---------|-------|
| `.env` files (via `--env` flag) | `EnvScanner` | All values encrypted unconditionally |
| Registered files in `.airlock/config.yaml:secret_files[]` | `FileScanner` | Multi-format (dotenv, JSON, YAML, INI, properties, text); selective or whole-file |
| `.claude/settings.json` | `ClaudeScanner` | Heuristic detection (key name + value prefix) |
| `.claude/settings.local.json` | `ClaudeScanner` | Same heuristic |
| Env secrets in `.airlock/config.yaml:env_secrets[]` | `EnvSecretScanner` | Injected as `NAME=ENC[age:...]` into the agent container's environment |

```
Host settings.json:  "SLACK_TOKEN": "xoxb-1234-abcdef"
Container:           "SLACK_TOKEN": "ENC[age:YWdlLWVuY3J5cHRpb24...]"
```

The agent sees only `ENC[age:...]` ciphertext. Even if it copies the value into source code or sends it to an LLM, the actual secret remains protected.

**Heuristic detection** identifies secrets by key name (contains `token`, `key`, `secret`, `password`, `credential`, `auth`) and value prefix (known patterns like `sk-`, `xoxb-`, `ghp_`, `AKIA`, `eyJ`). Non-secret values (feature flags, paths, URLs) are left as plaintext. See [ADR-0005](../decisions/ADR-0005-settings-secret-protection.md) for details.

**Key management:**
- Private key: `.airlock/keys/age.key` (host only, 0600 permissions, gitignored)
- Public key: `.airlock/keys/age.pub` (can be shared)
- Mapping file: Temporary, 0600 permissions, only in proxy container, deleted on session end

### Layer 3: Transparent Decryption Proxy

A mitmproxy sidecar intercepts outbound HTTP/HTTPS traffic from the agent container:

1. Agent makes an API call with `Authorization: Bearer ENC[age:...]`
2. Proxy finds the `ENC[age:...]` pattern in request headers/body/query params
3. Proxy replaces it with the decrypted plaintext value
4. Request reaches the external API with real credentials

**Passthrough behavior:** The GUI defaults to passthrough for Anthropic API hosts (`api.anthropic.com`, `auth.anthropic.com`) so that `ENC[age:...]` secrets in Claude Code traffic remain encrypted end-to-end. The CLI defaults to no passthrough (all traffic decrypted). Users can configure passthrough for additional hosts via `--passthrough-hosts` CLI flag, `passthrough_hosts` in `config.yaml`, or the GUI Settings panel.

**Passthrough guardrail (GUI):** Removing `api.anthropic.com` or `auth.anthropic.com` from passthrough subverts the privacy property — the proxy then substitutes ciphertext to plaintext on outbound Anthropic traffic, and Anthropic receives plaintext secrets in the conversation body. The GUI surfaces this as a non-blocking guardrail: an inline yellow warning appears live in the Settings and WorkspaceSettings passthrough editors, a destructive-styled confirmation alert fires on Save, and a persistent banner sits at the top of the Secrets tab whenever the resolved passthrough list is missing a protected host. The CLI does not guard this path; `airlock run --passthrough-hosts ""` is an intentional power-user override and the CLI already echoes the resolved list on session start. See [ADR-0010](../decisions/ADR-0010-environment-variable-secrets.md).

**Two Settings layers:** The GUI has two distinct places to edit passthrough hosts and they operate at different scopes.

- **Global Settings** (gear icon in the sidebar, or `Airlock → Settings...` menu) is the install-wide default, persisted in `~/Library/Application Support/Airlock/settings.json`. The `Network Defaults` editor here seeds the fall-through value for every workspace that has no per-workspace override.
- **Workspace Settings tab** (Cmd+4 on a selected workspace) is per-workspace, persisted in that workspace's entry in `workspaces.json`. The `Network Overrides` editor here is OPTIONAL — an empty editor means "inherit global," a non-empty editor means "use this exact list for this workspace instead." The caption line reminds the user of the current global value.

At session start, `ResolvedSettings.passthroughHosts = workspace.passthroughHostsOverride ?? global.passthroughHosts`. This two-layer model is subtle; if you are editing passthrough and not seeing the change take effect, confirm whether you are editing the global defaults or the workspace override.

**Response audit logging:** The proxy logs response metadata (status code, content type, size) for all traffic. Response body content is never logged.

### Layer 4: MCP Server Allow-List

A per-workspace allow-list restricts which MCP servers from `~/.claude/settings.json` are exposed to the agent container. Filtering happens at scan time in `ClaudeScanner.processFile`: entries in `mcpServers` whose names are not on the allow-list are removed from the in-memory JSON before the shadow mount is written. Their secrets are never registered in the proxy mapping, so a disabled MCP cannot leak credentials even if some other code path tries to substitute them later.

The allow-list is tri-state: `nil` = no filtering (default; all MCPs from settings.json are exposed), `[]` = filter out all MCPs, `[..]` = expose only the named entries. The same `global default + per-workspace override` pattern as passthrough hosts applies, with one important difference: empty lists round-trip safely through `config.yaml` because `Config.EnabledMCPServers` intentionally omits the `omitempty` YAML tag.

The GUI exposes this in two places:

- **Global Settings → MCP Servers** seeds the install-wide default. Toggling `Restrict available MCP servers` exposes a checkbox picker populated from `~/.claude/settings.json` via `MCPInventoryService`.
- **Workspace Settings → MCP Servers Override** mirrors the picker per workspace. Enabling the override seeds the selection from the global default; clearing the override falls back to the global setting.

The CLI exposes the same control via `airlock run --enabled-mcps slack,github` and `airlock start --enabled-mcps slack,github`. An empty value with the flag (`--enabled-mcps ""`) explicitly disables all MCPs; omitting the flag preserves the existing behavior.

## What This Protects

| Threat | Protected? | How |
|--------|-----------|-----|
| Secret in LLM prompt | Yes | Agent only has ciphertext |
| Secret in generated code | Yes | Code contains `ENC[age:...]`, not real keys |
| Secret pushed to public repo | Yes | Encrypted values are safe to publish |
| Unauthorized API calls | Partially | Proxy routes all traffic, could add allowlists |
| Untrusted MCP servers | Partially | Per-workspace allow-list filters mcpServers map at scan time (Layer 4) |
| Container breakout | Partially | cap-drop=ALL, but kernel exploits possible |
| Host compromise | No | Private key is on the host |

## Known Limitations

- **Client-side crypto operations** (HMAC signing, AWS Signature V4) require the real key at computation time. The proxy cannot help here since it only replaces values in transit.
- **Non-HTTP protocols** (direct database connections, gRPC without HTTP/2 proxy) are not intercepted.
- **Binary request bodies** are skipped (no UTF-8 decoding attempted).
- **Non-HTTP MCP servers** that use secrets for database connections or local auth receive `ENC[age:...]` values and fail. Only HTTP-based API calls are decrypted by the proxy.
- **Heuristic false negatives** -- secrets with unusual naming or format may not be detected in settings files. Use `.env` files for such cases (all values encrypted unconditionally).
- **In-place encryption is destructive** -- `airlock secret encrypt` modifies files on disk. The plaintext is replaced with `ENC[age:...]` ciphertext. Deleting an encrypted file without first running `airlock secret decrypt` means permanent data loss. The proxy mapping is ephemeral (exists only during a session). Always use version control or backups for files containing encrypted secrets.

## Recommendations for Enterprise Deployment

1. Run Docker with user namespace remapping for additional isolation
2. Use a dedicated Docker network per project (future enhancement)
3. Mount workspace on a tmpfs volume for ephemeral sessions
4. Rotate age keys periodically
5. Monitor proxy logs for unexpected outbound destinations
6. Consider adding a host allowlist to the proxy configuration
