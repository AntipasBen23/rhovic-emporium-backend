BEGIN;

CREATE TABLE IF NOT EXISTS support_threads (
  id TEXT PRIMARY KEY,
  customer_id TEXT NOT NULL REFERENCES users(id),
  order_id TEXT NULL REFERENCES orders(id),
  subject TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'open',
  assigned_admin_id TEXT NULL REFERENCES users(id),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  closed_at TIMESTAMPTZ NULL
);

CREATE TABLE IF NOT EXISTS support_messages (
  id TEXT PRIMARY KEY,
  thread_id TEXT NOT NULL REFERENCES support_threads(id) ON DELETE CASCADE,
  sender_id TEXT NOT NULL REFERENCES users(id),
  sender_role TEXT NOT NULL,
  message TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_support_threads_customer_created
  ON support_threads(customer_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_support_threads_status_updated
  ON support_threads(status, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_support_messages_thread_created
  ON support_messages(thread_id, created_at ASC);

COMMIT;
