# Settings Secret Protection

## Problem

Airlock encrypts `.env` file secrets so agents never see plaintext. But configuration files mounted into the container -- `~/.claude/settings.json`, `.claude/settings.local.json`, and equivalents for other tools -- can contain plaintext API tokens in `env` blocks. The agent can read these directly via `cat`.

Example of exposed secrets:

```json
{
  "env": {
    "ANTHROPIC_API_KEY": "sk-ant-api03-xxxxx"
  },
  "mcpServers": {
    "slack": {
      "command": "npx",
      "args": ["-y", "@anthropic/mcp-server-slack"],
      "env": {
        "SLACK_TOKEN": "xoxb-xxxxx"
      }
    }
  }
}
```

Both the top-level `env` and per-MCP-server `env` blocks are readable by the agent.

## Solution

Introduce a **Scanner interface** that discovers and encrypts secrets in well-known configuration formats. Each format (`.env`, `.claude/settings.json`, future OpenAI/Slack configs) is handled by a dedicated scanner. All scanners share a common heuristic engine for secret detection.

Encrypted values use the existing `ENC[age:...]` pattern. The proxy decrypts them at the network boundary, same as `.env` secrets.

### Passthrough removal

The default `passthrough_hosts` list changes from `[api.anthropic.com, auth.anthropic.com]` to `[]` (empty). This allows the proxy to decrypt `ENC[age:...]` patterns in ALL outbound traffic, including Anthropic API calls. Without this change, an encrypted `ANTHROPIC_API_KEY` would reach the Anthropic API as ciphertext and fail authentication.

Existing users with a `config.yaml` already on disk are not affected -- `airlock init` only writes defaults on first run.

## Architecture

### File structure

```
internal/secrets/
  envfile.go            existing: .env parse/write utilities
  mapping.go            existing: mapping.json save
  scanner.go            new: Scanner interface, ScanAll orchestrator
  scanner_env.go        new: .env file scanner (wraps existing logic)
  scanner_claude.go     new: .claude/settings.json scanner
  heuristic.go          new: shared secret detection heuristics
  heuristic_test.go     new: heuristic unit tests
  scanner_env_test.go   new: EnvScanner tests
  scanner_claude_test.go new: ClaudeScanner tests
  scanner_test.go       new: ScanAll integration tests
```

### Scanner interface

```go
// Scanner finds and encrypts secrets in a specific config format.
type Scanner interface {
    Name() string
    Scan(opts ScanOpts) (*ScanResult, error)
}

type ScanOpts struct {
    Workspace  string
    HomeDir    string
    PublicKey  string
    PrivateKey string
    TmpDir     string
}

type ShadowMount struct {
    HostPath      string // processed file in tmpDir
    ContainerPath string // container path to shadow
}

type ScanResult struct {
    Mounts  []ShadowMount
    Mapping map[string]string // ENC[age:...] -> plaintext
}
```

### ScanAll orchestrator

```go
func ScanAll(scanners []Scanner, opts ScanOpts) (*ScanResult, error) {
    merged := &ScanResult{Mapping: make(map[string]string)}
    for _, s := range scanners {
        result, err := s.Scan(opts)
        if err != nil {
            return nil, fmt.Errorf("scanner %s: %w", s.Name(), err)
        }
        merged.Mounts = append(merged.Mounts, result.Mounts...)
        for k, v := range result.Mapping {
            merged.Mapping[k] = v
        }
    }
    return merged, nil
}
```

### Heuristic secret detection

A value is classified as a secret when any positive signal matches AND no exclusion applies.

**Exclusion rules** (checked first -- if any match, the value is NOT a secret):

| Rule | Examples |
|------|----------|
| Length < 8 | `"1"`, `"true"`, `"debug"` |
| Boolean/number-like | `"true"`, `"false"`, `"0"`, `"1"` |
| Path (starts with `/`) | `"/usr/local/bin/node"` |
| URL (starts with `http://` or `https://`) | `"https://api.example.com"` |

**Positive signals** (if any match after exclusions, the value IS a secret):

