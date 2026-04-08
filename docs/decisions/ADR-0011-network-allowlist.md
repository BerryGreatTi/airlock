# ADR-0011: Per-Workspace Network Allow-list

## Status
Accepted

## Context

Airlock's decryption proxy (`proxy/addon/decrypt_proxy.py`) already sees every outbound HTTP request from the agent container: it decides whether to pass through, decrypt `ENC[age:...]` tokens, or merely log. It classifies but never blocks. The agent container has no direct egress — the Docker network is `Internal: true` — so everything non-HTTP is already dropped at the network layer, but HTTP/HTTPS freely reaches any host the agent asks for.

Pilot users asked for a way to restrict which hosts the agent can reach per workspace ("this codebase only talks to GitHub and Anthropic"). The goal is both defense in depth against supply-chain / exfiltration risk AND a way to catch surprising agent behavior during development — if Claude tries to POST to an unexpected host, the developer wants to see a 403 instead of the request landing.

`docs/guides/security-model.md` already flagged this as a future enhancement: *"Consider adding a host allowlist to the proxy configuration."*

## Decision

Implement a per-workspace network allow-list enforced in the mitmproxy addon, following the same "global default + per-workspace override" pattern as passthrough hosts (ADR-0010) and the MCP allow-list (PR1).

### Enforcement point

Enforce in `proxy/addon/decrypt_proxy.py` inside the `request()` hook, BEFORE the existing passthrough classification. A host that is not on the allow-list gets a synthesized `403` response with a JSON error body (`{"error":"blocked_by_airlock","detail":"host is not in the workspace network allow-list"}`) — debuggable by the agent and by humans reading proxy logs, unlike a silent `flow.kill()`. The request never reaches the upstream server.

Ordering invariant: allow-list check runs BEFORE passthrough classification. Otherwise users could accidentally exempt blocked hosts by adding them to passthrough — the opposite of the intended UX. The ordering is covered by `test_allowlist_runs_before_passthrough`.

### Host pattern support

Two rules:

- **Exact match**: `api.stripe.com` matches only that host.
- **Suffix wildcard**: `*.stripe.com` matches any host ending with `.stripe.com` — including `api.stripe.com`, `checkout.stripe.com`, `deeply.nested.stripe.com` — but NOT the bare `stripe.com`. This matches cookie-scope semantics and is the same anchoring rule the addon already uses internally. The wildcard implementation stores the stripped form `.stripe.com` (with leading dot) and uses `host.endswith(suffix)`, which naturally rejects bare domains because the host is one character shorter than the suffix.

No regex. No CIDR. No port filtering. The goal is a host allow-list, not a network firewall; users who need more power can set `NetworkAllowlist` to `nil` and lean on the upstream network layer.

### Case sensitivity

Both the Python addon and the Swift guardrail lowercase hosts and allow-list entries before comparison. RFC 1035 §2.3.3 says hostnames are case-insensitive; the GUI preview (Swift `NetworkAllowlistPolicy`) and runtime enforcement (`decrypt_proxy.is_allowed`) must agree on case semantics or the guardrail's "allow-list covers Anthropic" preview would lie.

### Two-state semantic (not tri-state)

Unlike `EnabledMCPServers`, which has a three-state `nil / [] / [..]` semantic (no filter / filter all / allow only these), the network allow-list has two states: **empty/nil = allow all HTTP** (back-compat default) and **populated = restrict to the listed hosts**. There is no "block all HTTP" state. Users who want to block all HTTP traffic can set an allow-list containing a single unused entry like `localhost.invalid`, but that's an anti-pattern — the agent will not function.

Because the allow-list is two-state, `Config.NetworkAllowlist` uses `yaml:"network_allowlist,omitempty"` — the `omitempty` round-trip collapse (empty slice → nil on Load) is semantically safe and documented in a regression test (`TestNetworkAllowlistEmptyCollapsesToNil`).

### Anthropic guardrail

If the resolved allow-list is non-empty AND does not cover `api.anthropic.com` and `auth.anthropic.com`, the GUI shows an inline yellow warning and a destructive-styled confirmation alert on Save (same UX as the passthrough guardrail). The check uses the same `is_allowed` logic as the addon, so a user typing `*.anthropic.com` is correctly recognized as covering both protected hosts and does NOT trigger the warning.

The warning is a usability guardrail, not a hard block: `airlock run --network-allowlist localhost` from the CLI is allowed and the session simply won't be able to reach Anthropic (Claude Code will stop responding). The CLI already echoes the resolved allow-list on session start.

### CLI interface

- `airlock run --network-allowlist api.github.com,*.stripe.com` — overrides config.yaml for this session.
- `airlock start --network-allowlist ...` — same, for the GUI-driven detached session.
- Omitting the flag preserves the config.yaml value (which may be nil/empty = allow all).
- An empty value with the flag (`--network-allowlist ""`) is semantically "allow all" (back-compat); the user probably meant to clear an override.

