package repo

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminMetricsRepo struct{ db *pgxpool.Pool }

func NewAdminMetricsRepo(db *pgxpool.Pool) *AdminMetricsRepo { return &AdminMetricsRepo{db: db} }

func (r *AdminMetricsRepo) Metrics(ctx context.Context) (map[string]any, error) {
	var totalVendors, pendingVendors, totalProducts, publishedProducts, totalOrders, failedPayments int64
	var monthlyGMV, totalCommission, pendingPayoutAmount int64
	var todayPageViews, todayUniqueVisitors int64

	err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM vendors`).Scan(&totalVendors)
	if err != nil {
		return nil, err
	}

	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM vendors WHERE status='pending'`).Scan(&pendingVendors)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&totalProducts)
	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM products WHERE status='published'`).Scan(&publishedProducts)

	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM orders`).Scan(&totalOrders)
	_ = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(total_amount),0) FROM orders WHERE created_at >= date_trunc('month', now())`).Scan(&monthlyGMV)

	_ = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(commission_amount),0) FROM order_items`).Scan(&totalCommission)
	_ = r.db.QueryRow(ctx, `SELECT COALESCE(SUM(amount),0) FROM payouts WHERE status='pending'`).Scan(&pendingPayoutAmount)

	_ = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM payments WHERE status='failed'`).Scan(&failedPayments)
	_ = r.db.QueryRow(ctx, `
		SELECT
			COUNT(*) AS page_views,
			COUNT(DISTINCT visitor_key) AS unique_visitors
		FROM page_visits
		WHERE created_at >= date_trunc('day', now())
	`).Scan(&todayPageViews, &todayUniqueVisitors)

	dailyVisits := make([]map[string]any, 0, 7)
	dailyRows, err := r.db.Query(ctx, `
		WITH days AS (
			SELECT generate_series(
				date_trunc('day', now()) - interval '6 day',
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
	`)
	if err == nil {
		defer dailyRows.Close()
		for dailyRows.Next() {
			var day string
			var pageViews, uniqueVisitors int64
			if scanErr := dailyRows.Scan(&day, &pageViews, &uniqueVisitors); scanErr != nil {
				dailyVisits = []map[string]any{}
				break
			}
			dailyVisits = append(dailyVisits, map[string]any{
				"day":             day,
				"page_views":      pageViews,
				"unique_visitors": uniqueVisitors,
			})
		}
	}

	topLocations := make([]map[string]any, 0, 10)
	locationRows, err := r.db.Query(ctx, `
		SELECT
			COALESCE(NULLIF(country, ''), 'Unknown') AS country,
			COALESCE(NULLIF(region, ''), 'Unknown') AS region,
			COALESCE(NULLIF(state, ''), 'Unknown') AS state,
			COUNT(*) AS page_views
		FROM page_visits
		WHERE created_at >= date_trunc('day', now())
		GROUP BY 1,2,3
		ORDER BY page_views DESC, country ASC, region ASC, state ASC
		LIMIT 10
	`)
	if err == nil {
		defer locationRows.Close()
		for locationRows.Next() {
			var country, region, state string
			var pageViews int64
			if scanErr := locationRows.Scan(&country, &region, &state, &pageViews); scanErr != nil {
				topLocations = []map[string]any{}
				break
			}
			topLocations = append(topLocations, map[string]any{
				"country":    country,
				"region":     region,
				"state":      state,
				"page_views": pageViews,
			})
		}
	}

	return map[string]any{
		"total_vendors":         totalVendors,
		"pending_vendors":       pendingVendors,
		"total_products":        totalProducts,
		"published_products":    publishedProducts,
		"total_orders":          totalOrders,
		"monthly_gmv":           monthlyGMV,
		"total_commission":      totalCommission,
		"pending_payout_amount": pendingPayoutAmount,
		"failed_payments":       failedPayments,
		"today_page_views":      todayPageViews,
		"today_unique_visitors": todayUniqueVisitors,
		"daily_visits":          dailyVisits,
		"top_locations":         topLocations,
	}, nil
}
