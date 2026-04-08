# Manual test runbook: per-workspace network allow-list

> Companion to [ADR-0011](../../decisions/ADR-0011-network-allowlist.md)
> and PR #24 (`feat/network-allowlist`). Executes seven end-to-end
> scenarios against a live Airlock session before merging. The first
> five are CLI-only and take ~10 minutes; the GUI scenarios (6-7)
> need a built `Airlock.app` and add ~10 minutes.

## Run log

| Date | Operator | Scenarios | Outcome |
|---|---|---|---|
| 2026-04-08 | Claude Code | 1, 2, 3, 4, 5 | All 5 CLI scenarios **PASS**. Scenarios 6, 7 pending (GUI, manual). |

Details of the 2026-04-08 run:

- **Scenario 1** (back-compat): `httpbin.org` and `api.github.com` both returned 200; proxy logged no `blocked` actions.
- **Scenario 2** (exact match): `api.github.com` → 200, `httpbin.org` → 403 with `blocked_by_airlock` body; `AIRLOCK_ALLOWED_HOSTS` env var correctly set on the proxy container.
- **Scenario 3** (cookie-scope wildcard): `api.stripe.com` → 401 (Stripe auth denied, upstream reached = allow-list passed), bare `stripe.com` → 403 `blocked_by_airlock`, `example.com` → 403.
- **Scenario 4** (case-insensitive): mixed-case `API.GitHub.COM` in the allow-list matched a lowercase `api.github.com` request; uppercase `HTTPBIN.ORG` request was still blocked with 403.
- **Scenario 5** (ordering invariant — CRITICAL): allow-list=`api.github.com`, passthrough=`api.anthropic.com,auth.anthropic.com`. Request to `api.anthropic.com` was **blocked** with 403 `blocked_by_airlock` (proving allow-list runs before passthrough); request to `api.github.com` → 200. Proxy log showed `"action": "blocked"` on `api.anthropic.com`, NO `"action": "passthrough"`.

Two runbook corrections from this run (already applied below):

1. `airlock` Go binary uses the raw Docker SDK and defaults to `/var/run/docker.sock`. On Rancher/Colima desktops the prerequisite section now documents the `DOCKER_HOST` export.
2. `curl` inside the agent container does not yet trust the mitmproxy-generated CA at the host level (pre-existing, unrelated to this PR). All test `curl`s now use `-k`. The allow-list check runs at the HTTP request layer AFTER the TLS handshake, so `-k` does not bypass it — a blocked host still gets the synthesized 403 regardless of trust chain.
3. The proxy addon emits JSON log lines via `json.dumps` default separators (`", "` and `": "` with spaces). Grep patterns updated to tolerate the spaces.

## Prerequisites

- Docker Desktop, Rancher Desktop, or Colima running
- `bin/airlock` built from the branch HEAD: `make build`
- `airlock-proxy:latest` image rebuilt on this branch (the Python addon
  changed): `docker build -t airlock-proxy:latest -f proxy/Dockerfile proxy/`
- Airlock GUI built (scenarios 6 and 7 only): `make gui-build` or `make gui-package`
- A separate scratch directory for each CLI scenario; do NOT reuse an existing project workspace
- A working Claude Code login inside the airlock volume (only scenario 7b — the full end-to-end test)
- **If using Rancher Desktop**, the airlock Go binary defaults to
  `/var/run/docker.sock` and will fail with "Cannot connect to the Docker
  daemon". Export the per-user socket before running any `airlock`
  command:
  ```bash
  export DOCKER_HOST=unix:///Users/$(whoami)/.rd/docker.sock
  ```
  Colima users: `export DOCKER_HOST=unix://$HOME/.colima/default/docker.sock`
- **Proxy CA trust**: `curl` inside the agent container does not yet
  trust the mitmproxy-generated CA by default, so raw TLS verification
  fails. Every test `curl` in this runbook uses `-k` (insecure) so the
  test exercises the allow-list enforcement independently of the CA
  trust chain — the allow-list check runs at the HTTP request layer
  AFTER the TLS handshake, so `-k` does not bypass it. If a blocked
  host returns the airlock-synthesized 403, the enforcement fired
  regardless of the trust chain.

