# ADR-0003: Open-source positioning as complementary security layer

## Status

Accepted

## Context

A competitive analysis of Conductor (docs.conductor.build, March 2026) revealed that existing AI agent orchestration tools focus on UX/workflow (workspace management, parallel agents, diff viewing, PR workflows) but provide zero security isolation. Conductor's FAQ states: "Agents run with the same permissions as your user account." No tool in the market addresses the security gap.

Enterprise security teams block or heavily restrict AI agent adoption because:
- Agents access plaintext secrets (.env files)
- Agents run with full user permissions (filesystem, network, processes)
- No audit trail for agent network activity
- No blast radius containment if an agent behaves unexpectedly

This friction is not theoretical -- the project maintainer experienced it firsthand when deploying Claude Code skills within their organization.

## Decision

Position airlock as an **open-source security layer** that complements (not competes with) existing orchestration tools:

1. **Not a product**: No monetization. Open-source project seeking community contribution and recognition.
2. **Complementary**: Designed to work alongside tools like Conductor, not replace them. "Conductor + airlock" should be a valid stack.
3. **Enterprise-friendly**: The primary value proposition is reducing friction with enterprise security teams by providing:
   - Container isolation (blast radius containment)
   - Encrypted secrets (no plaintext in agent environment)
   - Network proxy (auditable, controllable outbound traffic)
4. **Three sentences that change a security meeting**: "Agents operate only inside containers. Secrets exist only in encrypted form. Network traffic is proxied and auditable."

## Consequences

- Project roadmap prioritizes security completeness over feature breadth
- Documentation and README should emphasize the enterprise security use case
- Integration guides for popular orchestration tools (Conductor, etc.) are valuable contributions
- Community adoption depends on the project being easy to evaluate and trust (clear security model, minimal dependencies, auditable code)

## Alternatives Considered

- **Commercial product**: Ruled out -- maintainer's goal is community contribution, not revenue
- **Standalone orchestration tool**: Would compete with well-funded tools (Conductor) on UX, where airlock has no advantage
- **Anthropic-specific tool**: Artificially limits community; the security problem is universal across all AI agents
