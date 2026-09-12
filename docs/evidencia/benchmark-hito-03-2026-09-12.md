# Milestone 3 representative benchmark

[Español](benchmark-hito-03-2026-09-12.es.md)

- Status: passed on the target VM
- Date: September 12, 2026
- Environment: Linux x86_64, 2 logical CPUs, 975,064 KiB total RAM
- Stripped binary: 13,103,264 bytes
- Flow: bootstrap, Argon2id login, CSRF recovery, secret, configuration
  plan/apply/export, AgentToken issue/revoke, and logout
- Additional steady state: 120 seconds
- Samples: 2,136, approximately every 50 ms
- Average service RSS: 55,358 KiB
- Peak service RSS: 55,368 KiB
- Peak service swap: 0 KiB
- System memory available after the flow: 512,692 KiB

Sampling covered login to capture the Argon2id allocation and kept the real
administrative server running. The integration flow also confirmed that password
and API key did not enter logs/export and that logout removed the local session.
The VM had no Go toolchain, so the gate verified and used a precompiled Linux binary.

Public evidence omits hostname, IP, user, temporary paths, credentials, and revision
identifiers. RSS excludes kernel page cache and other services. With a peak near
54.1 MiB, the administrative plane retains ample margin on the approximately 1 GiB VM.
