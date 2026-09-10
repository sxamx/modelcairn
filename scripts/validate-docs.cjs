"use strict";

const fs = require("node:fs");
const path = require("node:path");

const root = path.resolve(__dirname, "..");
const markdownFiles = [];

function walk(directory) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (entry.name === ".git") continue;
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
console.log(`Documentation links valid: ${markdownFiles.length} Markdown files; configuration/settings schemas parsed and structural invariants checked (not full schema or runtime validation).`);
