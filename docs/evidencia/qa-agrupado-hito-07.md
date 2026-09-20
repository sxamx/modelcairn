# Milestone 7 grouped QA

[Español](qa-agrupado-hito-07.es.md)

- Date: September 19, 2026
- Final scope: independent Phase 1 closeout review on PR #26
- Approved revision: `bbdb472`
- Verdict: **approved, with no open blockers**

The first review blocked closeout. It found overly permissive onboarding
capabilities, missing quantitative budgets, insufficient egress/redirect
traceability, incomplete canary coverage, and partial lost-response recovery.
Those findings were corrected and reviewed again.

The second review found one additional high blocker: the console proposed revoking
and reissuing the same `AgentToken`, which the backend forbids. Atomic, repeatable
rotation was added. Tests prove the previous bearer becomes invalid, only the
newest works, a second rotation recovers another lost response, and unissued,
expired, or revoked identities cannot rotate. The Linux gate exercised the real
endpoint, required 401 for the former token, and used the replacement for normal
and SSE routes.

The final re-review confirmed:

- green CI tests and Linux AMD64/ARM64 builds;
- all 17 web tests and complete Go suites passing;
- the 600-second benchmark within every budget;
- executable egress inventory and no external telemetry;
- conservative `text` capability with opt-in streaming/tools;
- apply and lost-token recovery consistent with backend behavior;
- canaries absent from persistence, logs, and sensitive artifacts.

The only low observation was stale console wording that still said “revoke”; it
was corrected to “rotate”. No critical, high, or medium finding remains that would
invalidate the evidence or Phase 1 acceptance.
