-- Milestone 3: versioned administrative settings and captured session idle policy.
-- Existing sessions predate that policy, so revoke them instead of guessing it.
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
