# Manual test runbook: env secrets + passthrough guardrail

> Companion to the spec (`2026-04-07-env-secrets-and-passthrough-guardrail-design.md`)
> and the plan (`2026-04-07-env-secrets-and-passthrough-guardrail.md`).
> Executes three end-to-end scenarios against a live Airlock session
> before merging `feat/env-secrets-and-passthrough-guardrail`.

## Prerequisites

- Docker Desktop (or Colima/Rancher Desktop) running
- `bin/airlock` built from the branch HEAD: `make build`
- Airlock GUI built: `make gui-build` (optional; CLI suffices for scenarios 2 and 3)
- An empty scratch directory for each subscenario; do NOT reuse an existing project workspace
- A throwaway secret value (use a fake token like `ghp_fake_abcd1234_test`, not a real GitHub token)
- `jq` installed (for parsing `httpbin.org` JSON responses)

## Verification philosophy

Each scenario has one observable outcome that directly maps to a security property:

| Scenario | Property verified |
|---|---|
| 1 | Anthropic passthrough is the default; no regression of commit `7390166` |
| 2 | Proxy substitutes ciphertext → plaintext on the wire to non-Anthropic hosts (so real APIs authenticate) |
| 3 | Proxy does NOT substitute on the wire to Anthropic (so the model never receives plaintext secrets) |

All three are pass/fail on a single log line or HTTP response. If any step deviates from the "Expected" text, stop and investigate — do not proceed to the next scenario with a contaminated state.

---

## Scenario 1 — New workspace: default passthrough settings

**Goal:** A freshly added workspace inherits the default passthrough list (`api.anthropic.com`, `auth.anthropic.com`), the Secrets tab shows no warning banner, and the global Settings editor shows both Anthropic hosts populated.

**Setup:** GUI only. The CLI does not have the concept of "workspace registration" — that lives in the GUI's `WorkspaceStore`.

### Steps

- [ ] **1.1** Launch the GUI: `make gui-run` (or double-click `build/Airlock.app` if `make gui-package` was run earlier).
- [ ] **1.2** Note the currently registered workspaces (if any). Do NOT delete them.
- [ ] **1.3** Click the `+` button (or `File → New Workspace`) to add a workspace.
- [ ] **1.4** Choose a scratch directory, e.g., `mkdir -p /tmp/airlock-test-scenario-1 && pick that`.
- [ ] **1.5** Select the new workspace in the sidebar. Do NOT click "Activate" yet.
- [ ] **1.6** Switch to the **Settings** tab (Cmd+4) for this workspace.
- [ ] **1.7** Find the "Network Overrides" section. **Expected:**
    - Caption reads `Default: api.anthropic.com, auth.anthropic.com`
    - The override `TextEditor` is EMPTY (empty override = inherit global)
    - NO yellow inline warning row below the editor
- [ ] **1.8** Switch to the **Secrets** tab (Cmd+2) for this workspace. **Expected:**
    - NO yellow "⚠ Anthropic passthrough disabled" banner at the top of the view
    - The "Env Variables" sidebar section is visible and empty
- [ ] **1.9** Open global Settings (gear icon / menu `Airlock → Settings...`).
- [ ] **1.10** Find the "Network Defaults" section. **Expected:**
    - The `TextEditor` contains TWO lines: `api.anthropic.com` and `auth.anthropic.com`
    - NO yellow inline warning row below the editor
- [ ] **1.11** Close the Settings sheet with Cancel (do not save).

### Expected outcome

All four UI surfaces (workspace Settings, workspace Secrets, global Settings editor, global Settings warning) are clean. The workspace will activate with Anthropic in passthrough.

---

## Scenario 2 — Non-passthrough decryption (wire substitution)

**Goal:** Secrets registered via both file and env-var paths reach non-Anthropic backends as PLAINTEXT because the proxy substitutes `ENC[age:...]` → plaintext on the wire.

**Test target:** `httpbin.org/headers` (echoes request headers verbatim in the response JSON). Not in the default passthrough list; proxy will intercept.

**Setup (do once before both subscenarios):**

```bash
mkdir -p /tmp/airlock-test-scenario-2 && cd /tmp/airlock-test-scenario-2
/Users/berry.kim/Projects/airlock/bin/airlock init
```

Keep this terminal tab open for running `docker logs` out-of-band.

### Subscenario 2a — File secret → non-passthrough host

- [ ] **2a.1** In the workspace directory, create a `.env` file with a fake token:
    ```bash
    printf 'GITHUB_TOKEN=ghp_fake_abcd1234_test\n' > .env
    ```
