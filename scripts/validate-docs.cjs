"use strict";

const fs = require("node:fs");
const path = require("node:path");

const root = path.resolve(__dirname, "..");
const markdownFiles = [];

function walk(directory) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if ([".git", "node_modules", "dist"].includes(entry.name)) continue;
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) walk(absolute);
    else if (entry.name.endsWith(".md")) markdownFiles.push(absolute);
  }
}

walk(root);

const linkPattern = /\[[^\]]*\]\(([^)]+)\)/g;
const broken = [];
for (const file of markdownFiles) {
  const content = fs.readFileSync(file, "utf8");
  for (const match of content.matchAll(linkPattern)) {
    const target = match[1];
    if (/^(?:https?:\/\/|mailto:|#|codex:)/.test(target)) continue;
    const withoutFragment = target.split("#", 1)[0];
    if (!withoutFragment) continue;
    const resolved = path.resolve(path.dirname(file), decodeURIComponent(withoutFragment));
    if (!fs.existsSync(resolved)) {
      broken.push(`${path.relative(root, file)} -> ${target}`);
    }
  }
}

if (broken.length > 0) {
  throw new Error(`Broken local Markdown links:\n${broken.join("\n")}`);
}

const schemaPath = path.join(root, "docs", "contratos", "config", "modelcairn-config-v1alpha1.schema.json");
const schema = JSON.parse(fs.readFileSync(schemaPath, "utf8"));
if (schema.$defs.base.properties.metadata.$ref !== "#/$defs/metadata") {
  throw new Error("Present resources must use full metadata");
}
if (schema.$defs.tombstone.properties.metadata.$ref !== "#/$defs/tombstoneMetadata") {
  throw new Error("Absent resources must use tombstone metadata");
}

const settingsSchema = JSON.parse(fs.readFileSync(path.join(root, "docs", "contratos", "config", "admin-settings-v1alpha1.schema.json"), "utf8"));
const settingsFields = Object.keys(settingsSchema.$defs.spec.properties).sort();
const resolvedFields = [...settingsSchema.$defs.resolved.allOf[1].properties.spec.required].sort();
if (JSON.stringify(settingsFields) !== JSON.stringify(resolvedFields)) {
  throw new Error("Resolved settings must require every settings field");
}
for (const definition of ["initial", "update", "resolved"]) {
  if (!settingsSchema.$defs[definition]) throw new Error(`Missing settings definition: ${definition}`);
}

const acceptancePath = path.join(root, "docs", "fases", "fase-01-aceptacion.json");
const acceptance = JSON.parse(fs.readFileSync(acceptancePath, "utf8"));
const expectedPhaseOne = [
  ...Array.from({ length: 13 }, (_, index) => `RF-${String(index + 1).padStart(3, "0")}`),
  "RNF-001", "RNF-003", "RNF-004", "RNF-005", "RNF-006", "RNF-007", "RNF-008",
].sort();
const actualPhaseOne = acceptance.requirements.map((entry) => entry.id).sort();
if (JSON.stringify(actualPhaseOne) !== JSON.stringify(expectedPhaseOne)) {
  throw new Error("Phase 1 acceptance manifest has missing, duplicate, or unexpected requirements");
}
const deferredIds = acceptance.deferred.flatMap((entry) => entry.ids);
for (const requiredDeferred of ["RNF-002", "RF-101", "RF-108", "RF-201", "RF-206"]) {
  if (!deferredIds.includes(requiredDeferred)) throw new Error(`Missing deferred scope marker: ${requiredDeferred}`);
}
for (const requirement of acceptance.requirements) {
  if (!['candidate', 'accepted', 'blocked'].includes(requirement.status)) {
    throw new Error(`Invalid acceptance status for ${requirement.id}`);
  }
  if (!requirement.contracts?.length || !requirement.tests?.length) {
    throw new Error(`${requirement.id} must reference contracts and executable tests`);
  }
  for (const relative of [...requirement.contracts, ...(requirement.evidence || [])]) {
    if (!fs.existsSync(path.join(root, relative))) throw new Error(`${requirement.id} references missing artifact: ${relative}`);
  }
  for (const test of requirement.tests) {
    const absolute = path.join(root, test.path);
    if (!fs.existsSync(absolute)) throw new Error(`${requirement.id} references missing test: ${test.path}`);
    if (!fs.readFileSync(absolute, "utf8").includes(test.symbol)) {
      throw new Error(`${requirement.id} references missing test symbol ${test.symbol} in ${test.path}`);
    }
  }
}

console.log(`Documentation links valid: ${markdownFiles.length} Markdown files; schemas and ${actualPhaseOne.length} Phase 1 acceptance entries checked (not full schema or runtime validation).`);
