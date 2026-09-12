-- ModelCairn migration 0004: privacy-preserving failed-login statistics.

-- Existing v3 settings are canonical JSON without this newly resolved field.
-- json_insert appends it in the same order emitted by the typed Go encoder.
UPDATE admin_settings
SET spec_json = json_insert(spec_json, '$.failedLoginRetentionSeconds', 86400)
WHERE json_type(spec_json, '$.failedLoginRetentionSeconds') IS NULL;

CREATE TABLE failed_login_statistics (
    minute_unix INTEGER NOT NULL CHECK (minute_unix >= 0 AND minute_unix % 60 = 0),
    reason TEXT NOT NULL CHECK (reason IN ('invalid_credentials','throttled','malformed','unavailable')),
    count INTEGER NOT NULL CHECK (count > 0),
    PRIMARY KEY (minute_unix, reason)
) STRICT, WITHOUT ROWID;
