# Airlock Documentation

Airlock is a security isolation layer for AI coding agents. It runs agents inside Docker containers where secrets are always encrypted, with a transparent decryption proxy at the network boundary.

## Directory Structure

| Directory | Purpose | When to use |
|-----------|---------|-------------|
| `glossary/` | Authoritative definitions of core concepts and terms | When introducing a new concept or when a term's meaning is ambiguous |
| `decisions/` | Architecture Decision Records (ADRs) | When making a design choice that affects the project's direction |
| `specs/` | Design specifications and technical designs | When designing a feature or system component before implementation |
| `plans/` | Implementation plans with phased steps | When breaking a spec into executable work |
| `guides/` | User-facing documentation: installation, security model, troubleshooting | When writing how-to content for end users or operators |
| `superpowers/plans/` | Auto-generated implementation plans from superpowers skills | When superpowers skills generate detailed task-level plans |

## Documentation Governance

1. **Authoritative source**: `docs/` contains the authoritative design decisions, definitions, and specifications. Implementation follows documentation, not the other way around.
2. **Scope boundary**: `docs/` is for project-level knowledge -- concepts, processes, protocols, designs, and decisions that define the project's identity and direction. Development-time artifacts (test fixtures, smoke test docs, helper script notes, temporary analysis) belong in `.dev/`, not here.
3. **Concept lookup**: When a concept is ambiguous, check `docs/glossary/` before proceeding. If no entry exists, create one.
4. **Docs-first updates**: Update `docs/` before changing implementation. This prevents drift between what's documented and what's built.
5. **Glossary authority**: Glossary definitions are authoritative. If code behavior diverges from a glossary definition, the code needs to change.
6. **Decision records**: Every design or architectural decision must be recorded as an ADR in `docs/decisions/` using the format `ADR-NNNN-<slug>.md`. Check existing ADRs before making decisions that might conflict.
7. **Operational findings**: Compatibility discoveries, runtime environment issues, and deployment gotchas belong in `docs/guides/troubleshooting.md` as operational knowledge. Do not create standalone findings files -- integrate into the existing troubleshooting guide.
8. **Issue tracking**: Bugs and feature requests discovered during testing or development go to GitHub Issues, not `docs/`. Reference issue numbers from ADR revisions when relevant.

## ADR Index

| ADR | Title | Status |
|-----|-------|--------|
| [ADR-0001](decisions/ADR-0001-docs-first-workflow.md) | Documentation-first workflow | Accepted |
| [ADR-0002](decisions/ADR-0002-agent-agnostic-security-layer.md) | Agent-agnostic security layer with Claude Code first | Accepted |
| [ADR-0003](decisions/ADR-0003-open-source-positioning.md) | Open-source positioning as complementary security layer | Accepted |
| [ADR-0004](decisions/ADR-0004-gui-primary-interface.md) | GUI as primary interface, CLI as engine | Accepted |
| [ADR-0005](decisions/ADR-0005-settings-secret-protection.md) | Settings secret protection with modular Scanner pipeline | Accepted |
| [ADR-0006](decisions/ADR-0006-writable-claude-volume.md) | Writable .claude volume for persistent container state | Accepted |

## Guides Index

| Guide | Audience |
|-------|----------|
| [Installation](guides/installation.md) | Users setting up airlock for the first time |
| [Security Model](guides/security-model.md) | Security teams evaluating airlock, enterprise operators |
| [Troubleshooting](guides/troubleshooting.md) | Users encountering issues |

## Reading Order

1. This README (governance and structure)
2. [ADR-0004](decisions/ADR-0004-gui-primary-interface.md) (GUI-primary architecture -- key strategic decision)
3. `guides/installation.md` (GUI app setup)
4. `guides/` (security model, troubleshooting)
5. `glossary/` entries (understand the vocabulary)
6. `decisions/` (understand why things are the way they are)
7. `specs/` (understand current design -- see `superpowers/specs/2026-03-25-gui-primary-redesign.md` for the active spec)
8. `plans/` (understand what's being built)
