# Troubleshooting

## GUI Issues

### App won't open (macOS Gatekeeper)

The app is ad-hoc signed, not notarized. Allow it:
System Settings > Privacy & Security > scroll to "AirlockApp was blocked" > Open Anyway

### Workspace activation fails

1. Ensure Docker Desktop (or Rancher Desktop / Colima) is running
2. Ensure container images are built (`make docker-build`)
3. Check the airlock binary is in `PATH` (or configure in Settings > Airlock binary path)
4. If containers from a previous crash remain, the app will offer cleanup on launch
5. If using Rancher Desktop or Colima, check that the Docker socket is detected (see Docker Issues below)

### App doesn't receive keyboard focus (CLI launch)

When launching via `swift run` or the built binary from a terminal, the app may not capture keyboard focus. Clicking the app window should activate it. This is resolved in builds after 2026-03-27 via `NSApp.setActivationPolicy(.regular)`.

### "Not a git repository" in diff viewer

The diff viewer requires the workspace to be a git repository. Initialize git:
```bash
cd ~/your-project
git init && git add -A && git commit -m "initial"
```

### Korean/CJK input not working in terminal

The container has `LANG=C.UTF-8` set by default (since PR #11). If Korean or other CJK characters are garbled:
1. Verify locale inside the container: `docker exec airlock-claude-{id} bash -c 'echo $LANG'` -- should print `C.UTF-8`
2. If using a custom Dockerfile, ensure `ENV LANG=C.UTF-8` is set
3. Rebuild images if they predate the fix: `make docker-build`

### Terminals not opening after activation

The workspace must be in "Running" (green dot) state. If activation failed silently:
1. Check the Containers tab for error details
2. Try deactivating and reactivating the workspace
3. Check Docker logs: `docker logs airlock-claude-{workspace-id}`

---

## Docker Issues

### "Cannot connect to the Docker daemon"

```
docker init: create docker client: ...
```

Docker is not running, or the Docker socket is not at the expected path. Check:

**Docker is running but airlock can't find it (Rancher Desktop, Colima, etc.):**

The Go Docker SDK reads the `DOCKER_HOST` environment variable but does not read Docker CLI contexts. If you use a non-default Docker runtime, set `DOCKER_HOST`:

```bash
# Rancher Desktop
export DOCKER_HOST=unix://$HOME/.rd/docker.sock

# Colima
export DOCKER_HOST=unix://$HOME/.colima/docker.sock
```

The GUI app auto-detects common socket paths. If your socket is in a non-standard location, set `DOCKER_HOST` before launching the app.

**Docker is genuinely not running:**
- macOS: Open Docker Desktop (or Rancher Desktop, Colima, etc.)
- Linux: `sudo systemctl start docker`

### "image not found: airlock-claude:latest"

Container images have not been built. Run:
```bash
make docker-build
```

### Container won't start / port conflict

Stale containers from a previous session may exist:
```bash
airlock stop
# or manually:
docker rm -f airlock-claude airlock-proxy
docker network rm airlock-net
```

## Encryption Issues

### Quoted values in .env files

Airlock strips surrounding quotes from `.env` values before encryption:

```
# All three produce the same encrypted value for "my_secret":
KEY=my_secret
KEY="my_secret"
KEY='my_secret'
```

If you see quotes appearing in decrypted API responses, ensure both the Go CLI (`bin/airlock`) and GUI app are up to date.

### "load keypair: read private key: no such file or directory"

`.airlock/` is not initialized. Run `airlock init` first.

### "parse env file: open env file: no such file or directory"

The `.env` file path is wrong. Check:
```bash
ls -la .env  # Does it exist?
airlock encrypt .env  # Use the correct path
```

## Proxy Issues

### API calls fail with certificate errors

The proxy's mitmproxy CA certificate must be trusted inside the agent container. Airlock handles this automatically by building a combined CA bundle (`/tmp/airlock-ca-bundle.crt`) and setting `SSL_CERT_FILE`, `CURL_CA_BUNDLE`, and `NODE_EXTRA_CA_CERTS`. If it still fails:
1. Check proxy container is running: `docker ps | grep airlock-proxy`
2. Check CA cert was generated: `docker exec airlock-proxy ls /root/.mitmproxy/`
3. Check bundle exists in agent: `docker exec airlock-claude-{id} ls /tmp/airlock-ca-bundle.crt`
4. Restart the session: deactivate and reactivate the workspace (GUI), or `airlock stop && airlock run --env .env` (CLI)

### Requests to Claude API are being intercepted

By default, all traffic (including Anthropic API) goes through the decryption proxy. This is intentional -- it allows the proxy to replace `ENC[age:...]` tokens in API key headers.

If you want Anthropic traffic to bypass the proxy (e.g., using OAuth instead of API keys):
1. **GUI**: Add `api.anthropic.com` and `auth.anthropic.com` to Settings > Passthrough hosts
2. **CLI**: Pass `--passthrough-hosts "api.anthropic.com,auth.anthropic.com"`
3. **Config**: Add to `.airlock/config.yaml`:
   ```yaml
   passthrough_hosts:
     - api.anthropic.com
     - auth.anthropic.com
   ```

Note: The GUI always passes `--passthrough-hosts` to the CLI, overriding `config.yaml`. An empty passthrough hosts field in Settings means all traffic goes through the proxy.

## Claude Code Authentication

### OAuth login inside container

Claude Code uses OAuth for authentication. On macOS, tokens are stored in Keychain, which is not accessible from inside a Docker container. To authenticate inside the container:

1. Open the terminal tab for the workspace
2. Run `claude auth login`
3. A URL will be displayed -- copy it and open in your host browser
4. Complete the OAuth flow in the browser
5. Copy the callback URL and paste it back into the container terminal

**Known limitation:** The `~/.claude` directory is mounted read-only, so OAuth tokens cannot be persisted. You must re-authenticate after each container restart. See [GitHub issue #15](https://github.com/BerryGreatTi/airlock/issues/15) for progress on a permanent solution.

### "Not logged in" after successful auth login

If `claude auth login` succeeds but `claude` still prompts for login, this is because the read-only `~/.claude` mount prevents token storage. The token is lost immediately after the login flow completes. This is tracked in [issue #15](https://github.com/BerryGreatTi/airlock/issues/15).

Workaround: Use an API key via `.env` file instead of OAuth:
```
ANTHROPIC_API_KEY=sk-ant-...
```

## General

### How to check what's running

```bash
docker ps | grep airlock
docker network ls | grep airlock
```

### How to clean up everything

```bash
airlock stop
docker rm -f airlock-claude airlock-proxy 2>/dev/null
docker network rm airlock-net 2>/dev/null
```

### Logs

- Proxy logs: `docker logs airlock-proxy`
- Claude container logs: `docker logs airlock-claude`
