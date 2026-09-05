# Prototype disposition matrix

[Español](matriz-de-disposicion-del-prototipo.es.md)

- Status: cleanup decision prior to the clean repository
- Rule: do not copy previous code or documentation by default

| Previous material | Disposition | Rationale |
|---|---|---|
| `src/`, `public/`, scripts, and tests | local reference; reassess per milestone | they belong to a different architecture/name and do not prove the new contracts |
| `docs/architecture.es.md` | replaced | contradicts the current central custody and relay model |
| `docs/control-plane.es.md` | partial reference | may contribute cases, but is not a current contract |
| `docs/deployment.es.md`, `docs/install.es.md` | replace during Milestone 6 | describe prototype deployment |
| `docs/profiles.es.md` | partial reference | some versioning ideas may be migrated with tests |
| `docs/strategy-builder.es.md` | deferred | useful for the visual editor phase, not Phase 1 |
| `docs/web-console.es.md`, screenshots, and UI QA | non-binding visual reference | the console will be redesigned against ADR-0006 |
| `docs/roadmap.es.md`, `docs/roadmap-visual.es.md` | replaced | contain old Aegis phases and claims |
| SQLite databases, previews, and recovered conversation | private; exclude | may contain local data and are not a public source |

Before reusing a module, the responsible milestone must demonstrate compatibility,
licensing, tests, and adaptation cost. Rewriting is not mandatory; reuse is not
automatic either.
