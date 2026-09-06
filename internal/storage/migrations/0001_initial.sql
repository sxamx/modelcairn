-- ModelCairn migration 0001: initial persisted model.

CREATE TABLE installation_state (
  singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
  installation_id TEXT NOT NULL UNIQUE,
  active_key_version INTEGER NOT NULL CHECK(active_key_version > 0),
  config_revision INTEGER NOT NULL DEFAULT 1 CHECK(config_revision > 0),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
) STRICT;

CREATE TABLE consumed_plan_tokens (
  nonce_hash BLOB PRIMARY KEY CHECK(length(nonce_hash) = 32),
  expires_at TEXT NOT NULL,
  consumed_at TEXT NOT NULL
) STRICT;

CREATE TABLE resources (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  display_name TEXT,
  description TEXT,
  resource_version INTEGER NOT NULL DEFAULT 1 CHECK(resource_version > 0),
  spec_json TEXT NOT NULL CHECK(json_valid(spec_json)),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE(kind, name)
) STRICT;

CREATE TABLE secrets (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  key_version INTEGER NOT NULL CHECK(key_version > 0),
  algorithm TEXT NOT NULL CHECK(algorithm = 'XCHACHA20-POLY1305'),
  nonce BLOB NOT NULL CHECK(length(nonce) = 24),
  ciphertext BLOB NOT NULL,
  fingerprint TEXT NOT NULL,
  resource_version INTEGER NOT NULL DEFAULT 1 CHECK(resource_version > 0),
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
) STRICT;

CREATE TABLE provider_accounts (
  resource_id TEXT PRIMARY KEY REFERENCES resources(id) ON DELETE CASCADE,
  provider_id TEXT NOT NULL REFERENCES resources(id) ON DELETE RESTRICT
) STRICT;

CREATE TABLE provider_connections (
  resource_id TEXT PRIMARY KEY REFERENCES resources(id) ON DELETE CASCADE,
  provider_id TEXT NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
  base_url TEXT NOT NULL,
  allow_private_network INTEGER NOT NULL DEFAULT 0 CHECK(allow_private_network IN (0,1)),
  enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1))
) STRICT;

CREATE TABLE egresses (
  resource_id TEXT PRIMARY KEY REFERENCES resources(id) ON DELETE CASCADE,
  egress_type TEXT NOT NULL CHECK(egress_type = 'direct'),
  enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1))
) STRICT;

CREATE TABLE credentials (
  resource_id TEXT PRIMARY KEY REFERENCES resources(id) ON DELETE CASCADE,
  provider_account_id TEXT NOT NULL REFERENCES provider_accounts(resource_id) ON DELETE RESTRICT,
  egress_id TEXT NOT NULL REFERENCES egresses(resource_id) ON DELETE RESTRICT,
  secret_id TEXT NOT NULL REFERENCES secrets(id) ON DELETE RESTRICT,
  status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','blocked','disabled')),
  blocked_reason TEXT,
  UNIQUE(resource_id, egress_id)
) STRICT;

CREATE TABLE models (
  resource_id TEXT PRIMARY KEY REFERENCES resources(id) ON DELETE CASCADE,
  connection_id TEXT NOT NULL REFERENCES provider_connections(resource_id) ON DELETE RESTRICT,
  provider_model_id TEXT NOT NULL,
  capabilities_json TEXT NOT NULL CHECK(json_valid(capabilities_json)),
  enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
  UNIQUE(connection_id, provider_model_id)
) STRICT;

CREATE TABLE destinations (
  resource_id TEXT PRIMARY KEY REFERENCES resources(id) ON DELETE CASCADE,
  model_id TEXT NOT NULL REFERENCES models(resource_id) ON DELETE RESTRICT,
  credential_id TEXT NOT NULL REFERENCES credentials(resource_id) ON DELETE RESTRICT,
  weight INTEGER NOT NULL DEFAULT 100 CHECK(weight BETWEEN 1 AND 1000),
  enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
  UNIQUE(model_id, credential_id)
) STRICT;

CREATE TABLE strategy_versions (
  id TEXT PRIMARY KEY,
  strategy_id TEXT NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
  version INTEGER NOT NULL CHECK(version > 0),
  definition_json TEXT NOT NULL CHECK(json_valid(definition_json)),
  checksum TEXT NOT NULL,
  published_at TEXT NOT NULL,
  UNIQUE(strategy_id, version),
  UNIQUE(strategy_id, checksum)
) STRICT;

