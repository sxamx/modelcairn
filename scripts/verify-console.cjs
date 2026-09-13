"use strict";

const fs = require("node:fs");
const path = require("node:path");
const zlib = require("node:zlib");

const root = path.resolve(__dirname, "..");
const dist = path.join(root, "internal", "consoleui", "dist");
const required = ["index.html", "manifest.webmanifest", "sw.js", "icon.svg"];
for (const name of required) {
  if (!fs.statSync(path.join(dist, name)).isFile()) throw new Error(`missing console asset: ${name}`);
}

const files = fs.readdirSync(path.join(dist, "assets"));
if (files.some((name) => name.endsWith(".map"))) throw new Error("source maps must not be embedded");
const scripts = files.filter((name) => name.endsWith(".js"));
if (scripts.length !== 1) throw new Error(`expected one initial JavaScript asset, found ${scripts.length}`);
const gzipBytes = zlib.gzipSync(fs.readFileSync(path.join(dist, "assets", scripts[0]))).length;
const targetBytes = 250 * 1024;
if (gzipBytes >= targetBytes) throw new Error(`initial JavaScript is ${gzipBytes} gzip bytes; target is below ${targetBytes}`);

const worker = fs.readFileSync(path.join(dist, "sw.js"), "utf8");
for (const reserved of ["/api/", "/v1/", "/healthz", "/readyz"]) {
  if (!worker.includes(reserved)) throw new Error(`service worker does not exclude ${reserved}`);
}
const manifest = JSON.parse(fs.readFileSync(path.join(dist, "manifest.webmanifest"), "utf8"));
if (manifest.display !== "standalone" || manifest.start_url !== "/" || manifest.id !== "/" || !Array.isArray(manifest.icons) || manifest.icons.length === 0) throw new Error("manifest is not installable");
const index = fs.readFileSync(path.join(dist, "index.html"), "utf8");
if (!index.includes("apple-mobile-web-app-capable") || !index.includes("manifest.webmanifest")) throw new Error("mobile PWA metadata missing");

console.log(`Console assets valid; initial JavaScript ${gzipBytes} gzip bytes (< ${targetBytes}).`);