- [ ] **2a.2** Register and encrypt the file:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock secret add .env
    /Users/berry.kim/Projects/airlock/bin/airlock secret encrypt .env --keys GITHUB_TOKEN
    ```
- [ ] **2a.3** Verify the on-disk file is ciphertext:
    ```bash
    cat .env
    ```
    **Expected:** `GITHUB_TOKEN=ENC[age:...]` (not the fake token). If you see `ghp_fake_abcd1234_test`, STOP — encryption failed.
- [ ] **2a.4** Start a session:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock run
    ```
    Wait for `Starting Claude Code...` and the container shell. (Press `Ctrl+C` to exit Claude if you want a bare shell; alternatively use `airlock start --id test2a` then `docker exec -it airlock-claude-test2a bash` in a sidecar terminal.)
- [ ] **2a.5** Inside the container, inspect the shadow-mounted file:
    ```bash
    cat /workspace/airlock-test-scenario-2/.env
    ```
    **Expected:** `GITHUB_TOKEN=ENC[age:...]` (ciphertext visible to the agent).
- [ ] **2a.6** Source it and make an HTTP call to a non-passthrough host:
    ```bash
    set -a; . /workspace/airlock-test-scenario-2/.env; set +a
    echo "token as agent sees it: $GITHUB_TOKEN"
    curl -s -H "Authorization: Bearer $GITHUB_TOKEN" https://httpbin.org/headers | jq -r '.headers.Authorization'
    ```
    **Expected:**
    - First `echo` prints `token as agent sees it: ENC[age:...]`
    - `curl` + `jq` prints `Bearer ghp_fake_abcd1234_test` — **plaintext**, because the proxy substituted on the wire.
- [ ] **2a.7** In a second terminal on the host (outside the container), inspect the proxy log:
    ```bash
    docker ps --format '{{.Names}}' | grep airlock-proxy
    docker logs $(docker ps --format '{{.Names}}' | grep airlock-proxy) 2>&1 | grep httpbin.org
    ```
    **Expected:** At least one JSON line with `"host":"httpbin.org","action":"decrypt","location":"header","key":"Authorization"`.
- [ ] **2a.8** Exit the session (`exit` in the container shell; `Ctrl+C` if the CLI is in attached mode). Clean up: `/Users/berry.kim/Projects/airlock/bin/airlock stop --id test2a` if you used detached mode.

### Subscenario 2b — Env-var secret → non-passthrough host

- [ ] **2b.1** In the host shell (still in the workspace directory), first decrypt the `.env` file from 2a so it doesn't interfere:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock secret decrypt .env --all
    /Users/berry.kim/Projects/airlock/bin/airlock secret remove $(pwd)/.env
    rm -f .env
    ```
- [ ] **2b.2** Register an env-secret instead:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock secret env add GITHUB_TOKEN --value ghp_fake_abcd1234_test
    ```
    **Expected:** `Added env secret GITHUB_TOKEN`
- [ ] **2b.3** Verify ciphertext-at-rest and no plaintext leak:
    ```bash
    grep -F 'ENC[age:' .airlock/config.yaml | head -1 && echo "ciphertext OK"
    grep -F 'ghp_fake_abcd1234_test' .airlock/config.yaml && echo "LEAK" || echo "no plaintext OK"
    ```
    **Expected:** `ciphertext OK` and `no plaintext OK`.
- [ ] **2b.4** Start a session:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock run
    ```
- [ ] **2b.5** Inside the container:
    ```bash
    echo "env var as agent sees it: $GITHUB_TOKEN"
    curl -s -H "Authorization: Bearer $GITHUB_TOKEN" https://httpbin.org/headers | jq -r '.headers.Authorization'
    ```
    **Expected:**
    - First `echo` prints `env var as agent sees it: ENC[age:...]`
    - `curl` + `jq` prints `Bearer ghp_fake_abcd1234_test` — plaintext on the wire.
- [ ] **2b.6** In the host sidecar terminal, inspect proxy logs again for a decrypt action on `httpbin.org`.
- [ ] **2b.7** Exit the session and clean up:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock secret env remove GITHUB_TOKEN
    ```

### Expected outcome (both subscenarios)

The agent sees `ENC[age:...]` in both the file and the environment; `httpbin.org` receives `ghp_fake_abcd1234_test` (plaintext); the proxy logs a `decrypt` action for each outbound request. This is the "secret reaches real backend" loop working as designed.

---

## Scenario 3 — Passthrough encryption (ciphertext preserved to Anthropic)

**Goal:** Secrets registered via both file and env-var paths reach `api.anthropic.com` as CIPHERTEXT — the model never sees plaintext — because Anthropic is in the passthrough list.

**Test target:** A Claude Code prompt that induces Claude to read and echo back the secret. Because Anthropic is in passthrough, the proxy does NOT substitute, so Anthropic's servers (and therefore the model's conversation) see only `ENC[age:...]`.

**Setup:**

