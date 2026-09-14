"use strict";

const fs = require("node:fs");
const path = require("node:path");
const root = path.resolve(__dirname,"..");
const installer = fs.readFileSync(path.join(root,"scripts","install-linux.sh"),"utf8");
const bootstrap = fs.readFileSync(path.join(root,"scripts","bootstrap-linux.sh"),"utf8");
const uninstaller = fs.readFileSync(path.join(root,"scripts","uninstall-linux.sh"),"utf8");
const unit = fs.readFileSync(path.join(root,"packaging","systemd","modelcairn.service"),"utf8");

for (const text of ["--enable", "--no-enable", "--start", "--no-start", "--non-interactive", "/var/lib/modelcairn"]) {
  if (!installer.includes(text)) throw new Error(`installer invariant missing: ${text}`);
}
for (const forbidden of [/curl\s/i,/wget\s/i,/rm\s+-rf[^\n]*\/var\/lib\/modelcairn/i,/rm\s+-[^\n]*\/var\/lib\/modelcairn/i,/MODELCAIRN_.*PASSWORD/i]) {
  if (forbidden.test(installer+bootstrap+uninstaller)) throw new Error(`unsafe installation pattern: ${forbidden}`);
}
for (const required of ["</dev/tty","runuser -u modelcairn","admin bootstrap","mktemp /etc/modelcairn/","trap 'restore_service; cleanup' EXIT"]) {
  if (!bootstrap.includes(required)) throw new Error(`safe bootstrap invariant missing: ${required}`);
}
for (const directive of ["User=modelcairn","Group=modelcairn","NoNewPrivileges=true","ProtectSystem=strict","ProtectHome=true","ReadWritePaths=/var/lib/modelcairn","CapabilityBoundingSet="]) {
  if (!unit.includes(directive)) throw new Error(`systemd hardening missing: ${directive}`);
}
if (!unit.includes("EnvironmentFile=/etc/modelcairn/service.env") || !unit.includes("--data-dir /var/lib/modelcairn")) throw new Error("unit paths differ from the installation contract");
for (const required of ["disable --now modelcairn.service", "rm -f -- /etc/systemd/system/modelcairn.service", "rm -f -- /usr/local/bin/modelcairn", "Configuration and data remain in /etc/modelcairn and /var/lib/modelcairn"]) {
  if (!uninstaller.includes(required)) throw new Error(`conservative uninstaller invariant missing: ${required}`);
}
if (!installer.includes("else systemctl stop modelcairn.service")) throw new Error("--no-start must leave the service stopped");
console.log("Linux installation assets satisfy structural and safety invariants.");
