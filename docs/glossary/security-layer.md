# Security Layer

A transparent middleware that sits between an AI coding agent and the host system, enforcing isolation boundaries without requiring changes to the agent itself.

Airlock's security layer has three components:

1. **Container isolation** -- The agent runs inside a Docker container with limited filesystem and process access. This contains the blast radius if the agent behaves unexpectedly.

2. **Secret encryption** -- Secrets (API keys, tokens, passwords) are stored only in encrypted form (`ENC[age:...]`) inside the container. The agent never sees plaintext secrets.

3. **Network boundary proxy** -- A mitmproxy sidecar intercepts outbound traffic and replaces encrypted placeholders with plaintext values only at the network boundary. The decryption happens outside the agent's environment.

The security layer is designed to be agent-agnostic: it works with any AI coding agent that runs in a terminal (Claude Code, Codex, Aider, etc.).
