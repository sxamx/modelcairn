-- Draft executable contract, NOT an installed runtime migration.
-- Apply after the published migrations, inside the migration runner transaction.
-- Existing sessions have no captured idle policy: revoke rather than guess it.
UPDATE admin_sessions
SET revoked_at = strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
WHERE revoked_at IS NULL;

ALTER TABLE admin_sessions ADD COLUMN idle_seconds INTEGER
  CHECK (
    (idle_seconds IS NULL AND revoked_at IS NOT NULL)
    OR (idle_seconds IS NOT NULL AND idle_seconds BETWEEN 300 AND 86400)
  );

CREATE TABLE admin_settings (
  singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
  resource_version INTEGER NOT NULL CHECK(resource_version > 0),
  spec_json TEXT NOT NULL
    CHECK(length(CAST(spec_json AS BLOB)) <= 65536)
    CHECK(json_valid(spec_json))
    CHECK(json_type(spec_json) = 'object'),
  updated_at TEXT NOT NULL
) STRICT;
