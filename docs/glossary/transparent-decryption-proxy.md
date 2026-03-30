# Transparent Decryption Proxy

A mitmproxy sidecar container that intercepts all outbound HTTP/HTTPS traffic from the agent container and replaces `ENC[age:...]` patterns with their decrypted plaintext values before forwarding to the destination.

Key properties:
- **Transparent**: The agent does not know the proxy exists. It uses encrypted values as-is; the proxy handles substitution.
- **Boundary-only**: Decryption occurs only at the network boundary, never inside the agent's container.
- **Full coverage by default**: All outbound traffic goes through decryption. No hosts are excluded by default (see [ADR-0005](../decisions/ADR-0005-settings-secret-protection.md)). Users can opt in to passthrough for specific hosts via `--passthrough-hosts` CLI flag, `config.yaml`, or the GUI Settings. When the flag is explicitly passed (even as empty string), it overrides `config.yaml` defaults.
- **Response audit logging**: Logs response metadata (status code, content type, size) for all traffic. Response body content is never logged.
- **Hot-reload**: The proxy checks the mapping file's modification time on each non-passthrough request. If the file has changed, it reloads automatically without restart.
- **Limitation**: Cannot handle authentication schemes where the key is used in computation (e.g., HMAC signing, AWS Signature V4) rather than simple header/body substitution.