## Verification philosophy

| Scenario | Property verified |
|---|---|
| 1 | No allow-list = allow all HTTP (back-compat default, no regression of existing workspaces) |
| 2 | Allow-list with exact match blocks non-listed hosts with a synthesized 403 |
| 3 | Suffix wildcard `*.stripe.com` matches subdomains but NOT the bare domain (cookie-scope rule) |
| 4 | Case-insensitive matching (RFC 1035) — enforcement agrees with the GUI guardrail preview |
| 5 | Ordering invariant — allow-list wins over passthrough; a passthrough host NOT on the allow-list is still blocked |
| 6 | GUI global Settings allow-list + Anthropic guardrail (inline warning + confirm alert) |
| 7 | GUI workspace override + guardrail chaining (passthrough + allow-list alerts fire in order) |

All scenarios are pass/fail on a single observable — a `curl` exit code / response body / proxy log line, or a visible UI state. If any step deviates from the "Expected" text, stop and investigate before proceeding.

---

## Scenario 1 — No allow-list = allow all HTTP (back-compat)

**Goal:** A workspace with no `network_allowlist` configured can reach arbitrary HTTP hosts. Verifies that existing workspaces continue to work unchanged after upgrading to this PR.

**Setup:**

```bash
mkdir -p /tmp/airlock-test-allowlist-1 && cd /tmp/airlock-test-allowlist-1
/Users/berry.kim/Projects/airlock/bin/airlock init
```

### Steps

- [ ] **1.1** Verify the freshly-written config does NOT contain an allow-list:
    ```bash
    grep network_allowlist .airlock/config.yaml && echo "UNEXPECTED: field present" || echo "OK: field absent"
    ```
    **Expected:** `OK: field absent` (nil / unset = allow all).
- [ ] **1.2** Start a detached session:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock start --id allowlist1 --workspace $(pwd)
    ```
    **Expected:** JSON output with `"status":"running"` and container names.
- [ ] **1.3** Exec into the container and reach two unrelated public HTTP hosts:
    ```bash
    docker exec airlock-claude-allowlist1 sh -c 'curl -skS -o /dev/null -w "%{http_code}\n" https://httpbin.org/get'
    docker exec airlock-claude-allowlist1 sh -c 'curl -skS -o /dev/null -w "%{http_code}\n" https://api.github.com'
    ```
    **Expected:** Both print `200`. Neither is blocked.
- [ ] **1.4** Inspect the proxy log — no `blocked` actions should appear:
    ```bash
    docker logs airlock-proxy-allowlist1 2>&1 | grep -E '"action":\s*"blocked"' && echo "UNEXPECTED: blocks present" || echo "OK: no blocks"
    ```
    **Expected:** `OK: no blocks`.
- [ ] **1.5** Stop the session:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock stop --id allowlist1
    ```

### Expected outcome

Existing workspaces continue to work. The upgrade to this PR is a no-op for users who do not set an allow-list.

---

## Scenario 2 — Exact-match allow-list blocks non-listed hosts

**Goal:** An allow-list containing `api.github.com` allows GitHub through and blocks everything else with a synthesized 403.

**Setup:**

```bash
mkdir -p /tmp/airlock-test-allowlist-2 && cd /tmp/airlock-test-allowlist-2
/Users/berry.kim/Projects/airlock/bin/airlock init
```

### Steps

