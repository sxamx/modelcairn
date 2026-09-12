# Online CLI contract v1

[Español](cli-online-v1.es.md)

Status: session foundation implemented; operation parity pending.

The online CLI exclusively uses the administrative API and never opens the data
directory. `--server` is the exact public origin without path, query, or userinfo.
HTTPS is mandatory except for loopback HTTP, and redirects are never followed.

```text
modelcairn admin login --server origin --username name [--session-file path]
modelcairn admin whoami --server origin [--session-file path]
modelcairn admin logout --server origin [--session-file path]
```

Login reads the password from a no-echo terminal or stdin, never argv. The session
is stored beneath the user's configuration directory in a POSIX `0600` file;
`--session-file` selects an explicit location. It contains cookie, CSRF, origin,
and expiry, so it is a local credential that must not be versioned or shared.
`whoami` recovers/rotates CSRF and updates the file. Logout confirms server-side
revocation before deleting the local copy.

Every request sends the exact Origin, cookie, and CSRF, bounds responses to 1 MiB,
and uses a timeout. Cookie and CSRF are never printed. Future config, secret, and
AgentToken commands reuse this client rather than defining another protocol.