CREATE TABLE routes (
  resource_id TEXT PRIMARY KEY REFERENCES resources(id) ON DELETE CASCADE,
  model_alias TEXT NOT NULL UNIQUE,
  strategy_id TEXT NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
  active_strategy_version_id TEXT REFERENCES strategy_versions(id) ON DELETE RESTRICT,
  enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1))
) STRICT;

CREATE TABLE admin_users (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_phc TEXT NOT NULL,
  auth_version INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
) STRICT;

CREATE TABLE admin_sessions (
  id_hash BLOB PRIMARY KEY,
  admin_id TEXT NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  auth_version INTEGER NOT NULL,
  csrf_hash BLOB NOT NULL,
  csrf_previous_hash BLOB,
  csrf_rotated_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  last_seen_at TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  revoked_at TEXT
) STRICT;

CREATE TABLE agent_tokens (
  resource_id TEXT PRIMARY KEY REFERENCES resources(id) ON DELETE CASCADE,
  verifier_sha256 BLOB UNIQUE CHECK(verifier_sha256 IS NULL OR length(verifier_sha256) = 32),
  token_prefix TEXT,
  issued_at TEXT,
  expires_at TEXT,
  revoked_at TEXT,
  allowed_routes_json TEXT NOT NULL CHECK(json_valid(allowed_routes_json))
) STRICT;

CREATE TABLE requests (
  id TEXT PRIMARY KEY,
  agent_token_id TEXT REFERENCES agent_tokens(resource_id) ON DELETE SET NULL,
  route_id TEXT REFERENCES routes(resource_id) ON DELETE SET NULL,
  requested_alias TEXT NOT NULL,
  started_at TEXT NOT NULL,
  completed_at TEXT,
  outcome TEXT CHECK(outcome IN ('success','error','partial','cancelled','indeterminate')),
  http_status INTEGER,
  input_tokens INTEGER CHECK(input_tokens IS NULL OR input_tokens >= 0),
  output_tokens INTEGER CHECK(output_tokens IS NULL OR output_tokens >= 0),
  ttft_ms INTEGER CHECK(ttft_ms IS NULL OR ttft_ms >= 0),
  duration_ms INTEGER CHECK(duration_ms IS NULL OR duration_ms >= 0),
  content_stored INTEGER NOT NULL DEFAULT 0 CHECK(content_stored = 0)
) STRICT;

CREATE TABLE attempts (
  id TEXT PRIMARY KEY,
  request_id TEXT NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
  sequence INTEGER NOT NULL CHECK(sequence > 0),
  destination_id TEXT REFERENCES destinations(resource_id) ON DELETE SET NULL,
  started_at TEXT NOT NULL,
  completed_at TEXT,
  outcome TEXT NOT NULL CHECK(outcome IN ('success','error','partial','cancelled','indeterminate')),
  error_class TEXT,
  provider_status INTEGER,
  provider_request_id TEXT,
  retryable INTEGER NOT NULL DEFAULT 0 CHECK(retryable IN (0,1)),
  fallback_reason TEXT,
  UNIQUE(request_id, sequence)
) STRICT;

CREATE TABLE rate_limit_observations (
  id TEXT PRIMARY KEY,
  attempt_id TEXT NOT NULL REFERENCES attempts(id) ON DELETE CASCADE,
  scope_kind TEXT NOT NULL CHECK(scope_kind IN ('destination','model','credential','account','connection','provider','unknown')),
  scope_resource_id TEXT,
  source TEXT NOT NULL CHECK(source IN ('header','body','operator','inference')),
  limit_value INTEGER,
  remaining_value INTEGER,
  resets_at TEXT,
  observed_at TEXT NOT NULL,
  raw_content_stored INTEGER NOT NULL DEFAULT 0 CHECK(raw_content_stored = 0)
) STRICT;