- [ ] **2.1** Start a session with an allow-list that includes GitHub and Anthropic but NOT httpbin:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock start \
        --id allowlist2 \
        --workspace $(pwd) \
        --network-allowlist "api.github.com,api.anthropic.com,auth.anthropic.com"
    ```
- [ ] **2.2** Verify GitHub is reachable:
    ```bash
    docker exec airlock-claude-allowlist2 sh -c 'curl -skS -o /dev/null -w "%{http_code}\n" https://api.github.com'
    ```
    **Expected:** `200`.
- [ ] **2.3** Verify httpbin is BLOCKED with a 403 from the proxy:
    ```bash
    docker exec airlock-claude-allowlist2 sh -c 'curl -skS -w "\nHTTP=%{http_code}\n" https://httpbin.org/get'
    ```
    **Expected:** Output contains `HTTP=403` and the body contains
    `{"error":"blocked_by_airlock","detail":"host is not in the workspace network allow-list"}`.
    **CRITICAL:** If the HTTP code is 200, the allow-list is not being enforced — STOP and investigate.
- [ ] **2.4** Inspect the proxy log for the structured `blocked` action:
    ```bash
    docker logs airlock-proxy-allowlist2 2>&1 | grep httpbin.org | grep -E '"action":\s*"blocked"'
    ```
    **Expected:** At least one JSON line like
    `{"time": "HH:MM:SS", "host": "httpbin.org", "action": "blocked"}`
    (note the spaces after colons — Python `json.dumps` default).
- [ ] **2.5** Verify the `AIRLOCK_ALLOWED_HOSTS` env var is set on the proxy container:
    ```bash
    docker exec airlock-proxy-allowlist2 printenv AIRLOCK_ALLOWED_HOSTS
    ```
    **Expected:** `api.github.com,api.anthropic.com,auth.anthropic.com`.
- [ ] **2.6** Stop the session:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock stop --id allowlist2
    ```

### Expected outcome

Allow-list is enforced at the proxy layer. Unlisted hosts get a 403 with a clear debuggable body, and the proxy logs a `blocked` action.

---

## Scenario 3 — Suffix wildcard with cookie-scope rule

**Goal:** `*.stripe.com` matches subdomains but NOT the bare `stripe.com` or lookalike hosts. Same behaviour as browser cookie scoping; mirrors the Python `is_allowed` logic tested by `test_allowlist_suffix_wildcard_*`.

**Setup:**

```bash
mkdir -p /tmp/airlock-test-allowlist-3 && cd /tmp/airlock-test-allowlist-3
/Users/berry.kim/Projects/airlock/bin/airlock init
```

### Steps

- [ ] **3.1** Start with a suffix-wildcard allow-list + Anthropic so the proxy boots normally:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock start \
        --id allowlist3 \
        --workspace $(pwd) \
        --network-allowlist "*.stripe.com,api.anthropic.com,auth.anthropic.com"
    ```
- [ ] **3.2** Verify a Stripe subdomain is allowed (we use `api.stripe.com`; a 401 or 404 is fine — we only care that the proxy did NOT return 403):
    ```bash
    docker exec airlock-claude-allowlist3 sh -c 'curl -skS -o /dev/null -w "%{http_code}\n" https://api.stripe.com/v1/charges'
    ```
    **Expected:** A code in the 200-499 range that is NOT 403 (typically 401 without credentials). If you see 403, check whether the body is the airlock "blocked" JSON — if so the test has failed.
- [ ] **3.3** Verify the bare domain `stripe.com` is BLOCKED (cookie-scope rule: `*.stripe.com` does not match `stripe.com`):
    ```bash
    docker exec airlock-claude-allowlist3 sh -c 'curl -skS -w "\nHTTP=%{http_code}\n" https://stripe.com/'
    ```
    **Expected:** `HTTP=403` and body contains `blocked_by_airlock`.
- [ ] **3.4** Verify lookalike domains are BLOCKED:
    ```bash
    docker exec airlock-claude-allowlist3 sh -c 'curl -skS -o /dev/null -w "%{http_code}\n" https://example.com/'
    ```
    **Expected:** `403`.
- [ ] **3.5** Stop the session:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock stop --id allowlist3
    ```

### Expected outcome

The `*.stripe.com` pattern correctly matches subdomains while rejecting the bare domain. Enforcement matches the documented cookie-scope semantics and the Swift `NetworkAllowlistPolicy` guardrail preview.

---

## Scenario 4 — Case-insensitive matching

**Goal:** Allow-list entries and runtime hostnames are lowercased before compare (RFC 1035 §2.3.3). The GUI guardrail already normalizes; this scenario verifies the Python addon does too.

**Setup:**

```bash
mkdir -p /tmp/airlock-test-allowlist-4 && cd /tmp/airlock-test-allowlist-4
/Users/berry.kim/Projects/airlock/bin/airlock init
```

