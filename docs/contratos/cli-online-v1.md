# Online CLI contract v1

[Español](cli-online-v1.es.md)

Status: functional parity implemented for sessions, configuration, secrets, and
the AgentToken lifecycle.

The online CLI exclusively uses the administrative API and never opens the data
directory. `--server` is the exact public origin without path, query, or userinfo.
HTTPS is mandatory except for loopback HTTP, and redirects are never followed.

```text
modelcairn admin login --server origin --username name [--session-file path]
modelcairn admin whoami --server origin [--session-file path]
modelcairn admin logout --server origin [--session-file path]
modelcairn agent-token status --server origin [--session-file path] <name>
modelcairn agent-token issue --server origin [--session-file path] <name>
modelcairn agent-token revoke --server origin [--session-file path] <name>
modelcairn config plan --server origin [--session-file path] [--allow-delete] [--out plan.json] <file>
modelcairn config apply --server origin [--session-file path] [--allow-delete] --plan plan.json <file>
modelcairn config export --server origin [--session-file path]
modelcairn secret set --server origin [--session-file path] [--version n] <name>
modelcairn secret metadata --server origin [--session-file path] [name]
modelcairn secret delete --server origin [--session-file path] --version n <name>
```

Login reads the password from a no-echo terminal or stdin, never argv. The session
is stored beneath the user's configuration directory in a POSIX `0600` file;
`--session-file` selects an explicit location. It contains cookie, CSRF, origin,
and expiry, so it is a local credential that must not be versioned or shared.
`whoami` recovers/rotates CSRF and updates the file. Logout confirms server-side
revocation before deleting the local copy.

Every request sends the exact Origin, cookie, and CSRF, bounds responses to 1 MiB,
and uses a timeout. A 403 triggers CSRF recovery through `session/me` and exactly
one retry. Cookie and CSRF are never printed; `agent-token issue` is the sole
intentional one-time bearer output. Online configuration preserves plan/apply,
the private plan file, and exact input binding; `--server` cannot be mixed with
`--data-dir`. Secrets preserve no-echo input, preconditions, and metadata-only
responses. Master-key rotation remains local/offline.
