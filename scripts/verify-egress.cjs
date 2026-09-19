"use strict";

const fs = require("node:fs");
const path = require("node:path");
const root = path.resolve(__dirname, "..");
const inventory = JSON.parse(fs.readFileSync(path.join(root, "docs", "seguridad", "egress-inventory.json"), "utf8"));
const declared = new Set(inventory.productionCallsites.map((entry) => entry.path.replaceAll("\\", "/")));
const found = new Set();

function walk(directory) {
  for (const entry of fs.readdirSync(directory, { withFileTypes: true })) {
    if (["node_modules", "dist"].includes(entry.name)) continue;
    const absolute = path.join(directory, entry.name);
    if (entry.isDirectory()) { walk(absolute); continue; }
    if (entry.name.endsWith("_test.go") || entry.name.endsWith(".test.tsx")) continue;
    if (!entry.name.endsWith(".go") && !entry.name.endsWith(".ts") && !entry.name.endsWith(".tsx")) continue;
    const content = fs.readFileSync(absolute, "utf8");
    if (/(?:http\.Client|net\.Dialer|DialContext|fetch\s*\()/.test(content)) {
      found.add(path.relative(root, absolute).replaceAll("\\", "/"));
    }
  }
}

walk(path.join(root, "internal"));
walk(path.join(root, "web", "src"));
const unexpected = [...found].filter((item) => !declared.has(item));
const missing = [...declared].filter((item) => !found.has(item));
if (unexpected.length || missing.length) {
  throw new Error(`Unexpected production egress inventory change; unexpected=${unexpected.join(",")}; missing=${missing.join(",")}`);
}
const webClient = fs.readFileSync(path.join(root, "web", "src", "api", "client.ts"), "utf8");
for (const match of webClient.matchAll(/fetch\s*\(([^,\n]+)/g)) {
  if (!/^\s*(?:`\/|"\/|'\/)/.test(match[1])) throw new Error(`Browser fetch is not visibly same-origin: ${match[1]}`);
}
if (inventory.externalTelemetry.length !== 0) throw new Error("External telemetry inventory must remain empty in Phase 1");
console.log(`Egress inventory valid: ${found.size} production callsite files; no external telemetry declared.`);
