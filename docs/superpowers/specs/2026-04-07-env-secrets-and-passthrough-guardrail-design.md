# Environment-variable secrets and passthrough guardrail

**Status:** Design approved, ready for planning
**Date:** 2026-04-07
**Branch:** `feat/env-secrets-and-passthrough-guardrail`
**Related ADRs:** ADR-0005 (settings secret protection), ADR-0008 (multi-format secrets). New ADR-0010 to be added by this work.

## Background

Airlock protects agent-handled secrets with a two-part contract:

1. Inside the agent container, everything the model can read is ciphertext (`ENC[age:...]`). This includes files shadow-mounted by `FileScanner` and heuristic matches rewritten by `ClaudeScanner`. Anthropic's servers, which can see the conversation, therefore see only ciphertext. This property depends on `api.anthropic.com` being in the `passthrough_hosts` list so the proxy does not substitute outbound Anthropic traffic.
2. On outbound HTTP to non-passthrough hosts (the real backends — GitHub, Slack, etc.), the mitmproxy addon substitutes `ENC[age:...]` tokens with plaintext right at the network boundary. The backend authenticates successfully. No host other than the intended backend ever sees plaintext.

Two gaps motivate this work:

**Gap 1 — footgun:** the passthrough list is data, not code. If a user removes `api.anthropic.com` from the list (by editing workspace overrides, or by misunderstanding what passthrough means), the proxy begins substituting ciphertext → plaintext on Anthropic traffic, and Anthropic then receives plaintext secrets in the conversation body. Commit `7390166` (2026-04-03) restored the correct default after a regression; there is still no guardrail preventing a future regression or a user misstep.

**Gap 2 — missing secret source:** since PR #19 introduced multi-format file registration (`airlock secret add <file>`), secrets must have a file home. Some secrets don't — one-off tokens, CI variables, values the user wants to set without touching a dotenv. There is currently no way to register a single `NAME=value` pair as an Airlock-managed secret.

This spec addresses both.

## Goals

- Make it impossible to accidentally remove `api.anthropic.com` / `auth.anthropic.com` from passthrough via the GUI without an explicit acknowledgement.
- Provide a first-class env-var secret source: CLI commands and a GUI UI, ciphertext at rest, ciphertext in the agent container environment, substituted at the proxy boundary identically to file secrets.
- Preserve all existing properties: the model never sees plaintext; Anthropic never sees plaintext; secrets reach real backends as plaintext only on the wire.

## Non-goals