### Steps

- [ ] **4.1** Start with an allow-list using mixed case:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock start \
        --id allowlist4 \
        --workspace $(pwd) \
        --network-allowlist "API.GitHub.COM,API.Anthropic.com,Auth.Anthropic.com"
    ```
- [ ] **4.2** Verify a lowercase request matches the mixed-case allow-list entry:
    ```bash
    docker exec airlock-claude-allowlist4 sh -c 'curl -skS -o /dev/null -w "%{http_code}\n" https://api.github.com'
    ```
    **Expected:** `200` (curl also lowercases the Host header, so this exercises the allow-list-side normalization).
- [ ] **4.3** Verify a host not in the list is still blocked regardless of case:
    ```bash
    docker exec airlock-claude-allowlist4 sh -c 'curl -skS -o /dev/null -w "%{http_code}\n" https://HTTPBIN.ORG/get'
    ```
    **Expected:** `403`.
- [ ] **4.4** Stop the session:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock stop --id allowlist4
    ```

### Expected outcome

Mixed-case allow-list entries are matched case-insensitively, consistent with the Swift `NetworkAllowlistPolicy.testCaseInsensitive` test and RFC 1035.

---

## Scenario 5 — Ordering invariant: allow-list wins over passthrough

**Goal:** The allow-list check runs BEFORE the passthrough classification. A host that is on the passthrough list but NOT on the allow-list is still blocked — users cannot accidentally exempt blocked hosts by adding them to passthrough.

This is the security-critical ordering invariant documented in ADR-0011 and enforced by `test_allowlist_runs_before_passthrough`.

**Setup:**

```bash
mkdir -p /tmp/airlock-test-allowlist-5 && cd /tmp/airlock-test-allowlist-5
/Users/berry.kim/Projects/airlock/bin/airlock init
```

### Steps

- [ ] **5.1** Start with an allow-list that does NOT include Anthropic, and explicitly pass Anthropic in the passthrough flag:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock start \
        --id allowlist5 \
        --workspace $(pwd) \
        --network-allowlist "api.github.com" \
        --passthrough-hosts "api.anthropic.com,auth.anthropic.com"
    ```
- [ ] **5.2** Confirm both env vars are set on the proxy:
    ```bash
    docker exec airlock-proxy-allowlist5 sh -c 'printenv AIRLOCK_ALLOWED_HOSTS AIRLOCK_PASSTHROUGH_HOSTS'
    ```
    **Expected:**
    ```
    api.github.com
    api.anthropic.com,auth.anthropic.com
    ```
- [ ] **5.3** Verify the passthrough host is BLOCKED because it's not on the allow-list:
    ```bash
    docker exec airlock-claude-allowlist5 sh -c 'curl -skS -w "\nHTTP=%{http_code}\n" https://api.anthropic.com/'
    ```
    **Expected:** `HTTP=403` and body contains `blocked_by_airlock`.
    **CRITICAL:** If the response is anything other than a 403 with the airlock body, the ordering invariant is broken and this PR must not merge. A 403 from Anthropic's own servers (with a different body) would also indicate a wrong outcome.
- [ ] **5.4** Verify the allow-listed host is reachable:
    ```bash
    docker exec airlock-claude-allowlist5 sh -c 'curl -skS -o /dev/null -w "%{http_code}\n" https://api.github.com'
    ```
    **Expected:** `200`.
- [ ] **5.5** Inspect the proxy log — there should be a `blocked` action on `api.anthropic.com` but NO `passthrough` action on it:
    ```bash
    docker logs airlock-proxy-allowlist5 2>&1 | grep anthropic.com
    ```
    **Expected:** Lines with `"action": "blocked"`, NO lines with `"action": "passthrough"` for Anthropic (note the spaces).
- [ ] **5.6** Stop the session:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock stop --id allowlist5
    ```

### Expected outcome

Allow-list enforcement runs strictly before passthrough classification. The ordering invariant holds end-to-end.

---

## Scenario 6 — GUI global Settings + Anthropic guardrail

