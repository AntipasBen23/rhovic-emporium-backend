BEGIN;

CREATE TABLE IF NOT EXISTS security_events (
  id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL,
  principal_key TEXT NOT NULL DEFAULT '',
  email TEXT NOT NULL DEFAULT '',
  user_id TEXT NOT NULL DEFAULT '',
  ip_address TEXT NOT NULL DEFAULT '',
  path TEXT NOT NULL DEFAULT '',
  details_json TEXT NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_security_events_type_created
  ON security_events(event_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_security_events_email_created
  ON security_events(email, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_security_events_user_created
  ON security_events(user_id, created_at DESC);

COMMIT;
