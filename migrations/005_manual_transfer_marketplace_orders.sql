ALTER TABLE orders
  ADD COLUMN IF NOT EXISTS order_number TEXT,
  ADD COLUMN IF NOT EXISTS customer_id TEXT,
  ADD COLUMN IF NOT EXISTS payment_reference TEXT,
  ADD COLUMN IF NOT EXISTS currency TEXT NOT NULL DEFAULT 'NGN',
  ADD COLUMN IF NOT EXISTS subtotal_amount BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS delivery_amount BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS discount_amount BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS payment_method TEXT NOT NULL DEFAULT 'bank_transfer',
  ADD COLUMN IF NOT EXISTS payment_status TEXT NOT NULL DEFAULT 'pending',
  ADD COLUMN IF NOT EXISTS order_status TEXT NOT NULL DEFAULT 'pending_payment',
  ADD COLUMN IF NOT EXISTS bank_transfer_status TEXT NOT NULL DEFAULT 'pending',
  ADD COLUMN IF NOT EXISTS notes TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE orders SET customer_id = buyer_id WHERE customer_id IS NULL;
UPDATE orders SET subtotal_amount = total_amount WHERE subtotal_amount = 0;

CREATE UNIQUE INDEX IF NOT EXISTS uq_orders_order_number ON orders(order_number);
CREATE UNIQUE INDEX IF NOT EXISTS uq_orders_payment_reference ON orders(payment_reference);
CREATE INDEX IF NOT EXISTS idx_orders_customer_created ON orders(customer_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orders_payment_status_created ON orders(payment_status, created_at DESC);

ALTER TABLE order_items
  ADD COLUMN IF NOT EXISTS line_total BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS product_name_snapshot TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS product_image_snapshot TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS vendor_order_id TEXT NULL;

UPDATE order_items SET line_total = subtotal WHERE line_total = 0;

CREATE TABLE IF NOT EXISTS vendor_orders (
  id TEXT PRIMARY KEY,
  order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  vendor_id TEXT NOT NULL REFERENCES vendors(id),
  vendor_order_number TEXT NOT NULL,
  subtotal_amount BIGINT NOT NULL DEFAULT 0,
  delivery_amount BIGINT NOT NULL DEFAULT 0,
  commission_amount BIGINT NOT NULL DEFAULT 0,
  vendor_net_amount BIGINT NOT NULL DEFAULT 0,
  fulfillment_status TEXT NOT NULL DEFAULT 'pending',
  payout_status TEXT NOT NULL DEFAULT 'unpaid',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_vendor_orders_number ON vendor_orders(vendor_order_number);
CREATE INDEX IF NOT EXISTS idx_vendor_orders_order ON vendor_orders(order_id);
CREATE INDEX IF NOT EXISTS idx_vendor_orders_vendor_created ON vendor_orders(vendor_id, created_at DESC);

CREATE TABLE IF NOT EXISTS vendor_order_items (
  id TEXT PRIMARY KEY,
  vendor_order_id TEXT NOT NULL REFERENCES vendor_orders(id) ON DELETE CASCADE,
  order_item_id TEXT NOT NULL REFERENCES order_items(id) ON DELETE CASCADE,
  product_id TEXT NOT NULL REFERENCES products(id),
  quantity NUMERIC(18,6) NOT NULL,
  unit_price BIGINT NOT NULL,
  line_total BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_vendor_order_items_vendor_order ON vendor_order_items(vendor_order_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_vendor_order_items_order_item ON vendor_order_items(order_item_id);

CREATE TABLE IF NOT EXISTS payment_proofs (
  id TEXT PRIMARY KEY,
  order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  uploaded_by TEXT NOT NULL REFERENCES users(id),
  file_url TEXT NOT NULL,
  file_type TEXT NOT NULL,
  review_status TEXT NOT NULL DEFAULT 'pending',
  admin_note TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_payment_proofs_order_created ON payment_proofs(order_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payment_proofs_pending ON payment_proofs(review_status, created_at DESC);

ALTER TABLE payments
  ADD COLUMN IF NOT EXISTS payment_reference TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS method TEXT NOT NULL DEFAULT 'bank_transfer',
  ADD COLUMN IF NOT EXISTS provider_reference TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS meta_json TEXT NOT NULL DEFAULT '{}',
  ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

UPDATE payments SET provider_reference = provider_ref WHERE provider_reference = '';

CREATE INDEX IF NOT EXISTS idx_payments_order_created ON payments(order_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_payments_reference ON payments(payment_reference);

CREATE TABLE IF NOT EXISTS commissions (
  id TEXT PRIMARY KEY,
  order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  vendor_order_id TEXT NOT NULL REFERENCES vendor_orders(id) ON DELETE CASCADE,
  vendor_id TEXT NOT NULL REFERENCES vendors(id),
  commission_rate NUMERIC(8,6) NOT NULL,
  gross_amount BIGINT NOT NULL,
  commission_amount BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_commissions_order ON commissions(order_id);
CREATE INDEX IF NOT EXISTS idx_commissions_vendor ON commissions(vendor_id, created_at DESC);

CREATE TABLE IF NOT EXISTS vendor_payouts (
  id TEXT PRIMARY KEY,
  vendor_id TEXT NOT NULL REFERENCES vendors(id),
  vendor_order_id TEXT NOT NULL REFERENCES vendor_orders(id) ON DELETE CASCADE,
  order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  gross_amount BIGINT NOT NULL,
  commission_amount BIGINT NOT NULL,
  net_amount BIGINT NOT NULL,
  status TEXT NOT NULL DEFAULT 'unpaid',
  paid_at TIMESTAMPTZ NULL,
  reference TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_vendor_payouts_vendor_status ON vendor_payouts(vendor_id, status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_vendor_payouts_order ON vendor_payouts(order_id);

CREATE TABLE IF NOT EXISTS inventory_movements (
  id TEXT PRIMARY KEY,
  product_id TEXT NOT NULL REFERENCES products(id),
  order_id TEXT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
  order_item_id TEXT NOT NULL REFERENCES order_items(id) ON DELETE CASCADE,
  type TEXT NOT NULL,
  quantity NUMERIC(18,6) NOT NULL,
  note TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_inventory_movements_product_created ON inventory_movements(product_id, created_at DESC);

CREATE TABLE IF NOT EXISTS audit_logs (
  id TEXT PRIMARY KEY,
  actor_id TEXT NOT NULL REFERENCES users(id),
  actor_role TEXT NOT NULL,
  action TEXT NOT NULL,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  before_json TEXT NOT NULL DEFAULT '{}',
  after_json TEXT NOT NULL DEFAULT '{}',
  ip_address TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_entity_created ON audit_logs(entity_type, entity_id, created_at DESC);
