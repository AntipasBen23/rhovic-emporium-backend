BEGIN;

ALTER TABLE page_visits
  ADD COLUMN IF NOT EXISTS user_id TEXT NULL REFERENCES users(id),
  ADD COLUMN IF NOT EXISTS user_email TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_page_visits_user_created
  ON page_visits(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_page_visits_user_email_created
  ON page_visits(user_email, created_at DESC);

COMMIT;
