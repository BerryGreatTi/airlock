# ADR-0010: Environment-Variable Secrets and Passthrough Guardrail

## Status
Accepted

## Context

PR #19 (ADR-0008) shipped multi-format file-based secret registration. Some secrets have no natural file home — one-off API tokens, CI variables, values a user wants to set without touching a dotenv. The CLI offered no way to register a single `NAME=value` pair as an Airlock-managed secret.

Separately, the proxy passthrough list is data, not code. The proxy substitutes `ENC[age:...]` patterns with plaintext on any host that is not in the passthrough list. Removing `api.anthropic.com` from that list causes the proxy to begin substituting on outbound Anthropic traffic, which means Anthropic's servers would receive the user's plaintext credentials in the conversation body. This subverts the core "model never sees plaintext" privacy property. Commit `7390166` (2026-04-03) restored the GUI default after a regression; there was no guardrail against future regressions or a user misstep in Settings.

## Decision

### Environment-variable secrets

Store encrypted env vars inline in `.airlock/config.yaml` under a new `env_secrets:` list, parallel to `secret_files:`:

```yaml
env_secrets:
  - name: GITHUB_TOKEN
    value: ENC[age:AQIB...]
```

Encryption happens at `airlock secret env add` time using the workspace age public key, so plaintext never lives at rest. A new `EnvSecretScanner` (peer of `FileScanner`) reads each entry at session start: it decrypts to populate the proxy `mapping.json`, then emits an `EnvVar{Name, Value}` carrying the original ciphertext. The orchestrator passes these through `SessionParams` to the container manager, which appends `NAME=ENC[age:...]` to the agent container's `Env` block. The agent reads the variable from its environment and sees ciphertext; the proxy substitutes back to plaintext on the wire for outbound calls to non-passthrough hosts, identical to the file-secret flow.

Reserved names (`HTTP_PROXY`, `HTTPS_PROXY`, `http_proxy`, `https_proxy`, `NO_PROXY`, `LANG`) are rejected at config load time. Single source of truth is `internal/config/reserved.go`; the validator lives in the same package.

Four CLI subcommands, mirroring the existing `secret` layout:

- `airlock secret env add <NAME>` — reads value from `--value`, `--stdin`, or hidden TTY prompt; encrypts and stores.
- `airlock secret env list [--json]` — names only, sorted. JSON form is the GUI contract.
- `airlock secret env show <NAME> [--json]` — metadata + truncated ciphertext prefix. NEVER decrypts.
- `airlock secret env remove <NAME>` — unregister.

### Passthrough guardrail

Swift-only, non-blocking. A shared `PassthroughPolicy` constant defines `api.anthropic.com` and `auth.anthropic.com` as protected hosts. The Settings, WorkspaceSettings, and Secrets views all consult `PassthroughPolicy.missingProtectedHosts(...)`. When a protected host is missing:

1. An inline yellow warning appears in the editor live as the user types.
2. On Save, a destructive-styled confirmation alert blocks the change until the user clicks "Remove anyway."
3. A persistent yellow banner sits at the top of `SecretsView` whenever the resolved (global + workspace override) list is missing a protected host.

The CLI is unguarded — `airlock run --passthrough-hosts ""` is a power-user path and the CLI already prints the resolved list on startup. The guardrail is for the GUI-clickthrough footgun.

## Consequences

- Plaintext for env secrets never appears at rest. Encryption is performed once, at `add` time, before the value is persisted.
- Inside the agent container, env secrets are visible as `ENC[age:...]` ciphertext to any process that reads the environment. Same threat boundary as any container env var; the model never sees plaintext, Anthropic never sees plaintext.
- The `--value` argv path on `airlock secret env add` briefly exposes plaintext in `ps` output. This is the GUI plumbing path because Swift `Process` stdin piping is awkward. A `--stdin` alternative exists for CLI users who want to avoid argv.
- No re-encryption on key rotation. Inherited limitation; out of scope for this work.
- A determined user can still remove Anthropic from passthrough by clicking through the confirmation alert. The guardrail makes the action conscious, not impossible.
- Schema is forward-compatible: an absent `env_secrets:` field deserializes to nil; old configs continue to load.

## Alternatives Considered

### Separate `.airlock/env.secrets.yaml` file
Rejected. `.airlock/` is gitignored; a separate file is aesthetic, not functional. Inline in `config.yaml:env_secrets[]` follows the `secret_files:` precedent exactly and uses one loader. YAGNI.

### Plaintext at rest, encrypt at session start
Rejected. Violates the existing "encrypted at rest" invariant for file secrets and creates a window where plaintext lives on disk. The key advantage of `.airlock/` being gitignored is that nothing sensitive needs to round-trip through plaintext.

### Stdin-only, no `--value` flag
Rejected. The GUI needs synchronous value passing. Swift `Process` stdin piping in the async wrapper is clunky enough that the argv leakage is the practical trade-off. A future enhancement could wire the GUI into `--stdin` without changing the GUI surface.

### CLI passthrough guardrail
Rejected. Power-user path; the CLI already echoes the resolved passthrough list on session start, and `--passthrough-hosts ""` is an explicit override. Guarding it would be annoying for people running scripts.

### Block passthrough removal entirely (no escape hatch)
Rejected. Testing legitimately requires removing Anthropic from passthrough sometimes (e.g., to observe what the proxy would rewrite). Making it impossible would be hostile to power users. The confirmation alert is the right compromise: conscious, not blocked.

## Related
- ADR-0005 (settings secret protection)
- ADR-0008 (multi-format secrets)
