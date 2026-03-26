package repo

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type VisitAnalyticsRepo struct {
	db *pgxpool.Pool
}

type VisitEvent struct {
	ID         string
	VisitorKey string
	UserID     *string
	UserEmail  string
	Path       string
	Referrer   string
	Country    string
	Region     string
	State      string
	City       string
	UserAgent  string
	CreatedAt  time.Time
}

type DailyVisitStat struct {
	Day            string `json:"day"`
	PageViews      int64  `json:"page_views"`
	UniqueVisitors int64  `json:"unique_visitors"`
}

type LocationStat struct {
	Country   string `json:"country"`
	Region    string `json:"region"`
	State     string `json:"state"`
	PageViews int64  `json:"page_views"`
}

func NewVisitAnalyticsRepo(db *pgxpool.Pool) *VisitAnalyticsRepo {
	return &VisitAnalyticsRepo{db: db}
}

func (r *VisitAnalyticsRepo) Create(ctx context.Context, event VisitEvent) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO page_visits (
			id, visitor_key, user_id, user_email, path, referrer, country, region, state, city, user_agent, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	`, event.ID, event.VisitorKey, event.UserID, event.UserEmail, event.Path, event.Referrer, event.Country, event.Region, event.State, event.City, event.UserAgent, event.CreatedAt)
	return err
}

type VisitorSessionListItem struct {
	VisitorKey string    `json:"visitor_key"`
	UserID     *string   `json:"user_id,omitempty"`
	UserEmail  string    `json:"user_email"`
	Country    string    `json:"country"`
	Region     string    `json:"region"`
	State      string    `json:"state"`
	City       string    `json:"city"`
	UserAgent  string    `json:"user_agent"`
	LatestPath string    `json:"latest_path"`
	Referrer   string    `json:"referrer"`
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
	PageViews  int64     `json:"page_views"`
}

type VisitorSessionListResult struct {
	Items []VisitorSessionListItem `json:"items"`
	Total int64                    `json:"total"`
}

type VisitorPageHit struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Referrer  string    `json:"referrer"`
	CreatedAt time.Time `json:"created_at"`
	UserAgent string    `json:"user_agent"`
}

type VisitorSessionDetail struct {
	Session VisitorSessionListItem `json:"session"`
	Hits    []VisitorPageHit       `json:"hits"`
}

func (r *VisitAnalyticsRepo) ListSessions(ctx context.Context, search, country string, limit, offset int) (VisitorSessionListResult, error) {
	search = "%" + search + "%"

	var total int64
	err := r.db.QueryRow(ctx, `
		WITH session_rows AS (
			SELECT DISTINCT visitor_key
			FROM page_visits
			WHERE ($1 = '%%' OR user_email ILIKE $1 OR path ILIKE $1 OR country ILIKE $1 OR region ILIKE $1 OR state ILIKE $1)
			  AND ($2 = '' OR country = $2)
		)
		SELECT COUNT(*) FROM session_rows
	`, search, country).Scan(&total)
	if err != nil {
		return VisitorSessionListResult{}, err
	}

	rows, err := r.db.Query(ctx, `
		WITH filtered AS (
			SELECT *
			FROM page_visits
			WHERE ($1 = '%%' OR user_email ILIKE $1 OR path ILIKE $1 OR country ILIKE $1 OR region ILIKE $1 OR state ILIKE $1)
			  AND ($2 = '' OR country = $2)
		),
		ranked AS (
			SELECT
				p.*,
				ROW_NUMBER() OVER (PARTITION BY visitor_key ORDER BY created_at DESC) AS rn
			FROM filtered p
		)
		SELECT
			r.visitor_key,
			r.user_id,
			COALESCE(NULLIF(r.user_email, ''), 'Anonymous') AS user_email,
			COALESCE(NULLIF(r.country, ''), 'Unknown') AS country,
			COALESCE(NULLIF(r.region, ''), 'Unknown') AS region,
			COALESCE(NULLIF(r.state, ''), 'Unknown') AS state,
			COALESCE(NULLIF(r.city, ''), 'Unknown') AS city,
			COALESCE(NULLIF(r.user_agent, ''), '-') AS user_agent,
			COALESCE(NULLIF(r.path, ''), '/') AS latest_path,
			COALESCE(NULLIF(r.referrer, ''), '-') AS referrer,
			agg.first_seen,
			agg.last_seen,
			agg.page_views
		FROM ranked r
		JOIN (
			SELECT visitor_key, MIN(created_at) AS first_seen, MAX(created_at) AS last_seen, COUNT(*) AS page_views
			FROM filtered
			GROUP BY visitor_key
		) agg ON agg.visitor_key = r.visitor_key
		WHERE r.rn = 1
		ORDER BY agg.last_seen DESC
		LIMIT $3 OFFSET $4
	`, search, country, limit, offset)
	if err != nil {
		return VisitorSessionListResult{}, err
	}
	defer rows.Close()

	items := []VisitorSessionListItem{}
	for rows.Next() {
		var item VisitorSessionListItem
		if err := rows.Scan(
			&item.VisitorKey,
			&item.UserID,
			&item.UserEmail,
			&item.Country,
			&item.Region,
			&item.State,
			&item.City,
			&item.UserAgent,
			&item.LatestPath,
			&item.Referrer,
			&item.FirstSeen,
			&item.LastSeen,
			&item.PageViews,
		); err != nil {
			return VisitorSessionListResult{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return VisitorSessionListResult{}, err
	}
	return VisitorSessionListResult{Items: items, Total: total}, nil
}

func (r *VisitAnalyticsRepo) GetSession(ctx context.Context, visitorKey string) (VisitorSessionDetail, error) {
	var detail VisitorSessionDetail
	err := r.db.QueryRow(ctx, `
		WITH agg AS (
			SELECT visitor_key, MIN(created_at) AS first_seen, MAX(created_at) AS last_seen, COUNT(*) AS page_views
			FROM page_visits
			WHERE visitor_key = $1
			GROUP BY visitor_key
		)
		SELECT
			p.visitor_key,
			p.user_id,
			COALESCE(NULLIF(p.user_email, ''), 'Anonymous') AS user_email,
			COALESCE(NULLIF(p.country, ''), 'Unknown') AS country,
			COALESCE(NULLIF(p.region, ''), 'Unknown') AS region,
			COALESCE(NULLIF(p.state, ''), 'Unknown') AS state,
			COALESCE(NULLIF(p.city, ''), 'Unknown') AS city,
			COALESCE(NULLIF(p.user_agent, ''), '-') AS user_agent,
			COALESCE(NULLIF(p.path, ''), '/') AS latest_path,
			COALESCE(NULLIF(p.referrer, ''), '-') AS referrer,
			agg.first_seen,
			agg.last_seen,
			agg.page_views
		FROM page_visits p
		JOIN agg ON agg.visitor_key = p.visitor_key
		WHERE p.visitor_key = $1
		ORDER BY p.created_at DESC
		LIMIT 1
	`, visitorKey).Scan(
		&detail.Session.VisitorKey,
		&detail.Session.UserID,
		&detail.Session.UserEmail,
		&detail.Session.Country,
		&detail.Session.Region,
		&detail.Session.State,
		&detail.Session.City,
		&detail.Session.UserAgent,
		&detail.Session.LatestPath,
		&detail.Session.Referrer,
		&detail.Session.FirstSeen,
		&detail.Session.LastSeen,
		&detail.Session.PageViews,
	)
	if err != nil {
		return detail, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, path, COALESCE(referrer, ''), created_at, COALESCE(user_agent, '')
		FROM page_visits
		WHERE visitor_key = $1
		ORDER BY created_at DESC
		LIMIT 200
	`, visitorKey)
	if err != nil {
		return detail, err
	}
	defer rows.Close()

	hits := []VisitorPageHit{}
	for rows.Next() {
		var hit VisitorPageHit
		if err := rows.Scan(&hit.ID, &hit.Path, &hit.Referrer, &hit.CreatedAt, &hit.UserAgent); err != nil {
			return detail, err
		}
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return detail, err
	}
	detail.Hits = hits
	return detail, nil
}

