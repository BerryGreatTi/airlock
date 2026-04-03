# Transparent Decryption Proxy

A mitmproxy sidecar container that intercepts all outbound HTTP/HTTPS traffic from the agent container and replaces `ENC[age:...]` patterns with their decrypted plaintext values before forwarding to the destination.

Key properties:
- **Transparent**: The agent does not know the proxy exists. It uses encrypted values as-is; the proxy handles substitution.
- **Boundary-only**: Decryption occurs only at the network boundary, never inside the agent's container.
- **Passthrough behavior**: The GUI defaults to passthrough for Anthropic API hosts (`api.anthropic.com`, `auth.anthropic.com`) so that `ENC[age:...]` secrets remain encrypted in Claude Code traffic. The CLI defaults to no passthrough (all traffic decrypted). Users can configure passthrough via `--passthrough-hosts` CLI flag, `config.yaml`, or the GUI Settings. When the flag is explicitly passed (even as empty string), it overrides `config.yaml` defaults. See [ADR-0005 Revision 2026-04-03](../decisions/ADR-0005-settings-secret-protection.md) for rationale.
- **Response audit logging**: Logs response metadata (status code, content type, size) for all traffic. Response body content is never logged.
- **Hot-reload**: The proxy checks the mapping file's modification time on each non-passthrough request. If the file has changed, it reloads automatically without restart.
- **Limitation**: Cannot handle authentication schemes where the key is used in computation (e.g., HMAC signing, AWS Signature V4) rather than simple header/body substitution.