| Signal | Type | Examples |
|--------|------|---------|
| Key name contains (case-insensitive): `token`, `key`, `secret`, `password`, `credential`, `auth` | Key-based | `SLACK_TOKEN`, `API_KEY`, `DB_PASSWORD` |
| Value starts with: `sk-`, `sk_live_`, `sk_test_`, `pk_live_`, `pk_test_` | Value prefix | Stripe, Anthropic keys |
| Value starts with: `xoxb-`, `xoxp-`, `xoxa-`, `xoxr-` | Value prefix | Slack tokens |
| Value starts with: `ghp_`, `gho_`, `ghs_`, `ghu_` | Value prefix | GitHub tokens |
| Value starts with: `glpat-` | Value prefix | GitLab tokens |
| Value starts with: `AKIA` | Value prefix | AWS access keys |
| Value starts with: `eyJ` | Value prefix | JWT tokens |
| Value starts with: `whsec_` | Value prefix | Webhook secrets |
| Value starts with: `rk_live_`, `rk_test_` | Value prefix | Stripe restricted keys |

**Decision flow:**

```
Is value excluded? (short, boolean, path, URL)
  YES -> not a secret
  NO  -> Does key name match? OR Does value prefix match?
           YES -> secret (encrypt)
           NO  -> not a secret (leave as-is)
```

The function signature:

```go
func IsSecret(key, value string) bool
```

### ClaudeScanner

Processes up to 4 files:

| Host path | Container path | Condition |
|-----------|---------------|-----------|
| `~/.claude/settings.json` | `/home/airlock/.claude/settings.json` | File exists |
| `~/.claude/settings.local.json` | `/home/airlock/.claude/settings.local.json` | File exists |
| `<workspace>/.claude/settings.json` | `/workspace/.claude/settings.json` | File exists |
| `<workspace>/.claude/settings.local.json` | `/workspace/.claude/settings.local.json` | File exists |

For each file:

1. Read and parse as JSON (preserve structure using `json.RawMessage` or `map[string]any`)
2. Walk the JSON tree looking for `env` blocks:
   - Top-level `"env": { ... }`
   - `"mcpServers": { "<name>": { "env": { ... } } }`
3. For each key-value pair in an env block, apply `IsSecret(key, value)`
4. If secret: encrypt value, replace in JSON, add to mapping
5. Write processed JSON to `tmpDir/<unique-name>.json`
6. Return `ShadowMount{HostPath: tmpPath, ContainerPath: containerPath}`

### EnvScanner

Wraps the existing `ParseEnvFile` + `EncryptEntries` logic into the Scanner interface. Also handles the shadow mount for workspace-internal env files (the logic from commit `ae01e7e`).

- If `envFilePath` is empty, returns an empty result (no-op scanner)
- If `envFilePath` is inside workspace, adds a shadow mount

### Integration with CLI

In `run.go` and `start.go`, the current env-file-specific code is replaced with the scanner pipeline:

```go
scanners := []secrets.Scanner{
    secrets.NewClaudeScanner(),
}
if envFile != "" {
    scanners = append(scanners, secrets.NewEnvScanner(envFile, workspace))
}

result, err := secrets.ScanAll(scanners, secrets.ScanOpts{
    Workspace:  workspace,
    HomeDir:    homeDir,
    PublicKey:  kp.PublicKey,
    PrivateKey: kp.PrivateKey,
    TmpDir:     tmpDir,
})

// Save merged mapping
if len(result.Mapping) > 0 {
    params.MappingPath, _ = secrets.SaveMapping(result.Mapping, tmpDir)
}

// Collect shadow mounts
params.ShadowMounts = result.Mounts
```

### Integration with container manager

`RunOpts` gains a `ShadowMounts` field:

```go
type RunOpts struct {
    // ... existing fields ...
    ShadowMounts []secrets.ShadowMount
}
```

`BuildClaudeConfig` appends all shadow mounts as read-only binds:

```go
for _, m := range opts.ShadowMounts {
    binds = append(binds, fmt.Sprintf("%s:%s:ro", m.HostPath, m.ContainerPath))
}
```

This replaces the single-purpose `EnvShadowPath` field from commit `ae01e7e`. The env shadow mount is now just one entry in the `ShadowMounts` slice, produced by `EnvScanner`.

### Integration with orchestrator

`SessionParams` gains `ShadowMounts` (replaces `EnvShadowPath`):

```go
type SessionParams struct {
    // ... existing fields ...
    ShadowMounts []secrets.ShadowMount
}
```

