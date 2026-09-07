-- Authenticated evidence that the configured master key belongs to this installation.
-- NULL exists only while upgrading a pre-secret-store database; startup fills it
-- before readiness and never accepts NULL for an initialized secret store.
ALTER TABLE installation_state ADD COLUMN key_check BLOB CHECK(key_check IS NULL OR length(key_check) = 32);
