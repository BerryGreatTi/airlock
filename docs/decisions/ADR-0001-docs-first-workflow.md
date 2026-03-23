# ADR-0001: Documentation-first workflow

## Status

Accepted

## Context

As airlock grows, design decisions and concept definitions accumulate across code comments, commit messages, conversations, and memory files. Without a single authoritative source, definitions drift and new sessions restart without shared understanding. The project already has an implementation plan in `docs/superpowers/plans/` and memory files in `.claude/`, but no structured governance for project-level knowledge.

## Decision

Adopt `docs/` as the single authoritative source for design decisions, concept definitions, and specifications. Establish governance rules:

1. `docs/` is authoritative -- implementation follows documentation
2. Scope boundary -- project-level knowledge only; dev artifacts go in `.dev/`
3. Concept lookup via `docs/glossary/`
4. Docs-first updates -- update docs before changing implementation
5. Glossary authority -- code follows glossary, not vice versa
6. ADRs for all architectural decisions

## Consequences

- Every session should read `docs/README.md` for orientation
- Concepts and decisions cannot exist only in code or conversation
- Small upfront cost to writing documentation before code, but pays off as multiple sessions need consistent context
- Clear separation between project knowledge (`docs/`), development artifacts (`.dev/`), and conversation memory (`.claude/`)

## Alternatives Considered

- **CLAUDE.md only**: Too flat, mixes behavioral directives with design knowledge, grows unwieldy
- **Wiki/external docs**: Adds external dependency, harder for AI agents to read inline
- **No formal structure**: Status quo -- leads to knowledge fragmentation across sessions
