# Transparent Decryption Proxy

A mitmproxy sidecar container that intercepts all outbound HTTP/HTTPS traffic from the agent container and replaces `ENC[age:...]` patterns with their decrypted plaintext values before forwarding to the destination.

Key properties:
- **Transparent**: The agent does not know the proxy exists. It uses encrypted values as-is; the proxy handles substitution.
- **Boundary-only**: Decryption occurs only at the network boundary, never inside the agent's container.
- **Passthrough for Claude API**: Traffic to Anthropic's API is passed through without modification (the API key is handled separately).
- **Limitation**: Cannot handle authentication schemes where the key is used in computation (e.g., HMAC signing, AWS Signature V4) rather than simple header/body substitution.