```bash
mkdir -p /tmp/airlock-test-scenario-3 && cd /tmp/airlock-test-scenario-3
/Users/berry.kim/Projects/airlock/bin/airlock init
```

You will need a working Claude Code login inside the airlock container. If this is a fresh volume, run `airlock config import` first or log in manually on first session start.

### Subscenario 3a — File secret → Claude API

- [ ] **3a.1** Create and register the secret file:
    ```bash
    printf 'GITHUB_TOKEN=ghp_fake_abcd1234_test\n' > .env
    /Users/berry.kim/Projects/airlock/bin/airlock secret add .env
    /Users/berry.kim/Projects/airlock/bin/airlock secret encrypt .env --keys GITHUB_TOKEN
    ```
- [ ] **3a.2** Start a session:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock run
    ```
- [ ] **3a.3** Wait for Claude Code to finish booting. Type this prompt verbatim:
    > Please read the file `/workspace/airlock-test-scenario-3/.env` using the Read tool and tell me exactly what it contains. Do not redact or summarize; quote the content literally.
- [ ] **3a.4** **Expected response:** Claude quotes back something containing `GITHUB_TOKEN=ENC[age:...]` (ciphertext). It must NOT contain `ghp_fake_abcd1234_test`.
    - If Claude responds with the plaintext token, STOP — the "model never sees plaintext" property is broken.
    - If Claude responds with `ENC[age:...]`, the security property holds.
- [ ] **3a.5** In a host sidecar terminal, grep the proxy logs for Anthropic action:
    ```bash
    docker logs $(docker ps --format '{{.Names}}' | grep airlock-proxy) 2>&1 | grep anthropic.com | tail -10
    ```
    **Expected:** All lines for `api.anthropic.com` show `"action":"passthrough"`. NO lines show `"action":"decrypt"` for an Anthropic host.
- [ ] **3a.6** Exit the session (`/exit` or `Ctrl+C` twice).

### Subscenario 3b — Env-var secret → Claude API

- [ ] **3b.1** Clean up the file from 3a and register an env-secret:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock secret decrypt .env --all
    /Users/berry.kim/Projects/airlock/bin/airlock secret remove $(pwd)/.env
    rm -f .env
    /Users/berry.kim/Projects/airlock/bin/airlock secret env add GITHUB_TOKEN --value ghp_fake_abcd1234_test
    ```
- [ ] **3b.2** Start a session:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock run
    ```
- [ ] **3b.3** At the Claude Code prompt, type verbatim:
    > Run the bash command `echo "$GITHUB_TOKEN"` using the Bash tool and quote the output literally.
- [ ] **3b.4** **Expected response:** Claude reports the command output as `ENC[age:...]`. It must NOT contain `ghp_fake_abcd1234_test`.
- [ ] **3b.5** In the host sidecar terminal, re-check proxy logs:
    ```bash
    docker logs $(docker ps --format '{{.Names}}' | grep airlock-proxy) 2>&1 | grep anthropic.com | tail -10
    ```
    **Expected:** All `api.anthropic.com` actions are `passthrough`. NO `decrypt` actions on Anthropic hosts.
- [ ] **3b.6** Exit the session and clean up:
    ```bash
    /Users/berry.kim/Projects/airlock/bin/airlock secret env remove GITHUB_TOKEN
    ```

### Expected outcome (both subscenarios)

Claude's response quotes `ENC[age:...]` to you (via Anthropic), NOT the plaintext token. Proxy logs show `passthrough` action for every Anthropic request. This confirms that the cloud model and Anthropic's servers only ever observe ciphertext.

---

## Global cleanup

After completing all three scenarios:

```bash
cd /tmp
rm -rf airlock-test-scenario-1 airlock-test-scenario-2 airlock-test-scenario-3
docker ps --format '{{.Names}}' | grep '^airlock-' | xargs -r docker stop
docker ps -a --format '{{.Names}}' | grep '^airlock-' | xargs -r docker rm
```

If you used the GUI in Scenario 1, remove the `/tmp/airlock-test-scenario-1` workspace from the GUI sidebar (right-click → Remove).

---

## Pass / fail summary

Record outcomes:

| Scenario | Subscenario | Result |
|---|---|---|
| 1 | default passthrough in new workspace | ☐ pass / ☐ fail |
| 2a | file secret → httpbin (plaintext on wire) | ☐ pass / ☐ fail |
| 2b | env-var secret → httpbin (plaintext on wire) | ☐ pass / ☐ fail |
| 3a | file secret → Anthropic (ciphertext preserved) | ☐ pass / ☐ fail |
| 3b | env-var secret → Anthropic (ciphertext preserved) | ☐ pass / ☐ fail |

**All five must pass before merging this branch.** Any failure in Scenario 3 is a CRITICAL security regression — do not merge under any circumstances.
