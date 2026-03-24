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
			id, visitor_key, path, referrer, country, region, state, city, user_agent, created_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`, event.ID, event.VisitorKey, event.Path, event.Referrer, event.Country, event.Region, event.State, event.City, event.UserAgent, event.CreatedAt)
	return err
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
