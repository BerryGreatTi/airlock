# ADR-0002: Agent-agnostic security layer with Claude Code first

## Status

Accepted

## Context

Airlock's security isolation (container + encrypted secrets + network proxy) solves a problem common to all AI coding agents, not just Claude Code. Tools like Conductor, Codex CLI, Aider, and Cursor all face the same issue: agents run with the user's full permissions and have access to plaintext secrets.

However, the project maintainer's organization has standardized on Claude Code, providing an immediate validation environment. Building for all agents at once risks over-engineering and delaying the first usable release.

## Decision

1. **Design for agent-agnosticism**: The container isolation, secret encryption, and network proxy layers must not have Claude Code-specific assumptions baked in. The agent to run inside the container should be configurable.
2. **Implement Claude Code first**: The initial release targets Claude Code as the primary agent. Claude Code-specific configuration (container image, entrypoint, default settings) is the first implementation, not a hardcoded assumption.
3. **Expand later**: Support for other agents (Codex, Aider, Cursor, etc.) will follow once the core security layer is validated with Claude Code in production.

## Consequences

- Container runtime interface and orchestrator should accept agent configuration as a parameter
- Claude Code-specific details (image, entrypoint, CLI flags) live in configuration, not in core logic
- First release is simpler and immediately testable in a real environment
- Other agents can be added without architectural changes

## Alternatives Considered

- **Claude Code only**: Simpler but artificially limits the project's usefulness and community adoption
- **All agents from day one**: Higher initial complexity, risk of building abstractions for untested use cases
- **Agent plugin system**: Over-engineering for the current stage; configuration-based approach is sufficient