Propagated to `RunOpts.ShadowMounts` in both `StartSession` and `StartDetachedSession`.

### Keypair requirement

The scanner pipeline needs the age keypair. Currently, `run.go` and `start.go` only load the keypair when `--env` is provided. With settings scanning, the keypair is always needed (settings files may contain secrets even without `--env`).

Change: always load the keypair at the start of `run` and `start` commands. If `.airlock/keys/` does not exist, skip the scanner pipeline entirely (the user has not run `airlock init`).

### Default passthrough change

In `internal/config/config.go`, the `Default()` function changes:

```go
// Before
PassthroughHosts: []string{"api.anthropic.com", "auth.anthropic.com"},

// After
PassthroughHosts: []string{},
```

Existing `config.yaml` files on disk are not modified. Only new `airlock init` runs produce the empty default. Users can manually add passthrough hosts if desired.

## Data flow

```
Host                          Container

~/.claude/settings.json  -->  ClaudeScanner  -->  processed JSON (ENC values)
  "SLACK_TOKEN": "xoxb-xxx"                        "SLACK_TOKEN": "ENC[age:...]"
                                    |
.env                     -->  EnvScanner     -->  env.enc (ENC values)
  STRIPE_KEY=sk_live_xxx                           STRIPE_KEY='ENC[age:...]'
                                    |
                              ScanAll merges  -->  mapping.json
                                                    { "ENC[age:aaa]": "xoxb-xxx",
                                                      "ENC[age:bbb]": "sk_live_xxx" }
                                    |
                              Shadow mounts   -->  Docker bind mounts
                              + mapping mount      overlay originals with processed files

Agent reads /workspace/.env          --> sees ENC[age:bbb]
Agent reads ~/.claude/settings.json  --> sees ENC[age:aaa]
MCP server calls Slack API           --> Header: Bearer ENC[age:aaa]
                                          --> Proxy replaces --> Bearer xoxb-xxx
                                          --> Slack receives real token
```

## Testing

### Heuristic tests (`heuristic_test.go`)

Table-driven tests covering:
- Known secret prefixes (sk-, xoxb-, ghp_, etc.) -> true
- Key name matches (SLACK_TOKEN, API_KEY, etc.) -> true
- Exclusions (short values, booleans, paths, URLs) -> false
- Non-secret values ("production", "us-east-1") -> false
- Edge cases: empty string, exactly 8 chars, key match but excluded value

### ClaudeScanner tests (`scanner_claude_test.go`)

- Settings file with mcpServers env block -> secrets encrypted, non-secrets preserved
- Settings file with top-level env block -> secrets encrypted
- Settings file without env blocks -> no mounts, empty mapping
- Missing settings file -> skipped, no error
- Mixed: some values are secrets, some are not -> only secrets encrypted
- Both global and project-level files exist -> 4 shadow mounts
- Malformed JSON -> error returned

### EnvScanner tests (`scanner_env_test.go`)

- Wraps existing encrypt logic -> same behavior as current EncryptEntries
- Env file inside workspace -> shadow mount produced
- Env file outside workspace -> no shadow mount
- Empty env file path -> empty result

### ScanAll tests (`scanner_test.go`)

- Multiple scanners -> mappings merged, mounts concatenated
- One scanner fails -> error propagated with scanner name
- No secrets found by any scanner -> empty result

### Integration

- `go test -race ./internal/...` -- all packages pass
- `make test-python` -- proxy tests pass (no Python changes in this spec)

## Limitations

- **Non-HTTP MCP servers**: MCP servers that use secrets for non-HTTP operations (database connections, local file auth) will receive `ENC[age:...]` values and fail. This is a known limitation of the proxy-based architecture, documented in the security model.
- **Heuristic false negatives**: Secrets with unusual naming or format may not be detected. Users should put such secrets in `.env` files where all values are encrypted unconditionally.
- **Heuristic false positives**: A non-secret value matching a key pattern (e.g., `AUTH_MODE=oauth2`) would be encrypted unnecessarily. The proxy decrypts it transparently, so this causes no functional issue -- only a minor performance overhead.
- **JSON formatting**: The processed settings file may have different whitespace/formatting than the original. This is cosmetic and does not affect functionality since Claude Code parses JSON, not raw text.
