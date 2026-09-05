# Clean repository preparation

[Español](preparacion-del-repositorio-limpio.es.md)

- Status: clean clone and allowlist completed; commit and push pending
- Objective: publish ModelCairn without the history or discarded files from the previous prototype

## Constraint

The previous local workspace retains the prototype's Git history. This
`modelcairn` directory was cloned from the empty remote and does not contain that history.

## Proposed safe procedure

1. Rename the remote repository to `modelcairn`. **Completed.**
2. Authenticate `gh` with the correct public account. **Completed.**
3. Verify `user.name` and `user.email` locally without publishing them in documents.
4. Retain the previous directory as a local archive. **Completed.**
5. Clone the empty remote into a new `modelcairn` directory. **Completed.**
6. Copy only current documentation and approved public files through an allowlist.
   **Completed.**
7. Add `LICENSE`, `NOTICE`, `.gitignore`, README, and index. **Completed.**
8. Search for secrets, local paths, private identifiers, and obsolete product references.
9. Validate JSON Schema, OpenAPI, SQL, links, and Markdown.
10. Create one initial documentation commit with the correct identity.
11. Review the diff and local commit before requesting authorization to push.

Moving the directory is recoverable; the local archive and existing backup will
not be deleted. Push will be a separate, explicit action.

## Initial documentation allowlist

- `README.md`, `LICENSE`, `NOTICE`, `.gitignore`;
- `docs/README.es.md`;
- charter, status, glossary, requirements, and target architecture;
- ADR-0001 through ADR-0006;
- public initial audit and disposition matrix;
- all current documents and contracts under `docs/fases/` and `docs/contratos/`.

No code, database, screenshots, conversation files, or prototype documents will
be copied unless specifically reviewed and decided during each milestone.