func (r *VisitAnalyticsRepo) TodaySummary(ctx context.Context) (pageViews, uniqueVisitors int64, err error) {
	err = r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) AS page_views,
			COUNT(DISTINCT visitor_key) AS unique_visitors
		FROM page_visits
		WHERE created_at >= date_trunc('day', now())
	`).Scan(&pageViews, &uniqueVisitors)
	return
}

func (r *VisitAnalyticsRepo) DailySummary(ctx context.Context, days int) ([]DailyVisitStat, error) {
	rows, err := r.db.Query(ctx, `
		WITH days AS (
			SELECT generate_series(
				date_trunc('day', now()) - (($1::int - 1) * interval '1 day'),
				date_trunc('day', now()),
				interval '1 day'
			) AS day
		)
		SELECT
			to_char(days.day, 'YYYY-MM-DD') AS day,
			COALESCE(COUNT(v.id), 0) AS page_views,
			COALESCE(COUNT(DISTINCT v.visitor_key), 0) AS unique_visitors
		FROM days
		LEFT JOIN page_visits v
			ON v.created_at >= days.day
			AND v.created_at < days.day + interval '1 day'
		GROUP BY days.day
		ORDER BY days.day ASC
	`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]DailyVisitStat, 0, days)
	for rows.Next() {
		var item DailyVisitStat
		if err := rows.Scan(&item.Day, &item.PageViews, &item.UniqueVisitors); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *VisitAnalyticsRepo) TopLocationsToday(ctx context.Context, limit int) ([]LocationStat, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			COALESCE(NULLIF(country, ''), 'Unknown') AS country,
			COALESCE(NULLIF(region, ''), 'Unknown') AS region,
			COALESCE(NULLIF(state, ''), 'Unknown') AS state,
			COUNT(*) AS page_views
		FROM page_visits
		WHERE created_at >= date_trunc('day', now())
		GROUP BY 1,2,3
		ORDER BY page_views DESC, country ASC, region ASC, state ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]LocationStat, 0, limit)
	for rows.Next() {
		var item LocationStat
		if err := rows.Scan(&item.Country, &item.Region, &item.State, &item.PageViews); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
