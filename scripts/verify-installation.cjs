"use strict";

const fs = require("node:fs");
const path = require("node:path");
const root = path.resolve(__dirname,"..");
const installer = fs.readFileSync(path.join(root,"scripts","install-linux.sh"),"utf8");
const bootstrap = fs.readFileSync(path.join(root,"scripts","bootstrap-linux.sh"),"utf8");
const uninstaller = fs.readFileSync(path.join(root,"scripts","uninstall-linux.sh"),"utf8");
const packageUpgrade = fs.readFileSync(path.join(root,"scripts","verify-package-upgrade-linux.sh"),"utf8");
const releaseFailures = fs.readFileSync(path.join(root,"scripts","verify-release-package-failures.sh"),"utf8");
const unit = fs.readFileSync(path.join(root,"packaging","systemd","modelcairn.service"),"utf8");

if (!bootstrap.includes("^https://[A-Za-z0-9.-]+(:[1-9][0-9]{0,4})?$")) {
  throw new Error("guided bootstrap must accept HTTPS DNS origins");
}
if (!bootstrap.includes("no se guardaron cambios") || bootstrap.includes("existing state was not replaced")) {
  throw new Error("guided bootstrap failure must explain rollback without implying prior state");
}

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
if (!installer.includes("systemctl stop modelcairn.service >/dev/null 2>&1 || true")) throw new Error("--no-start must leave the service stopped");
for (const required of ["verify_service_started", "systemctl is-active --quiet modelcairn.service", "journalctl -u modelcairn.service"]) {
  if (!installer.includes(required)) throw new Error(`verified service-start invariant missing: ${required}`);
}
for (const required of ["MODELCAIRN_ALLOW_DESTRUCTIVE_SYSTEM_TEST", "refusing host with an installed binary", "pre-test.tar", "trap cleanup EXIT", "tar --acls --xattrs --numeric-owner", "restore_host", "rm -rf -- /etc/modelcairn /var/lib/modelcairn"]) {
  if (!packageUpgrade.includes(required)) throw new Error(`isolated upgrade gate missing: ${required}`);
}
for (const required of ["incorrect checksum", "truncated archive", "wrong architecture"]) {
  if (!releaseFailures.includes(required)) throw new Error(`release negative gate missing: ${required}`);
}
console.log("Linux installation assets satisfy structural and safety invariants.");
