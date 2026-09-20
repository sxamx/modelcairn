"use strict";

const fs = require("node:fs");
const workflow = fs.readFileSync(".github/workflows/publish-release.yml", "utf8");

for (const required of [
  "workflow_dispatch:",
  "environment: release",
  "actions: read",
  "attestations: read",
  "contents: write",
  'PUBLISH modelcairn $VERSION',
  ".github/workflows/release-candidate.yml",
  "git rev-parse origin/main",
  "sha256sum --check --strict",
  "gh attestation verify",
  "gh release create",
  "does not promise a stable API",
]) {
  if (!workflow.includes(required)) throw new Error(`publication gate missing: ${required}`);
}

for (const forbidden of [
  /\bgo build\b/,
  /package-release\.sh/,
  /\bcurl\b/,
  /\bwget\b/,
  /secrets\./,
  /pull_request:/,
  /^\s+push:/m,
]) {
  if (forbidden.test(workflow)) throw new Error(`unsafe publication pattern: ${forbidden}`);
}

console.log("Publication workflow satisfies promotion-only safety invariants.");
