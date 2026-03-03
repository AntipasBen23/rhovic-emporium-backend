-- Stronger numeric types for fractional quantities + safe stock math
ALTER TABLE products
  ALTER COLUMN stock_quantity TYPE NUMERIC(18,6) USING stock_quantity::numeric;

ALTER TABLE order_items
  ALTER COLUMN quantity TYPE NUMERIC(18,6) USING quantity::numeric;

-- Subscription plans
CREATE TABLE IF NOT EXISTS subscription_plans (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  monthly_price BIGINT NOT NULL DEFAULT 0,
  yearly_price BIGINT NOT NULL DEFAULT 0,
  commission_rate NUMERIC(8,6) NOT NULL DEFAULT 0.10,
  product_limit INT NOT NULL DEFAULT 100,
  features_json TEXT NOT NULL DEFAULT '{}',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Default plan seed (id must be stable)
INSERT INTO subscription_plans (id, name, commission_rate, product_limit)
VALUES
  ('plan_vendor', 'Vendor', 0.10, 100)
ON CONFLICT (id) DO NOTHING;

-- Vendors default plan if empty
UPDATE vendors SET subscription_plan_id='plan_vendor' WHERE subscription_plan_id='';

-- Platform settings (commission default etc.)
CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO settings (key, value) VALUES ('commission_default_rate', '0.10')
ON CONFLICT (key) DO NOTHING;

-- Payouts
CREATE TABLE IF NOT EXISTS payouts (
  id TEXT PRIMARY KEY,
  vendor_id TEXT NOT NULL REFERENCES vendors(id),
  amount BIGINT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending', -- pending, approved, rejected, paid, failed
  reason TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payouts_vendor_created ON payouts(vendor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payouts_status_created ON payouts(status, created_at DESC);

-- Disputes
CREATE TABLE IF NOT EXISTS disputes (
  id TEXT PRIMARY KEY,
  order_id TEXT NOT NULL REFERENCES orders(id),
  opened_by TEXT NOT NULL REFERENCES users(id),
  status TEXT NOT NULL DEFAULT 'open', -- open, under_review, resolved, rejected
  internal_notes TEXT NOT NULL DEFAULT '',
  resolution_notes TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_disputes_status_created ON disputes(status, created_at DESC);

-- Refresh tokens (hashed)
CREATE TABLE IF NOT EXISTS refresh_tokens (
  id TEXT PRIMARY KEY,
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL UNIQUE,
  jti TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_refresh_user ON refresh_tokens(user_id, created_at DESC);

-- Payment webhook idempotency (processed refs)
CREATE TABLE IF NOT EXISTS webhook_events (
  id TEXT PRIMARY KEY,
  provider TEXT NOT NULL,
  provider_ref TEXT NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);