**Goal:** The GUI's global Network Allow-list editor shows the inline yellow warning when the allow-list does not cover `api.anthropic.com` / `auth.anthropic.com`, and the Save button triggers a destructive-styled confirmation alert. A wildcard `*.anthropic.com` clears the warning.

**Setup:** GUI only. Launch `make gui-run` or double-click `build/Airlock.app`.

### Steps

- [ ] **6.1** Open global Settings (gear icon in the sidebar or menu `Airlock → Settings...`).
- [ ] **6.2** Scroll to the "Network Allow-list" section. **Expected initial state:**
    - Toggle `Restrict outbound hosts` is OFF.
    - Caption reads: `Agent container can reach any HTTP/HTTPS host. Non-HTTP traffic is already blocked by the isolated Docker network.`
    - No TextEditor visible.
- [ ] **6.3** Flip the toggle ON. **Expected:**
    - A monospaced TextEditor appears (empty).
    - Caption reads: `Only the listed hosts can receive outbound HTTP/HTTPS traffic. Use *.example.com for subdomain wildcards. One entry per line.`
    - NO yellow warning row yet (empty list is not actionable).
- [ ] **6.4** Type `api.github.com` on the first line. **Expected:**
    - An inline yellow warning appears below the editor saying the allow-list is missing `api.anthropic.com` and `auth.anthropic.com`.
- [ ] **6.5** Click **Save**. **Expected:**
    - A red/destructive confirmation alert titled `Allow-list blocks Anthropic?` with a Cancel button and a `Save anyway` button.
    - Click **Cancel** — the sheet stays open, nothing is persisted.
- [ ] **6.6** Delete the line and replace with `*.anthropic.com` on line one and `api.github.com` on line two. **Expected:**
    - The yellow warning disappears (the wildcard covers both protected hosts).
- [ ] **6.7** Click **Save**. **Expected:**
    - No confirmation alert.
    - The sheet closes, "Saved" briefly flashes.
