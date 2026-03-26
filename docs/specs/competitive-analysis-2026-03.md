# Competitive Analysis: Conductor vs Airlock (March 2026)

## Conductor Overview

**Source**: https://docs.conductor.build/

Conductor is a macOS desktop app for orchestrating teams of coding agents. It creates isolated workspaces (git worktree copies) for parallel agent execution and manages the full Issue-to-PR development workflow.

### Key Features

- Git-tracked file copy as workspace isolation
- Parallel agent execution (multiple Claude Code instances)
- Checkpoint system (git ref-based snapshots/rollback)
- Diff viewer + GitHub PR workflow
- Scripts automation (setup/run/archive)
- Multi-provider support (Anthropic, OpenRouter, Bedrock, Vertex, Azure)
- IDE integration (Cursor, VSCode)
- Free (seed-funded, paid team features planned)

### Security Model

**Effectively none.** Key findings from their documentation:

- FAQ: "Agents in Conductor run with the same permissions as your user account. They can read and write files, run shell commands, and access anything you can access on your machine."
- `.env` files are copied in plaintext to workspaces via setup scripts
- No network isolation
- No sandboxing
- No secret encryption
- Privacy: chat data stored locally, not sent to Conductor servers

## Comparison

| Aspect | Conductor | Airlock |
|--------|-----------|---------|
| Primary focus | UX / workflow orchestration | Security / isolation |
| Isolation level | Git worktree (file copy) | Docker container (OS-level) |
| Secret handling | Plaintext .env copy | age-encrypted, never plaintext in container |
| Network isolation | None | mitmproxy transparent proxy |
| Security model | Same as user permissions | Least-privilege container |
| Platform | macOS only | Cross-platform (Docker) |
| Interface | GUI (desktop app) | macOS GUI + CLI |
| Agent support | Multi-provider + Codex | Claude Code first, extensible |
| Workflow scope | Issue to PR (full cycle) | Session-based (run/stop) |
| Pricing | Free (seed-funded) | Open-source |

## Key Insight

**Conductor and airlock are complementary, not competing.** Conductor solves "how to manage agents comfortably" while airlock solves "how to run agents securely." No tool in the current market addresses the security isolation gap that airlock targets.

## Airlock Differentiators

1. **Secrets never exist as plaintext in agent environment** -- core differentiator vs all existing tools
2. **OS-level container isolation** -- true blast radius containment vs git file copying
3. **Network boundary security** -- mitmproxy decrypts only at proxy, auditable traffic
4. **Cross-platform** -- anywhere Docker runs
5. **Enterprise security team enabler** -- provides the guarantees security teams need to approve AI agent adoption

## Airlock Weaknesses

1. **Adoption friction** -- requires Docker installation and setup
2. **HMAC/signature auth** -- keys used in computation (AWS Signature V4) unsupported by current proxy design
3. **Platform risk** -- Anthropic could build native container isolation into Claude Code
4. **Cross-platform GUI** -- macOS GUI available, Linux users rely on CLI