### Data model wiring

- Go `Config.NetworkAllowlist []string` → `RunOpts.NetworkAllowlist` → `AIRLOCK_ALLOWED_HOSTS` env var on the proxy container in `BuildProxyConfig`.
- Swift `AppSettings.networkAllowlist: [String]?` (global), `Workspace.networkAllowlistOverride: [String]?` (per-workspace), resolved via `ResolvedSettings.networkAllowlist = workspace.networkAllowlistOverride ?? global.networkAllowlist`.
- Swift `ContainerSessionService.activate` passes `--network-allowlist` only when the resolved value is non-nil.

### Shared helpers

- Python: `_split_csv_env` helper in `decrypt_proxy.py` dedupes the comma-split-trim-filter pattern between `PASSTHROUGH_HOSTS_RAW` and `ALLOWED_HOSTS_RAW`.
- Swift: `NetworkAllowlistPolicy.splitHostLines` delegates to `PassthroughPolicy.splitHostLines` because the two editors share the same parsing contract.
- Swift: `HostListEditor` reusable view consolidates the four near-identical "TextEditor + Anthropic warning" blocks across `SettingsView` and `WorkspaceSettingsView`, matching the `MCPAllowListPicker` precedent from PR1.

## Consequences

- The agent container can no longer exfiltrate secrets or call surprise APIs over HTTP(S) once a user sets an allow-list. Non-HTTP protocols are already blocked by the Docker internal network.
- A misconfigured allow-list (e.g., forgetting `*.anthropic.com`) stops Claude Code from responding. The GUI guardrail mitigates the footgun; the CLI echoes the resolved list so the user sees it.
- `AIRLOCK_ALLOWED_HOSTS` env var size grows linearly with the allow-list. Practical upper bound is thousands of hosts; `ARG_MAX` on Linux is 128KB–2MB.
- Case-insensitivity applies to `passthrough` too (PR 2 touched that normalization). `API.anthropic.com` is now a passthrough match in both the GUI preview and runtime; previously only the GUI normalized.
- DNS rebinding: mitmproxy terminates TLS and uses the client-supplied hostname (SNI / Host header), not the resolved IP. An attacker who poisons the agent's DNS resolver would still need the agent to *send* a request to an allow-listed hostname. The allow-list is a hostname filter, not an IP filter; this is acceptable for the documented threat model.
- The allow-list does not cover requests that bypass the proxy (e.g., `NO_PROXY=localhost,127.0.0.1` is unchanged — the agent can still reach localhost).
- Forward-compatible: absent `network_allowlist:` field in old `config.yaml` deserializes to nil = allow all. Old workspaces.json files without `networkAllowlistOverride` likewise decode with the field nil.

## Alternatives Considered

### iptables inside the container

Rejected. Would require `NET_ADMIN` capability or a privileged init sidecar, both of which conflict with the `cap-drop=ALL` posture from ADR-0002. The proxy already sees every request; adding a second enforcement point is work and risk.

### Docker network policies / `--network-alias`

Rejected. Docker native network policies don't filter egress by hostname. They operate at L3/L4 and would force us to maintain an IP allow-list that drifts from reality as DNS changes.

### `flow.kill()` instead of synthesized 403

Rejected. A `kill` produces an opaque connection reset on the agent side, which manifests as ambiguous network errors in Claude Code. A 403 with a clear JSON body is a debuggable signal.

### Per-URL-path filtering

Rejected as out of scope. If `api.github.com` is on the allow-list, all paths on it are allowed. Path-level enforcement (e.g., block `POST /gists`) would push the proxy into API-firewall territory and requires a different design.

### Three-state `nil / [] / [..]` like MCP allow-list

Rejected. The MCP allow-list distinguishes "don't filter" (nil) from "filter everything" (empty) because disabling all MCPs is a valid security posture — the agent can still do its job. Disabling all HTTP traffic is not a valid posture; the agent immediately stops working. Collapsing empty and nil simplifies the semantic and matches user intent.

### Regex / CIDR / port matching

Rejected for MVP. Exact + suffix wildcard covers the pilot user requests without the footgun surface of regex. Adding more patterns is additive if real demand appears.

## References

- `proxy/addon/decrypt_proxy.py` — addon enforcement (`is_allowed`, `request()` hook)
- `proxy/addon/test_decrypt_proxy.py` — behaviour tests (12 new cases)
- `internal/config/config.go` — `Config.NetworkAllowlist`
- `internal/container/manager.go` — `RunOpts.NetworkAllowlist`, `AIRLOCK_ALLOWED_HOSTS` env var
- `AirlockApp/Sources/AirlockApp/Models/NetworkAllowlistPolicy.swift` — Swift mirror for guardrail
- `AirlockApp/Sources/AirlockApp/Views/Settings/HostListEditor.swift` — reusable editor view
- ADR-0010 — passthrough guardrail precedent
- Security model guide — Layer 5 section added for allow-list enforcement