- No change to proxy substitution logic (`proxy/addon/decrypt_proxy.py` is not touched).
- No change to the default passthrough list (already correct).
- No change to `FileScanner`, `ClaudeScanner`, or `EnvScanner` (the dotenv-file one).
- No threat-model change.
- No key rotation / re-encryption tooling (already unhandled repo-wide; out of scope).
- No CLI passthrough guardrail (`airlock run --passthrough-hosts ""` remains unguarded; the footgun we're closing is the GUI path).
- No env-secret value editing, decryption, or export commands (remove + re-add is the edit path).
- No stdin pipe from the GUI (uses `--value` argv; known limitation, documented).

## Architecture model (reference)

```
 Anthropic Cloud                    Real backend (github.com, slack.com, ...)
       ▲                                         ▲
       │ must see ciphertext                     │ must see plaintext
       │ (passthrough = skip substitution)       │ (substitute on wire)
       │                                         │
       └────────────┐                 ┌──────────┘
                    │                 │
                 airlock-proxy (mitmproxy + decrypt addon)
                    ▲
                    │ HTTP_PROXY / HTTPS_PROXY
                    │
                 airlock-claude (agent container)
                    │
                    ├── Files: shadow-mounted ENC[age:...]        (FileScanner)
                    ├── Settings: heuristic ENC[age:...] in JSON  (ClaudeScanner)
                    └── Env vars: injected NAME=ENC[age:...]      (EnvSecretScanner, NEW)
```

`EnvSecretScanner` is a new peer of `FileScanner`. It produces `ScanResult.Env` (new field) which the orchestrator hands to `BuildClaudeConfig`, which appends `NAME=ENC[age:...]` to the container `Env` block. The proxy substitutes these ciphertexts back to plaintext on outbound calls to non-passthrough hosts using the same `mapping.json` it already consumes.

## Section 1 — Passthrough guardrail (GUI-only)

### Protected hosts

Defined in a single Swift constant so it stays in sync with ADR-0005 and the proxy default:

```swift
// AirlockApp/Sources/AirlockApp/Models/PassthroughPolicy.swift
enum PassthroughPolicy {
    static let protectedHosts: Set<String> = [
        "api.anthropic.com",
        "auth.anthropic.com",
    ]

    /// Returns protected hosts missing from the given list. Pure; unit-testable.
    static func missingProtectedHosts(from list: [String]) -> [String] {
        let present = Set(list.map { $0.trimmingCharacters(in: .whitespaces).lowercased() })
        return protectedHosts
            .filter { !present.contains($0) }
            .sorted()
    }
}
```

### Three touch-points

1. **Inline warning in the passthrough editor** (Settings tab). While the user is editing the list, if `missingProtectedHosts(from: currentEdit)` is non-empty, show an inline warning row below the editor:
   > ⚠ Removing `api.anthropic.com` from passthrough means Airlock will decrypt secrets in requests to Anthropic. Your plaintext credentials will be sent to Anthropic's servers. This defeats the purpose of Airlock — only remove for testing.

2. **Save-time confirmation modal**. Only fires on Save when the final list is missing a protected host. Two buttons: `Cancel` (default, `Esc`) and `Remove anyway` (destructive-styled). Body restates which host(s) are being dropped.

3. **Persistent banner on Secrets tab**. When the resolved passthrough list (global settings + workspace override) is missing a protected host, show a yellow banner at the top of `SecretsView`:
   > ⚠ Anthropic passthrough disabled — secrets will be sent as plaintext to api.anthropic.com.
   Click opens Settings → Passthrough.

### Workspace override parity

The same `missingProtectedHosts` check runs against `workspace.passthroughHostsOverride` edits. Same component, same constant, same modal.

### Not changed

- CLI (`airlock run --passthrough-hosts`) — power-user path.
- Default passthrough list — already includes both protected hosts after commit `7390166`.
- The threat model — users retain the ability to remove Anthropic from passthrough after confirming.

### Files touched

- `AirlockApp/Sources/AirlockApp/Models/PassthroughPolicy.swift` — NEW, ~25 lines.
- `AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift` — global passthrough editor (lines ~143/147 handle `settings.passthroughHosts`). Add inline warning + save-time confirm modal.
- `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift` — workspace override editor (lines ~78/87 handle `passthroughHostsOverride`). Same inline warning + confirm (nil override inherits global; only flag when override is non-nil AND missing a protected host).
- `AirlockApp/Sources/AirlockApp/Views/Secrets/SecretsView.swift` — add persistent banner keyed off the resolved list (global + workspace override).
- `AirlockApp/Tests/AirlockAppTests/PassthroughPolicyTests.swift` — NEW. Cases: empty list, both present, one missing, both missing, case-insensitive, whitespace-tolerant.

**Size budget:** ~30 LOC production + ~60 LOC tests + three view edits.

## Section 2 — Env-secrets data model

### Config schema

New section in `.airlock/config.yaml`, parallel to `secret_files:`:

```yaml
container_image: airlock-claude:latest
proxy_image: airlock-proxy:latest
passthrough_hosts:
  - api.anthropic.com
  - auth.anthropic.com
secret_files:
  - path: /Users/berry/proj/.env
    format: dotenv
env_secrets:
  - name: GITHUB_TOKEN
    value: ENC[age:AQIBAAAB...base64...]
  - name: SLACK_BOT_TOKEN
    value: ENC[age:AQIBAAAC...base64...]
```

### Go types

```go
// internal/config/config.go

// EnvSecretConfig is a single encrypted environment variable injected
// into the agent container as NAME=ENC[age:...].
type EnvSecretConfig struct {
    Name  string `yaml:"name"`
    Value string `yaml:"value"` // always ENC[age:...] ciphertext
}

type Config struct {
    // ... existing fields ...
    SecretFiles []SecretFileConfig `yaml:"secret_files,omitempty"`
    EnvSecrets  []EnvSecretConfig  `yaml:"env_secrets,omitempty"` // NEW
}
```

### Invariants (enforced in `config.Load`)

1. `Name` matches `^[A-Za-z_][A-Za-z0-9_]*$` (POSIX env var name).
2. `Name` is unique within `env_secrets[]`.
3. `Name` is not in the reserved-name list (see Section 4).
4. `Value` passes `crypto.IsEncrypted(...)`.

Any violation is a hard error at load time. Every airlock command that loads config — including `airlock secret env list` and `airlock run` — fails with a clear message pointing at the offending entry. We do not silently drop invalid entries.

Save path uses the existing atomic `config.Save` (temp file + rename, `0o600`).

### Files touched

- `internal/config/config.go` — new struct, new field, new validator function `validateEnvSecrets(cfg *Config) error` called from `Load`.
- `internal/config/config_test.go` — round-trip (load → save → load equals identity), plus one failing test per invariant.

## Section 3 — CLI commands

Four new subcommands under `airlock secret env`.

```
airlock secret env add <NAME>   [--value <v>] [--stdin] [--force]
airlock secret env list         [--json]
airlock secret env remove <NAME>
airlock secret env show <NAME>  [--json]
```

### `add` — value sourcing precedence

1. `--value <v>` — literal. GUI and scripting path. Known `ps`-leakage trade-off.
2. `--stdin` — read until EOF, strip one trailing newline. Pipe-friendly.
3. TTY fallback — prompt `Value for <NAME>: ` via `golang.org/x/term.ReadPassword`, no echo. (`golang.org/x/term` is already a transitive dep via age.)
4. Non-TTY with no flag — error: `refusing to read plaintext from non-terminal without --value or --stdin`.

`--value` and `--stdin` are mutually exclusive.

After reading the value:

- If `crypto.IsEncrypted(value)` → accept as-is (idempotent round-tripping for scripts).
- Else → `ciphertext, _ := crypto.Encrypt(value, publicKey)`; `wrapped := crypto.WrapENC(ciphertext)`; store.
- Zero the plaintext byte slice (best-effort).
- If the name already exists without `--force` → error `env secret "<NAME>" already exists; use --force to overwrite`.
- Requires `.airlock/keys/age.pub` to exist (otherwise `run 'airlock init' first`).

### `list`

- Human default: table `NAME | SIZE | ADDED`, where SIZE is the ciphertext length (fingerprint-ish), and ADDED is omitted for v1 (no timestamp stored — adding one would require schema change, not justified).
- `--json`: `[{"name":"GITHUB_TOKEN"},{"name":"SLACK_BOT_TOKEN"}]`. Sorted by name. Names only. This is the GUI contract.

### `show`

- Human: `name: GITHUB_TOKEN\nencrypted: true\nvalue: ENC[age:AQIB…` (truncated at 16 ciphertext chars).
- JSON: `{"name":"GITHUB_TOKEN","encrypted":true,"value_prefix":"ENC[age:AQIB"}`.
- **Never** decrypts. There is no code path in `show` that calls `crypto.Decrypt`.

### `remove`

- Filter out, save, exit 0. If name not found, exit non-zero with `no such env secret: <NAME>`.
- No confirmation prompt (symmetric with `secret remove <file>`).

### Files touched

Mirrors existing `internal/cli/secret_*.go` layout:

```
internal/cli/secret_env.go          // parent subcommand group
internal/cli/secret_env_add.go      // RunSecretEnvAdd(name, value, stdin, force, airlockDir)
internal/cli/secret_env_list.go     // RunSecretEnvList(airlockDir, asJSON) -> []byte
internal/cli/secret_env_remove.go   // RunSecretEnvRemove(name, airlockDir)
internal/cli/secret_env_show.go     // RunSecretEnvShow(name, airlockDir, asJSON) -> []byte
```

Each `Run*` function is cobra-free; cobra wrappers do flag parsing only. Tests hit `Run*` directly with `t.TempDir()` as the fake `.airlock/`.

### Test matrix for Section 3

- `add` without keys dir → error.
- `add --value xxx` → config has 1 entry, `crypto.IsEncrypted` true, round-trip plaintext matches.
- `add --value xxx` twice → second errors; with `--force` → updates in place.
- `add` with invalid name (`1FOO`, `foo-bar`, `PATH=x`) → validation error before any encryption.
- `add` with pre-wrapped `ENC[age:...]` → stored as-is, no double-wrap.
- `add --value x --stdin` → mutual-exclusion error.
- `list --json` → sorted by name, only names present.
- `show --json` → does not contain the plaintext value.
- `remove` unknown → error, config unchanged.
- `remove` known → entry filtered, siblings preserved.

## Section 4 — Scanner and container injection

### New scanner

`internal/secrets/scanner_env_secret.go` — intentionally named `EnvSecretScanner` to avoid conflation with the existing `EnvScanner` (which handles `.env` files).

```go
type EnvSecretScanner struct {
    entries []config.EnvSecretConfig
}

func NewEnvSecretScanner(entries []config.EnvSecretConfig) *EnvSecretScanner

func (s *EnvSecretScanner) Name() string { return "env-secret" }

func (s *EnvSecretScanner) Scan(opts ScanOpts) (*ScanResult, error)
```

`Scan` does:

1. For each entry: defense-in-depth `crypto.IsEncrypted(entry.Value)` check; `inner := crypto.UnwrapENC(entry.Value)`; `plain := crypto.Decrypt(inner, opts.PrivateKey)`.
2. `result.Mapping[entry.Value] = plain`.
3. `result.Env = append(result.Env, EnvVar{Name: entry.Name, Value: entry.Value})`.

The mapping entry is what lets the proxy substitute the ciphertext back to plaintext on the wire. The `Env` entry is what causes the ciphertext to appear in the agent container's environment.

### `ScanResult` extension

```go
// internal/secrets/scanner.go

type ScanResult struct {
    Mounts  []ShadowMount
    Mapping map[string]string
    Env     []EnvVar           // NEW
}

type EnvVar struct {
    Name  string
    Value string   // always ENC[age:...] ciphertext
}
```

`ScanAll` merges `Env` across scanners like it already merges `Mounts` and `Mapping`. Merge policy for duplicate names across scanners: last-write-wins with a debug log. Practically irrelevant today (only `EnvSecretScanner` produces `Env` entries) but the merge rule must exist.

### `SessionParams` and `RunOpts` extension

```go
// internal/orchestrator/session.go
type SessionParams struct {
    // ... existing ...
    EnvSecrets []secrets.EnvVar   // NEW
}

// internal/container/manager.go
type RunOpts struct {
    // ... existing ...
    EnvSecrets []secrets.EnvVar   // NEW
}
```

### Container injection

In both `BuildClaudeConfig` and `BuildClaudeDetachedConfig`, after the existing `HTTP_PROXY` / `LANG` env block:

```go
for _, e := range opts.EnvSecrets {
    env = append(env, fmt.Sprintf("%s=%s", e.Name, e.Value))
}
```

Result inside the container: `echo $GITHUB_TOKEN` prints `ENC[age:AQIB...]`. The agent shell processes (e.g., `gh`, `curl`, `aws`) that read the env and put it into an outbound HTTP header get their bytes substituted by the proxy on the wire. Identical contract to file-based secrets.

### Reserved env var names

Names hard-coded in `internal/container/manager.go` (single source of truth) and re-exported for `config.validateEnvSecrets` to import:

```
HTTP_PROXY, HTTPS_PROXY, http_proxy, https_proxy, NO_PROXY, LANG
```

`config.Load` rejects any `env_secret` whose `Name` is in this set.

### Wiring into `run.go` and `start.go`

Both files already build a `[]secrets.Scanner` pipeline. In both places, after the `FileScanner` append:

```go
if len(cfg.EnvSecrets) > 0 {
    scanners = append(scanners, secrets.NewEnvSecretScanner(cfg.EnvSecrets))
}
```

And after `ScanAll`:

```go
params.EnvSecrets = scanResult.Env
```

Two mechanical insertions per file. If the diff gets ugly, extract a `buildScannerPipeline(cfg, workspace, envFile)` helper — but not as part of this branch unless needed.

### Test matrix for Section 4

**Unit (`internal/secrets/scanner_env_secret_test.go`):**

- Empty entries → empty `Env`, empty `Mapping`.
- One entry with a real age keypair generated in `t.TempDir()` → `Mapping[ciphertext] == plaintext`, `Env[0].Value == ciphertext`.
- Entry with ciphertext that can't be decrypted (wrong key) → error, message includes `entry.Name`.
- Entry with non-encrypted value → error (defense-in-depth; config validation should catch this but the scanner re-checks).

**Integration (`internal/container/manager_test.go`):**

- `BuildClaudeConfig` with two `EnvSecrets` → `Env` slice contains both `NAME=ENC[age:...]` lines in addition to the existing HTTP_PROXY block.
- `BuildClaudeConfig` with zero `EnvSecrets` → `Env` byte-identical to current behavior (regression guard).
- `BuildClaudeDetachedConfig` parity check.

**E2E (`internal/cli/run_env_secret_test.go`, new file):**

- `airlock run` with one env secret in config → fake runtime captures `ContainerConfig` passed to `RunAttached`; assert `Env` contains `NAME=ENC[age:...]` and the written `mapping.json` contains the ciphertext → plaintext entry.

## Section 5 — GUI integration

### Sidebar restructure

`SecretsView.swift`'s left panel today has two sections: `Files` and `Claude Settings`. Add a third, above `Files`:

```
┌ Secrets sidebar ────────────┐
│ Env Variables   ← NEW       │
│   GITHUB_TOKEN              │
│   SLACK_BOT_TOKEN           │
│ Files                       │
│   .env                      │
│   secrets.json              │
│ Claude Settings             │
│   settings.json             │
│                             │
│ [+ Add File]  [+ Add Env]   │
└─────────────────────────────┘
```

### Right panel for env-var selection

When an env var is selected, the right panel shows a single-row view (not the file `Table`):

- `Name:` GITHUB_TOKEN
- `Status:` encrypted (always)
- `Value:` `ENC[age:AQIB…` (truncated identically to CLI `show`)

Actions: `Remove` (context menu, with confirm modal) and `Copy name` (copies the name string). **No** Copy-value. **No** Decrypt. **No** Edit (remove + re-add is the edit path).

### Add flow

1. `+ Add Env` button → `AddEnvSecretSheet` modal.
2. Two fields:
   - `Name` — `TextField`, live-validated against `^[A-Za-z_][A-Za-z0-9_]*$`. Add button disabled until valid.
   - `Value` — `SecureField` (no keystroke visibility).
3. On Add: `cli.run(args: ["secret", "env", "add", name, "--value", value], workingDirectory: workspace.path)`.
4. On exit 0: dismiss, re-run `loadEnvSecrets()`.
5. On failure: surface `result.stderr` inline in the sheet (same pattern as `AddSecretFileSheet.swift`).
6. Drop the Swift `String` reference for `value` after the call returns (best-effort zeroization; consistent with the `--value` argv threat model).

### Remove flow

- Context menu → `Remove` → modal: "Remove GITHUB_TOKEN? This will delete the encrypted value from `.airlock/config.yaml`. Unrecoverable." → `Cancel` / `Remove` (destructive).
- `cli.run(["secret", "env", "remove", name])` → reload.

### Restart banner

Existing restart banner (`appState.isActive(workspace)` check at top of `SecretsView`) is unchanged — it already covers the "edited secrets while workspace is active" case semantically for env vars too.

### Passthrough guardrail banner

Lives at the top of `SecretsView`, above the sidebar. Already enumerated in Section 1. Not re-described here.

### Files touched

- `AirlockApp/Sources/AirlockApp/Models/EnvSecret.swift` — NEW. `struct EnvSecret: Identifiable { let id: UUID; let name: String }`. Never holds the value.
- `AirlockApp/Sources/AirlockApp/Views/Secrets/AddEnvSecretSheet.swift` — NEW. Modal sheet, SecureField, client-side name validation.
- `AirlockApp/Sources/AirlockApp/Views/Secrets/SecretsView.swift` — MODIFIED. New section, new right-panel routing, new load function, new stable pseudo-UUID for the Env section header.
- `AirlockApp/Tests/AirlockAppTests/EnvSecretTests.swift` — NEW. Parses `secret env list --json` fixture output, client-side name validation regex.

**Size budget:** ~120 LOC Swift production + ~40 LOC tests. Three new files, two modified.

### Known limitation: `--value` argv

Passing the plaintext through `argv` briefly exposes it in `ps` output. Same limitation `AddSecretFileSheet` would have if it accepted inline values. Accepted trade-off for v1 because Swift-Process stdin-piping is clunky in the async wrapper. Documented in the ADR. If a real concern surfaces, the CLI already has a `--stdin` path we can wire the GUI into later without changing the GUI surface.

## Section 6 — Error handling catalog

| Layer | Condition | Behavior |
|---|---|---|
| `config.Load` | `env_secrets[i].Name` fails regex | Hard error: `config load: env secret at index N: invalid name "1FOO": must match ^[A-Za-z_][A-Za-z0-9_]*$` |
| `config.Load` | `Name` is reserved | Hard error: `config load: env secret name "HTTP_PROXY" is reserved by airlock` |
| `config.Load` | duplicate `Name` | Hard error: `config load: duplicate env secret name "GITHUB_TOKEN"` |
| `config.Load` | `Value` fails `IsEncrypted` | Hard error: `config load: env secret "GITHUB_TOKEN": value is not an ENC[age:...] ciphertext` |
| `RunSecretEnvAdd` | no `.airlock/keys/age.pub` | `no encryption keys found; run 'airlock init' first` |
| `RunSecretEnvAdd` | `--value` and `--stdin` both set | `--value and --stdin are mutually exclusive` |
| `RunSecretEnvAdd` | non-TTY with no `--value`/`--stdin` | `refusing to read plaintext from non-terminal without --value or --stdin` |
| `RunSecretEnvAdd` | name exists, no `--force` | `env secret "GITHUB_TOKEN" already exists; use --force to overwrite` |
| `EnvSecretScanner.Scan` | decrypt fails (wrong key, corrupt) | `env secret "GITHUB_TOKEN": decrypt failed: <inner>` — aborts the whole session start |
| `GUI AddEnvSecretSheet` | CLI non-zero exit | Show `stderr` verbatim inline in the sheet |
| `GUI SecretsView` | `secret env list --json` non-zero | Set `errorMessage`, show `ContentUnavailableView` (mirrors file-list error handling) |

Guiding principle: on the session-start path, any failure for any configured secret aborts activation. We never silently drop a secret — a partial success that omits one is worse than a loud failure because the agent would then run without the credential and the user might not notice until a tool call fails obscurely.

## Section 7 — Security regression tests

These four assertions are the privacy safety net. If any fail, the whole architecture is broken.

1. **`config_test.go`**: round-trip of `EnvSecretConfig` through YAML never includes a `plaintext` field. Regression guard against someone adding a debug field.
2. **`scanner_env_secret_test.go`**: after `Scan`, `result.Env[i].Value` matches `crypto.IsEncrypted`. The ciphertext, not the plaintext, is what flows into the container.
3. **`manager_test.go`**: for every `EnvSecret` in `RunOpts`, the resulting `ContainerConfig.Env` contains `NAME=ENC[age:...]` and does **not** contain `NAME=<plaintext>`. Concretely: walk `cfg.Env`, match the `NAME=` prefix, assert `strings.HasPrefix(val[len("NAME="):], "ENC[age:")`.
4. **`cli/secret_env_add_test.go`**: after `RunSecretEnvAdd(name, plaintext, ...)`, the bytes of the written `config.yaml` contain `ENC[age:` and do **not** contain the plaintext value.

Each is one assertion, each is specific, each maps to a property in the threat model.

## Section 8 — Test matrix summary

### Go

| Package | New/changed files | Type |
|---|---|---|
| `internal/config` | `config_test.go` additions | Round-trip + invariant enforcement |
| `internal/secrets` | `scanner_env_secret_test.go` NEW | Unit with real age keypair from `t.TempDir()` |
| `internal/secrets` | `scanner_test.go` additions | `ScanAll` merge of `Env` across scanners |
| `internal/container` | `manager_test.go` additions | `BuildClaudeConfig` / `BuildClaudeDetachedConfig` env injection |
| `internal/cli` | `secret_env_add_test.go`, `secret_env_list_test.go`, `secret_env_remove_test.go`, `secret_env_show_test.go` NEW | `Run*`-level with `t.TempDir()` |
| `internal/cli` | `run_env_secret_test.go` NEW | E2E with fake runtime |

### Swift

| Target | New file |
|---|---|
| `AirlockAppTests` | `PassthroughPolicyTests.swift` |
| `AirlockAppTests` | `EnvSecretTests.swift` |

### Python

None. `proxy/addon/decrypt_proxy.py` is not touched — `EnvSecretScanner`'s ciphertexts flow through the same `mapping.json` substitution code path that already handles file-secret ciphertexts.

## Section 9 — Documentation

### New ADR: `docs/decisions/ADR-0010-environment-variable-secrets.md`

Sections (mirrors ADR-0005 / ADR-0008 style already in the repo):

- **Context**: PR #19 added file-based registration; some secrets have no natural file home. Passthrough guardrail is a related concern because removing Anthropic from passthrough subverts the "model never sees plaintext" property.
- **Decision (env secrets)**: store encrypted env vars inline in `config.yaml:env_secrets[]`, inject into the agent container as `NAME=ENC[age:...]`, rely on the existing proxy substitution boundary. Single source of truth for reserved names in `container/manager.go`.
- **Decision (guardrail)**: Swift-only, non-blocking, user can override with explicit confirmation.
- **Consequences**:
    - Ciphertext appears in process environment, visible to any process in the container — same as any env var, mitigated by the container isolation assumption already in the threat model.
    - No re-encryption on key rotation (inherited repo-wide limitation).
    - `--value` argv leakage briefly exposes plaintext in `ps` output; known and documented.
    - Guardrail does not prevent a determined user from removing Anthropic from passthrough.
- **Alternatives considered**:
    - Separate `env.secrets.yaml` file — rejected (YAGNI; `.airlock/` is already gitignored).
    - Plaintext-at-rest with session-start encryption — rejected (violates the existing "encrypted at rest" invariant).
    - Stdin-only injection with no `--value` flag — rejected (GUI needs synchronous value passing; `--value` is the practical bridge).
    - CLI guardrail — rejected (power-user path; existing `--passthrough-hosts` already prints the resolved list at session start).
- **Related**: ADR-0005 (settings secret protection), ADR-0008 (multi-format secrets).

### Other docs

- `docs/guides/security-model.md` — add env-secret as the third source (alongside file-secrets and claude-settings heuristic scanner) in the three-layer defense diagram.
- `CLAUDE.md` — add four rows to the Commands table:

  ```
  | `airlock secret env add <name>`  | Register an environment variable secret (prompts for value) |
  | `airlock secret env list [--json]` | List registered env secrets (names only) |
  | `airlock secret env show <name> [--json]` | Show env secret metadata (never decrypts) |
  | `airlock secret env remove <name>` | Unregister an env secret |
  ```

- No `docs/glossary/` additions. Terms are self-explanatory.

## Files touched (consolidated)

### New

- `AirlockApp/Sources/AirlockApp/Models/PassthroughPolicy.swift`
- `AirlockApp/Sources/AirlockApp/Models/EnvSecret.swift`
- `AirlockApp/Sources/AirlockApp/Views/Secrets/AddEnvSecretSheet.swift`
- `AirlockApp/Tests/AirlockAppTests/PassthroughPolicyTests.swift`
- `AirlockApp/Tests/AirlockAppTests/EnvSecretTests.swift`
- `internal/cli/secret_env.go`
- `internal/cli/secret_env_add.go`
- `internal/cli/secret_env_list.go`
- `internal/cli/secret_env_remove.go`
- `internal/cli/secret_env_show.go`
- `internal/cli/secret_env_add_test.go`
- `internal/cli/secret_env_list_test.go`
- `internal/cli/secret_env_remove_test.go`
- `internal/cli/secret_env_show_test.go`
- `internal/cli/run_env_secret_test.go`
- `internal/secrets/scanner_env_secret.go`
- `internal/secrets/scanner_env_secret_test.go`
- `docs/decisions/ADR-0010-environment-variable-secrets.md`

### Modified

- `AirlockApp/Sources/AirlockApp/Views/Secrets/SecretsView.swift`
- `AirlockApp/Sources/AirlockApp/Views/Settings/SettingsView.swift`
- `AirlockApp/Sources/AirlockApp/Views/Settings/WorkspaceSettingsView.swift`
- `internal/config/config.go`
- `internal/config/config_test.go`
- `internal/secrets/scanner.go`
- `internal/secrets/scanner_test.go`
- `internal/container/manager.go`
- `internal/container/manager_test.go`
- `internal/orchestrator/session.go`
- `internal/cli/run.go`
- `internal/cli/start.go`
- `docs/guides/security-model.md`
- `CLAUDE.md`

## Delivery

- Branch: `feat/env-secrets-and-passthrough-guardrail`.
- Single PR bundling both deliverables. The ADR ties them together under the shared security-model narrative.
- Implementation plan to be written next (separate document).