- [ ] **6.8** Reopen global Settings and confirm the editor shows `*.anthropic.com` and `api.github.com` persisted across reload.
- [ ] **6.9** Flip the toggle OFF again and click **Save**. **Expected:**
    - The sheet closes normally.
    - Reopen: the caption is back to the "allow any host" text and the toggle is OFF. `~/Library/Application Support/Airlock/settings.json` no longer contains a `networkAllowlist` key (or it's `null`).

### Expected outcome

The inline warning, Save-time alert, wildcard-coverage recognition, and toggle-off persistence all behave as designed.

---

## Scenario 7 — GUI workspace override + guardrail chaining

**Goal:** The workspace-level override works independently of global, and when the user removes Anthropic from BOTH the passthrough editor AND the allow-list editor, both confirmation alerts fire **in order** — confirming the first does not silently bypass the second (the H1 fix applied during the simplify pass).

This is the critical UX regression fix from the code review.

**Setup:** GUI only. You need at least one workspace in the sidebar. Use Scenario 1's workspace or create a fresh one at `/tmp/airlock-test-allowlist-7`.

### Subscenario 7a — Chained guardrails (no session activation needed)

- [ ] **7a.1** Select the workspace in the sidebar, switch to the **Settings** tab (Cmd+4).
- [ ] **7a.2** In "Network Overrides", type a passthrough override that does NOT include Anthropic:
    ```
    api.github.com
    ```
    **Expected:** An inline yellow warning fires because the override would drop both Anthropic hosts.
- [ ] **7a.3** In "Network Allow-list Override", flip the toggle ON and type:
    ```
    api.github.com
    ```
    **Expected:** A second inline yellow warning fires in the allow-list section (missing Anthropic).
- [ ] **7a.4** Click **Save**. **Expected flow:**
    - **First alert**: `Disable Anthropic passthrough for this workspace?` — title mentions passthrough. Click `Remove anyway`.
    - **Second alert** (immediately after dismissing the first): `Allow-list blocks Anthropic in this workspace?` — title mentions allow-list. This alert MUST appear. If it does not, the guardrail-chaining fix has regressed — STOP and investigate.
    - Click `Cancel` on the second alert. Neither override is saved.
- [ ] **7a.5** Verify persistence: reopen the tab. The two TextEditors should still show the in-progress drafts (not yet persisted). The workspace's `workspaces.json` entry should NOT contain `passthroughHostsOverride` or `networkAllowlistOverride`.
- [ ] **7a.6** Clear both editors (delete all text in both), click **Save**. **Expected:** No alerts, sheet closes. The workspace now falls back to global settings for both.

### Subscenario 7b — End-to-end: allow-list-restricted workspace runs Claude Code

This is the most expensive scenario. It verifies that Claude Code still works when the allow-list is correctly configured to cover Anthropic, and that an obviously-unneeded host is still blocked.

- [ ] **7b.1** In the same workspace's Settings tab, set the allow-list override to:
    ```
    *.anthropic.com
    api.github.com
    ```
    Clear the passthrough override. Save.
- [ ] **7b.2** Activate the workspace via the GUI.
- [ ] **7b.3** Wait for Claude Code to finish booting. Switch to the Terminal tab.
- [ ] **7b.4** Verify Claude Code is responsive: type a simple prompt like:
    > What is 2 plus 2?
    **Expected:** Claude responds normally. This confirms the proxy is allowing `api.anthropic.com` traffic through, and `*.anthropic.com` correctly covers the auth and API hosts.
    **CRITICAL:** If Claude Code hangs or reports network errors, the wildcard coverage is broken — STOP and investigate.
- [ ] **7b.5** Ask Claude to reach a host NOT on the allow-list:
    > Please run the bash command `curl -skS -w "\nHTTP=%{http_code}\n" https://httpbin.org/get` using the Bash tool and quote the output literally.
    **Expected:** Claude's response quotes the curl output, which shows `HTTP=403` and a body containing `blocked_by_airlock`.
- [ ] **7b.6** Deactivate the workspace.
- [ ] **7b.7** Reopen Settings, clear the allow-list override (toggle OFF), save.

### Expected outcome

**Subscenario 7a** confirms the guardrail-chaining fix: both alerts fire sequentially, neither is silently suppressed.

**Subscenario 7b** confirms the end-to-end loop: Claude Code operates normally under a restrictive allow-list that includes Anthropic, and the agent is correctly denied egress to other hosts.

---

## Global cleanup

After completing all scenarios:

```bash
cd /tmp
rm -rf airlock-test-allowlist-1 airlock-test-allowlist-2 airlock-test-allowlist-3 \
       airlock-test-allowlist-4 airlock-test-allowlist-5 airlock-test-allowlist-7
docker ps --format '{{.Names}}' | grep '^airlock-' | xargs -r docker stop
docker ps -a --format '{{.Names}}' | grep '^airlock-' | xargs -r docker rm
docker network ls --format '{{.Name}}' | grep '^airlock-net-' | xargs -r docker network rm
```

If you used the GUI in Scenario 7, remove the test workspace from the sidebar via right-click → Remove.

---

## Pass / fail summary

Record outcomes:

| Scenario | What it verifies | Result |
|---|---|---|
| 1 | No allow-list = allow all (back-compat) | ✅ pass (2026-04-08) |
| 2 | Exact match blocks with 403 | ✅ pass (2026-04-08) |
| 3 | `*.stripe.com` rejects bare `stripe.com` | ✅ pass (2026-04-08) |
| 4 | Case-insensitive matching | ✅ pass (2026-04-08) |
| 5 | Allow-list wins over passthrough (ordering invariant) | ✅ pass (2026-04-08) |
| 6 | GUI global allow-list + Anthropic guardrail | ☐ pass / ☐ fail (GUI, pending) |
| 7a | Workspace override guardrail chaining | ☐ pass / ☐ fail (GUI, pending) |
| 7b | Claude Code end-to-end under restrictive allow-list | ☐ pass / ☐ fail (GUI, pending) |

**Scenarios 2, 3, and 5 MUST pass before merging.** A failure in scenario 5 is a CRITICAL security regression — do not merge under any circumstances. A failure in scenario 7a means the guardrail-chaining fix has regressed and users can silently commit broken configurations.
