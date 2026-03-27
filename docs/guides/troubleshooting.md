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

Claude API traffic (`api.anthropic.com`, `auth.anthropic.com`) should pass through without MITM. If intercepted:
1. Check `.airlock/config.yaml` includes anthropic hosts in `passthrough_hosts`
2. Default config should have them. If missing, add:
   ```yaml
   passthrough_hosts:
     - api.anthropic.com
     - auth.anthropic.com
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
