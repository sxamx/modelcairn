-- Authenticated evidence that the configured master key belongs to this installation.
-- NULL exists only as an invalid transitional value; startup fails closed instead
-- of guessing which key belongs to an installation without authenticated evidence.
ALTER TABLE installation_state ADD COLUMN key_check BLOB CHECK(key_check IS NULL OR length(key_check) = 32);
