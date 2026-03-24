CREATE TABLE IF NOT EXISTS page_visits (
  id TEXT PRIMARY KEY,
  visitor_key TEXT NOT NULL,
  path TEXT NOT NULL,
  referrer TEXT NOT NULL DEFAULT '',
  country TEXT NOT NULL DEFAULT '',
  region TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT '',
  city TEXT NOT NULL DEFAULT '',
  user_agent TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_page_visits_created_at
  ON page_visits(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_page_visits_visitor_created
  ON page_visits(visitor_key, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_page_visits_country_region_state
  ON page_visits(country, region, state, created_at DESC);
