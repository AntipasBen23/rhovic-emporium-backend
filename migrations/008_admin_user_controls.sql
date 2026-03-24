BEGIN;

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;

ALTER TABLE vendors
  ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ NULL;

CREATE INDEX IF NOT EXISTS idx_users_deleted_at
  ON users(deleted_at);

CREATE INDEX IF NOT EXISTS idx_vendors_deleted_at
  ON vendors(deleted_at);

COMMIT;
