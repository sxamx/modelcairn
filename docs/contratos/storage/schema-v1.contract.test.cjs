// Executable documentation: verifies provider-affinity constraints in schema-v1.sql.
const fs = require("node:fs");
const path = require("node:path");
const { DatabaseSync } = require("node:sqlite");

function database() {
  const db = new DatabaseSync(":memory:");
  db.exec(fs.readFileSync(path.join(__dirname, "schema-v1.sql"), "utf8"));
  const now = "2026-01-01T00:00:00Z";
  const resource = db.prepare("INSERT INTO resources(id,kind,name,spec_json,created_at,updated_at) VALUES(?,?,?,?,?,?)");
  for (const [id, kind] of [["p1","Provider"],["p2","Provider"],["a1","ProviderAccount"],["a2","ProviderAccount"],["c1","ProviderConnection"],["c2","ProviderConnection"],["e1","Egress"],["cr1","Credential"],["cr2","Credential"],["m1","Model"],["m2","Model"],["d1","Destination"]]) {
    resource.run(id, kind, id, "{}", now, now);
  }
  db.exec("INSERT INTO provider_accounts VALUES('a1','p1'),('a2','p2')");
  db.exec("INSERT INTO provider_connections VALUES('c1','p1','https://one.invalid',0,1),('c2','p2','https://two.invalid',0,1)");
  db.exec("INSERT INTO egresses VALUES('e1','direct',1)");
  db.exec("INSERT INTO secrets VALUES('s1','s1',1,'XCHACHA20-POLY1305',zeroblob(24),x'01','fp1',1,'2026-01-01','2026-01-01'),('s2','s2',1,'XCHACHA20-POLY1305',zeroblob(24),x'02','fp2',1,'2026-01-01','2026-01-01')");
  db.exec("INSERT INTO credentials VALUES('cr1','a1','e1','s1','active',NULL),('cr2','a2','e1','s2','active',NULL)");
  db.exec("INSERT INTO models VALUES('m1','c1','m1','[]',1),('m2','c2','m2','[]',1)");
  db.exec("INSERT INTO destinations VALUES('d1','m1','cr1',100,1)");
  return db;
}

const mutations = [
  "UPDATE destinations SET credential_id='cr2' WHERE resource_id='d1'",
  "UPDATE destinations SET model_id='m2' WHERE resource_id='d1'",
  "UPDATE provider_accounts SET provider_id='p2' WHERE resource_id='a1'",
  "UPDATE provider_connections SET provider_id='p2' WHERE resource_id='c1'",
  "UPDATE credentials SET provider_account_id='a2' WHERE resource_id='cr1'",
  "UPDATE models SET connection_id='c2' WHERE resource_id='m1'"
];

for (const mutation of mutations) {
  const db = database();
  let rejected = false;
  try { db.exec(mutation); } catch { rejected = true; }
  if (!rejected) throw new Error(`Unsafe mutation accepted: ${mutation}`);
}
console.log(`Provider-affinity mutations rejected: ${mutations.length}/6`);
