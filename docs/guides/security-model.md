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
- Only the workspace directory is mounted read-write
- `~/.claude/` is mounted read-only (for authentication)
- No direct network access (internal Docker network)

**Blast radius:** Even in the worst case, damage is limited to the mounted workspace directory.

### Layer 2: Secret Encryption

All secrets are encrypted with [age](https://age-encryption.org/) (X25519) before entering the container:

```
Host:      STRIPE_KEY=sk_live_abc123
Container: STRIPE_KEY=ENC[age:YWdlLWVuY3J5cHRpb24...]
```

The agent sees only `ENC[age:...]` ciphertext. Even if it copies the value into source code or sends it to an LLM, the actual secret remains protected.

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

**Passthrough:** Traffic to `api.anthropic.com` and `auth.anthropic.com` is not intercepted (no MITM on Claude API communication).

## What This Protects

| Threat | Protected? | How |
|--------|-----------|-----|
| Secret in LLM prompt | Yes | Agent only has ciphertext |
| Secret in generated code | Yes | Code contains `ENC[age:...]`, not real keys |
| Secret pushed to public repo | Yes | Encrypted values are safe to publish |
| Unauthorized API calls | Partially | Proxy routes all traffic, could add allowlists |
| Container breakout | Partially | cap-drop=ALL, but kernel exploits possible |
| Host compromise | No | Private key is on the host |

## Known Limitations

- **Client-side crypto operations** (HMAC signing, AWS Signature V4) require the real key at computation time. The proxy cannot help here since it only replaces values in transit.
- **Non-HTTP protocols** (direct database connections, gRPC without HTTP/2 proxy) are not intercepted.
- **Binary request bodies** are skipped (no UTF-8 decoding attempted).
- **Only one workspace session at a time** due to hard-coded container names in the current implementation.

## Recommendations for Enterprise Deployment

1. Run Docker with user namespace remapping for additional isolation
2. Use a dedicated Docker network per project (future enhancement)
3. Mount workspace on a tmpfs volume for ephemeral sessions
4. Rotate age keys periodically
5. Monitor proxy logs for unexpected outbound destinations
6. Consider adding a host allowlist to the proxy configuration