CREATE TABLE cooldowns (
  id TEXT PRIMARY KEY,
  scope_kind TEXT NOT NULL CHECK(scope_kind IN ('destination','model','credential','account','connection','provider')),
  scope_resource_id TEXT NOT NULL CHECK(length(scope_resource_id) > 0),
  reason TEXT NOT NULL,
  starts_at TEXT NOT NULL,
  ends_at TEXT NOT NULL,
  source_observation_id TEXT REFERENCES rate_limit_observations(id) ON DELETE SET NULL,
  UNIQUE(scope_kind, scope_resource_id)
) STRICT;

CREATE TABLE audit_events (
  id TEXT PRIMARY KEY,
  actor_type TEXT NOT NULL CHECK(actor_type IN ('admin','cli','system')),
  actor_id TEXT,
  action TEXT NOT NULL,
  resource_kind TEXT,
  resource_id TEXT,
  result TEXT NOT NULL CHECK(result IN ('success','failure')),
  details_json TEXT NOT NULL DEFAULT '{}' CHECK(json_valid(details_json)),
  occurred_at TEXT NOT NULL
) STRICT;

CREATE INDEX idx_resources_kind_name ON resources(kind, name);
CREATE INDEX idx_requests_started ON requests(started_at DESC);
CREATE INDEX idx_requests_route_started ON requests(route_id, started_at DESC);
CREATE INDEX idx_attempts_destination_started ON attempts(destination_id, started_at DESC);
CREATE INDEX idx_observations_scope_time ON rate_limit_observations(scope_kind, scope_resource_id, observed_at DESC);
CREATE INDEX idx_audit_time ON audit_events(occurred_at DESC);
CREATE INDEX idx_sessions_admin_expiry ON admin_sessions(admin_id, expires_at);
CREATE INDEX idx_plan_tokens_expiry ON consumed_plan_tokens(expires_at);

CREATE TRIGGER destinations_provider_match_insert
BEFORE INSERT ON destinations
BEGIN
  SELECT CASE WHEN (
    SELECT pa.provider_id
    FROM credentials c JOIN provider_accounts pa ON pa.resource_id = c.provider_account_id
    WHERE c.resource_id = NEW.credential_id
  ) != (
    SELECT pc.provider_id
    FROM models m JOIN provider_connections pc ON pc.resource_id = m.connection_id
    WHERE m.resource_id = NEW.model_id
  ) THEN RAISE(ABORT, 'provider_mismatch') END;
END;

CREATE TRIGGER destinations_provider_match_update
BEFORE UPDATE OF model_id, credential_id ON destinations
BEGIN
  SELECT CASE WHEN (
    SELECT pa.provider_id
    FROM credentials c JOIN provider_accounts pa ON pa.resource_id = c.provider_account_id
    WHERE c.resource_id = NEW.credential_id
  ) != (
    SELECT pc.provider_id
    FROM models m JOIN provider_connections pc ON pc.resource_id = m.connection_id
    WHERE m.resource_id = NEW.model_id
  ) THEN RAISE(ABORT, 'provider_mismatch') END;
END;

CREATE TRIGGER provider_accounts_provider_immutable_when_used
BEFORE UPDATE OF provider_id ON provider_accounts
WHEN EXISTS (SELECT 1 FROM credentials c JOIN destinations d ON d.credential_id = c.resource_id WHERE c.provider_account_id = OLD.resource_id)
BEGIN
  SELECT RAISE(ABORT, 'provider_account_in_use');
END;

CREATE TRIGGER provider_connections_provider_immutable_when_used
BEFORE UPDATE OF provider_id ON provider_connections
WHEN EXISTS (SELECT 1 FROM models m JOIN destinations d ON d.model_id = m.resource_id WHERE m.connection_id = OLD.resource_id)
BEGIN
  SELECT RAISE(ABORT, 'provider_connection_in_use');
END;

CREATE TRIGGER credentials_account_immutable_when_used
BEFORE UPDATE OF provider_account_id ON credentials
WHEN EXISTS (SELECT 1 FROM destinations d WHERE d.credential_id = OLD.resource_id)
BEGIN
  SELECT RAISE(ABORT, 'credential_in_use');
END;

CREATE TRIGGER models_connection_immutable_when_used
BEFORE UPDATE OF connection_id ON models
WHEN EXISTS (SELECT 1 FROM destinations d WHERE d.model_id = OLD.resource_id)
BEGIN
  SELECT RAISE(ABORT, 'model_in_use');
END;
