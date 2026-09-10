// Executable draft contract; does not exercise the production Go migration runner.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const { DatabaseSync } = require("node:sqlite");
const root = path.resolve(__dirname, "../../..");
const draft = fs.readFileSync(path.join(__dirname, "admin-identity-v1.draft.sql"), "utf8");
const now = "2026-01-01T00:00:00Z";
function seeded() {
  const db = new DatabaseSync(":memory:");
  db.exec("PRAGMA foreign_keys=ON");
  for (const file of ["0001_initial.sql", "0002_key_check.sql"]) {
    db.exec(fs.readFileSync(path.join(root, "internal/storage/migrations", file), "utf8"));
  }
  db.prepare("INSERT INTO admin_users VALUES(?,?,?,1,?,?)").run("admin", "admin", "test-phc", now, now);
  db.prepare(`INSERT INTO admin_sessions VALUES(?, 'admin', 1, ?, NULL, ?, ?, ?, ?, NULL)`)
    .run(Buffer.alloc(32, 1), Buffer.alloc(32, 2), now, now, now, "2026-01-02T00:00:00Z");
  return db;
}
let checks = 0;
function verify(condition) { assert.ok(condition); checks++; }
function rejects(action) { assert.throws(action); checks++; }
const db = seeded();
try {
  db.exec("BEGIN");
  db.exec(draft);
  db.exec("COMMIT");
  const legacy = db.prepare("SELECT revoked_at,idle_seconds FROM admin_sessions").get();
  verify(legacy.revoked_at !== null && legacy.idle_seconds === null);
  rejects(() => db.exec("UPDATE admin_sessions SET revoked_at=NULL"));
  const insert = db.prepare(`INSERT INTO admin_sessions
    (id_hash,admin_id,auth_version,csrf_hash,csrf_rotated_at,created_at,last_seen_at,expires_at,idle_seconds)
    VALUES(?,'admin',1,?,?,?,?,?,?)`);
  for (const duration of [null, 299, 86401]) {
    rejects(() => insert.run(Buffer.alloc(32, 3), Buffer.alloc(32, 4), now, now, now, now, duration));
  }
  for (const [index, duration] of [300, 1800, 86400].entries()) {
    insert.run(Buffer.alloc(32, 10 + index), Buffer.alloc(32, 4), now, now, now, now, duration);
    checks++;
  }
  const settings = db.prepare("INSERT INTO admin_settings VALUES(?,?,?,?)");
  for (const [id, version, value] of [[2,1,"{}"],[1,0,"{}"],[1,1,"[]"],[1,1,"bad"],[1,1,JSON.stringify({large:"x".repeat(65536)})]]) {
    rejects(() => settings.run(id, version, value, now));
  }
  settings.run(1, 1, "{}", now); // Semantic field validation belongs to the shared validator.
  verify(db.prepare("SELECT count(*) AS n FROM admin_settings").get().n === 1);
  rejects(() => settings.run(1, 2, "{}", now));
} finally { db.close(); }
const rollback = seeded();
try {
  rollback.exec("BEGIN");
  rollback.exec(draft);
  rejects(() => rollback.exec("INSERT INTO admin_settings VALUES(2,1,'{}','test')"));
  rollback.exec("ROLLBACK");
  verify(rollback.prepare("SELECT revoked_at FROM admin_sessions").get().revoked_at === null);
  verify(!rollback.prepare("PRAGMA table_info(admin_sessions)").all().some(c => c.name === "idle_seconds"));
  verify(!rollback.prepare("SELECT name FROM sqlite_schema WHERE name='admin_settings'").get());
} finally { rollback.close(); }
console.log(`Administrative identity draft SQL: ${checks} checks passed; production runner and HTTP not tested.`);
