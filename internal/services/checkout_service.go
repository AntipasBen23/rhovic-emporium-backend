package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"rhovic/backend/internal/db"
	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/repo"
	"rhovic/backend/internal/util"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	BankName    = "ACCESS BANK"
	AccountName = "RHOVIC EMPORIUM LTD."
	AccountNo   = "1895123825"
)

type CheckoutService struct {
	pool     *pgxpool.Pool
	settings *repo.SettingsRepo
}

func NewCheckoutService(pool *pgxpool.Pool, settings *repo.SettingsRepo) *CheckoutService {
	return &CheckoutService{pool: pool, settings: settings}
}

type CheckoutItem struct {
	ProductID string `json:"product_id"`
	Quantity  string `json:"quantity"`
}

type CheckoutRequest struct {
	Items []CheckoutItem `json:"items"`
}

type CheckoutVendorSummary struct {
	VendorID    string `json:"vendorId"`
	VendorName  string `json:"vendorName"`
	Subtotal    int64  `json:"subtotal"`
	Commission  int64  `json:"commission"`
	NetPayable  int64  `json:"netPayable"`
	VendorOrder string `json:"vendorOrderId"`
}

type CheckoutResponse struct {
	OrderID            string                  `json:"orderId"`
	OrderNumber        string                  `json:"orderNumber"`
	PaymentReference   string                  `json:"paymentReference"`
	PaymentMethod      string                  `json:"paymentMethod"`
	Amount             int64                   `json:"amount"`
	Currency           string                  `json:"currency"`
	Status             string                  `json:"status"`
	BankDetails        map[string]string       `json:"bankDetails"`
	PaymentInstruction string                  `json:"paymentInstruction"`
	Vendors            []CheckoutVendorSummary `json:"vendors"`
}

type checkoutProduct struct {
	ProductID           string
	VendorID            string
	VendorName          string
	Name                string
	ImageURL            string
	Price               int64
	Status              string
	VendorStatus        string
	StockQty            float64
	AdminCommissionRate *float64
	VendorOverride      *float64
}

func paymentRef() string {
	return fmt.Sprintf("PAY-%s-%s", time.Now().UTC().Format("20060102"), strings.ToUpper(util.NewID()[:6]))
}

func orderNumber() string {
	return fmt.Sprintf("ORD-%s-%s", time.Now().UTC().Format("20060102"), strings.ToUpper(util.NewID()[:6]))
}

func vendorOrderNumber() string {
	return fmt.Sprintf("VOR-%s-%s", time.Now().UTC().Format("20060102"), strings.ToUpper(util.NewID()[:6]))
}

func parseQty(raw string) (float64, string, error) {
	q := strings.TrimSpace(raw)
	if q == "" {
		return 0, "", domain.ErrInvalidInput
	}
	v, err := strconv.ParseFloat(q, 64)
	if err != nil || v <= 0 {
		return 0, "", domain.ErrInvalidInput
	}
	return v, strconv.FormatFloat(v, 'f', -1, 64), nil
}

func (s *CheckoutService) Checkout(ctx context.Context, customerID string, req CheckoutRequest) (CheckoutResponse, error) {
	if len(req.Items) == 0 {
		return CheckoutResponse{}, domain.ErrInvalidInput
	}

	defaultRate := 0.10
	if raw, _ := s.settings.Get(ctx, "commission_default_rate"); raw != "" {
		if v, err := strconv.ParseFloat(raw, 64); err == nil && v >= 0 {
			defaultRate = v
		}
	}

	out := CheckoutResponse{
		PaymentMethod: "bank_transfer",
		Currency:      "NGN",
		Status:        "pending_payment",
		BankDetails: map[string]string{
			"bankName":      BankName,
			"accountName":   AccountName,
			"accountNumber": AccountNo,
		},
		PaymentInstruction: "Transfer the exact amount and include the payment reference in narration.",
	}

	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		orderID := util.NewID()
		orderNo := orderNumber()
		ref := paymentRef()

		type builtItem struct {
			itemID      string
			vendorOrder string
			productID   string
			vendorID    string
			name        string
			imageURL    string
			qtyText     string
			unitPrice   int64
			lineTotal   int64
			commission  int64
			rate        float64
		}
		built := make([]builtItem, 0, len(req.Items))

		vendorSummary := map[string]*CheckoutVendorSummary{}
		total := int64(0)

		for _, it := range req.Items {
			qtyF, qtyText, err := parseQty(it.Quantity)
			if err != nil {
				return err
			}

			var p checkoutProduct
			err = tx.QueryRow(ctx, `
				SELECT p.id,p.vendor_id,COALESCE(v.business_name,''),p.name,COALESCE(p.image_url,''),
				       p.price,p.status,v.status,p.stock_quantity::float8,p.admin_commission_rate,v.commission_override
				FROM products p
				JOIN vendors v ON v.id=p.vendor_id
				WHERE p.id=$1
			`, it.ProductID).Scan(
				&p.ProductID, &p.VendorID, &p.VendorName, &p.Name, &p.ImageURL,
				&p.Price, &p.Status, &p.VendorStatus, &p.StockQty, &p.AdminCommissionRate, &p.VendorOverride,
			)
			if err != nil {
				return domain.ErrNotFound
			}
			if p.Status != "published" || p.VendorStatus != "approved" {
				return domain.ErrForbidden
			}
			if p.StockQty < qtyF {
				return domain.ErrInsufficient
			}

			rate := ResolveCommissionRate(defaultRate, p.VendorOverride, p.AdminCommissionRate)
			lineTotal, commission, _ := CalculateCheckoutAmounts(p.Price, qtyF, rate)

			vs := AccumulateVendorSummary(vendorSummary, VendorSplitInput{
				VendorID:    p.VendorID,
				VendorName:  p.VendorName,
				VendorOrder: util.NewID(),
				LineTotal:   lineTotal,
				Commission:  commission,
			})

			built = append(built, builtItem{
				itemID: util.NewID(), vendorOrder: vs.VendorOrder, productID: p.ProductID, vendorID: p.VendorID,
				name: p.Name, imageURL: p.ImageURL, qtyText: qtyText, unitPrice: p.Price, lineTotal: lineTotal,
				commission: commission, rate: rate,
			})
			total += lineTotal
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO orders (
				id,order_number,buyer_id,customer_id,payment_reference,currency,
				subtotal_amount,delivery_amount,discount_amount,total_amount,
				payment_method,payment_status,order_status,bank_transfer_status,status,notes,
				created_at,updated_at
			) VALUES (
				$1,$2,$3,$3,$4,'NGN',
				$5,0,0,$5,
				'bank_transfer','pending','pending_payment','pending','pending','',
				NOW(),NOW()
			)
		`, orderID, orderNo, customerID, ref, total)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO payments (
				id,order_id,payment_reference,method,provider,provider_ref,provider_reference,
				status,amount,idempotency_key,meta_json,created_at,updated_at
			) VALUES (
				$1,$2,$3,'bank_transfer','manual_bank_transfer',$3,$3,
				'pending',$4,$5,'{}',NOW(),NOW()
			)
		`, util.NewID(), orderID, ref, total, "manual:"+orderID)
		if err != nil {
			return err
		}

		for _, vs := range vendorSummary {
			_, err = tx.Exec(ctx, `
				INSERT INTO vendor_orders (
					id,order_id,vendor_id,vendor_order_number,
					subtotal_amount,delivery_amount,commission_amount,vendor_net_amount,
					fulfillment_status,payout_status,created_at,updated_at
				) VALUES ($1,$2,$3,$4,$5,0,$6,$7,'pending','unpaid',NOW(),NOW())
			`, vs.VendorOrder, orderID, vs.VendorID, vendorOrderNumber(), vs.Subtotal, vs.Commission, vs.NetPayable)
			if err != nil {
				return err
			}

			_, err = tx.Exec(ctx, `
				INSERT INTO commissions (id,order_id,vendor_order_id,vendor_id,commission_rate,gross_amount,commission_amount,created_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,NOW())
			`, util.NewID(), orderID, vs.VendorOrder, vs.VendorID, defaultRate, vs.Subtotal, vs.Commission)
			if err != nil {
				return err
			}

			_, err = tx.Exec(ctx, `
				INSERT INTO vendor_payouts (id,vendor_id,vendor_order_id,order_id,gross_amount,commission_amount,net_amount,status,reference,created_at,updated_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,'unpaid','',NOW(),NOW())
			`, util.NewID(), vs.VendorID, vs.VendorOrder, orderID, vs.Subtotal, vs.Commission, vs.NetPayable)
			if err != nil {
				return err
			}
		}

		for _, it := range built {
			_, err = tx.Exec(ctx, `
				INSERT INTO order_items (
					id,order_id,vendor_id,vendor_order_id,product_id,quantity,
					unit_price,subtotal,line_total,commission_amount,
					product_name_snapshot,product_image_snapshot,created_at
				) VALUES (
					$1,$2,$3,$4,$5,$6::numeric,
					$7,$8,$8,$9,$10,$11,NOW()
				)
			`, it.itemID, orderID, it.vendorID, it.vendorOrder, it.productID, it.qtyText,
				it.unitPrice, it.lineTotal, it.commission, it.name, it.imageURL)
			if err != nil {
				return err
			}

			_, err = tx.Exec(ctx, `
				INSERT INTO vendor_order_items (id,vendor_order_id,order_item_id,product_id,quantity,unit_price,line_total,created_at)
				VALUES ($1,$2,$3,$4,$5::numeric,$6,$7,NOW())
			`, util.NewID(), it.vendorOrder, it.itemID, it.productID, it.qtyText, it.unitPrice, it.lineTotal)
			if err != nil {
				return err
			}
		}

		out.OrderID = orderID
		out.OrderNumber = orderNo
		out.PaymentReference = ref
		out.Amount = total
		for _, v := range vendorSummary {
			out.Vendors = append(out.Vendors, *v)
		}
		return nil
	})
	if err != nil {
		return CheckoutResponse{}, err
	}
	return out, nil
}

func sanitizeExt(contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	switch ct {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "application/pdf":
		return ".pdf"
	default:
		return ""
	}
}

func proofStoragePath(fileURL string) (string, error) {
	const prefix = "/files/payment-proofs/"
	if !strings.HasPrefix(fileURL, prefix) {
		return "", domain.ErrInvalidInput
	}
	base := filepath.Base(fileURL)
	if base == "." || base == "/" || strings.Contains(base, "..") {
		return "", domain.ErrInvalidInput
	}
	return filepath.Join("uploads", "payment_proofs", base), nil
}

func (s *CheckoutService) UploadPaymentProof(ctx context.Context, customerID, orderID string, file multipart.File, header *multipart.FileHeader) (map[string]any, error) {
	if orderID == "" || file == nil || header == nil {
		return nil, domain.ErrInvalidInput
	}
	const maxFileSize = 8 * 1024 * 1024
	raw, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || len(raw) > maxFileSize {
		return nil, domain.ErrInvalidInput
	}
	detected := strings.ToLower(strings.TrimSpace(strings.Split(http.DetectContentType(raw[:min(512, len(raw))]), ";")[0]))
	ext := sanitizeExt(detected)
	if ext == "" {
		return nil, domain.ErrInvalidInput
	}
	contentType := detected

	var out map[string]any
	err = db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var ownerID string
		var paymentStatus string
		err := tx.QueryRow(ctx, `SELECT customer_id,payment_status FROM orders WHERE id=$1`, orderID).Scan(&ownerID, &paymentStatus)
		if err != nil {
			return domain.ErrNotFound
		}
		if ownerID != customerID {
			return domain.ErrForbidden
		}
		if paymentStatus == "paid" {
			return domain.ErrConflict
		}

		if err := os.MkdirAll(filepath.Join("uploads", "payment_proofs"), 0o755); err != nil {
			return err
		}
		filename := fmt.Sprintf("%s-%s%s", orderID, util.NewID(), ext)
		savePath := filepath.Join("uploads", "payment_proofs", filename)
		dst, err := os.Create(savePath)
		if err != nil {
			return err
		}
		defer dst.Close()
		if _, err := dst.Write(raw); err != nil {
			return err
		}

		fileURL := "/files/payment-proofs/" + filename
		proofID := util.NewID()
		_, err = tx.Exec(ctx, `
			INSERT INTO payment_proofs (id,order_id,uploaded_by,file_url,file_type,review_status,admin_note,created_at,updated_at)
			VALUES ($1,$2,$3,$4,$5,'pending','',NOW(),NOW())
		`, proofID, orderID, customerID, fileURL, contentType)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE orders SET payment_status='proof_uploaded', bank_transfer_status='proof_uploaded', updated_at=NOW() WHERE id=$1`, orderID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE payments SET status='proof_uploaded', updated_at=NOW() WHERE order_id=$1`, orderID)
		if err != nil {
			return err
		}
		out = map[string]any{"proof_id": proofID, "file_url": fileURL, "status": "proof_uploaded", "order_id": orderID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (s *CheckoutService) GetPaymentProofForCustomer(ctx context.Context, customerID, orderID, proofID string) (string, string, error) {
	var fileURL, fileType string
	err := s.pool.QueryRow(ctx, `
		SELECT p.file_url,p.file_type
		FROM payment_proofs p
		JOIN orders o ON o.id=p.order_id
		WHERE p.id=$1 AND p.order_id=$2 AND o.customer_id=$3
	`, proofID, orderID, customerID).Scan(&fileURL, &fileType)
	if err != nil {
		return "", "", domain.ErrNotFound
	}
	path, err := proofStoragePath(fileURL)
	if err != nil {
		return "", "", err
	}
	return path, fileType, nil
}

func (s *CheckoutService) AdminGetPaymentProof(ctx context.Context, proofID string) (string, string, error) {
	var fileURL, fileType string
	if err := s.pool.QueryRow(ctx, `SELECT file_url,file_type FROM payment_proofs WHERE id=$1`, proofID).Scan(&fileURL, &fileType); err != nil {
		return "", "", domain.ErrNotFound
	}
	path, err := proofStoragePath(fileURL)
	if err != nil {
		return "", "", err
	}
	return path, fileType, nil
}

func (s *CheckoutService) ListMyOrders(ctx context.Context, customerID string, limit, offset int) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id,order_number,payment_reference,total_amount,currency,payment_status,order_status,created_at
		FROM orders
		WHERE customer_id=$1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`, customerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, no, ref, currency, payStatus, orderStatus string
		var total int64
		var createdAt time.Time
		if err := rows.Scan(&id, &no, &ref, &total, &currency, &payStatus, &orderStatus, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "order_number": no, "payment_reference": ref, "total_amount": total, "currency": currency, "payment_status": payStatus, "order_status": orderStatus, "created_at": createdAt})
	}
	return out, nil
}

func (s *CheckoutService) GetOrderForCustomer(ctx context.Context, customerID, orderID string) (map[string]any, error) {
	var out map[string]any
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var id, no, ref, currency, payStatus, orderStatus string
		var subtotal, delivery, discount, total int64
		var createdAt time.Time
		err := tx.QueryRow(ctx, `
			SELECT id,order_number,payment_reference,currency,subtotal_amount,delivery_amount,discount_amount,total_amount,payment_status,order_status,created_at
			FROM orders WHERE id=$1 AND customer_id=$2
		`, orderID, customerID).Scan(&id, &no, &ref, &currency, &subtotal, &delivery, &discount, &total, &payStatus, &orderStatus, &createdAt)
		if err != nil {
			return domain.ErrNotFound
		}

		itemsRows, err := tx.Query(ctx, `
			SELECT product_id,product_name_snapshot,product_image_snapshot,quantity::text,unit_price,line_total,vendor_id
			FROM order_items WHERE order_id=$1 ORDER BY created_at ASC
		`, orderID)
		if err != nil {
			return err
		}
		defer itemsRows.Close()
		items := []map[string]any{}
		for itemsRows.Next() {
			var pid, name, image, qty, vendorID string
			var unitPrice, lineTotal int64
			if err := itemsRows.Scan(&pid, &name, &image, &qty, &unitPrice, &lineTotal, &vendorID); err != nil {
				return err
			}
			items = append(items, map[string]any{"product_id": pid, "name": name, "image_url": image, "quantity": qty, "unit_price": unitPrice, "line_total": lineTotal, "vendor_id": vendorID})
		}

		proofRows, err := tx.Query(ctx, `
			SELECT id,file_url,file_type,review_status,admin_note,created_at
			FROM payment_proofs WHERE order_id=$1 ORDER BY created_at DESC
		`, orderID)
		if err != nil {
			return err
		}
		defer proofRows.Close()
		proofs := []map[string]any{}
		for proofRows.Next() {
			var pid, url, typ, status, note string
			var pCreated time.Time
			if err := proofRows.Scan(&pid, &url, &typ, &status, &note, &pCreated); err != nil {
				return err
			}
			proofs = append(proofs, map[string]any{"id": pid, "file_url": url, "file_type": typ, "review_status": status, "admin_note": note, "created_at": pCreated})
		}

		out = map[string]any{
			"id": id, "order_number": no, "payment_reference": ref, "currency": currency,
			"subtotal_amount": subtotal, "delivery_amount": delivery, "discount_amount": discount,
			"total_amount": total, "payment_status": payStatus, "order_status": orderStatus, "created_at": createdAt,
			"bank_details": map[string]any{"bank_name": BankName, "account_name": AccountName, "account_number": AccountNo},
			"items":        items, "payment_proofs": proofs,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *CheckoutService) VendorListOrders(ctx context.Context, userID string, limit, offset int) ([]map[string]any, error) {
	var vendorID string
	if err := s.pool.QueryRow(ctx, `SELECT id FROM vendors WHERE user_id=$1`, userID).Scan(&vendorID); err != nil {
		return nil, domain.ErrForbidden
	}
	rows, err := s.pool.Query(ctx, `
		SELECT vo.id,vo.order_id,vo.vendor_order_number,vo.subtotal_amount,vo.commission_amount,
		       vo.vendor_net_amount,vo.fulfillment_status,vo.payout_status,vo.created_at,
		       o.payment_status,o.order_status,o.payment_reference
		FROM vendor_orders vo
		JOIN orders o ON o.id=vo.order_id
		WHERE vo.vendor_id=$1
		ORDER BY vo.created_at DESC
		LIMIT $2 OFFSET $3
	`, vendorID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, orderID, no, fulfill, payout, payStatus, orderStatus, ref string
		var subtotal, commission, net int64
		var createdAt time.Time
		if err := rows.Scan(&id, &orderID, &no, &subtotal, &commission, &net, &fulfill, &payout, &createdAt, &payStatus, &orderStatus, &ref); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "order_id": orderID, "vendor_order_number": no, "subtotal_amount": subtotal, "commission_amount": commission, "vendor_net_amount": net, "fulfillment_status": fulfill, "payout_status": payout, "created_at": createdAt, "payment_status": payStatus, "order_status": orderStatus, "payment_reference": ref})
	}
	return out, nil
}

func (s *CheckoutService) VendorGetOrder(ctx context.Context, userID, vendorOrderID string) (map[string]any, error) {
	var vendorID string
	if err := s.pool.QueryRow(ctx, `SELECT id FROM vendors WHERE user_id=$1`, userID).Scan(&vendorID); err != nil {
		return nil, domain.ErrForbidden
	}
	var out map[string]any
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var id, orderID, no, fulfill, payout string
		var subtotal, commission, net int64
		err := tx.QueryRow(ctx, `
			SELECT id,order_id,vendor_order_number,subtotal_amount,commission_amount,vendor_net_amount,fulfillment_status,payout_status
			FROM vendor_orders WHERE id=$1 AND vendor_id=$2
		`, vendorOrderID, vendorID).Scan(&id, &orderID, &no, &subtotal, &commission, &net, &fulfill, &payout)
		if err != nil {
			return domain.ErrNotFound
		}
		rows, err := tx.Query(ctx, `
			SELECT voi.product_id,oi.product_name_snapshot,oi.product_image_snapshot,voi.quantity::text,voi.unit_price,voi.line_total
			FROM vendor_order_items voi
			JOIN order_items oi ON oi.id=voi.order_item_id
			WHERE voi.vendor_order_id=$1
			ORDER BY oi.created_at ASC
		`, id)
		if err != nil {
			return err
		}
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var pid, name, image, qty string
			var unitPrice, lineTotal int64
			if err := rows.Scan(&pid, &name, &image, &qty, &unitPrice, &lineTotal); err != nil {
				return err
			}
			items = append(items, map[string]any{"product_id": pid, "name": name, "image_url": image, "quantity": qty, "unit_price": unitPrice, "line_total": lineTotal})
		}
		out = map[string]any{"id": id, "order_id": orderID, "vendor_order_number": no, "subtotal_amount": subtotal, "commission_amount": commission, "vendor_net_amount": net, "fulfillment_status": fulfill, "payout_status": payout, "items": items}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *CheckoutService) VendorUpdateFulfillment(ctx context.Context, userID, vendorOrderID, status string) error {
	allowed := map[string]bool{"pending": true, "ready_for_fulfillment": true, "processing": true, "shipped": true, "delivered": true, "cancelled": true}
	if !allowed[status] {
		return domain.ErrInvalidInput
	}
	var vendorID string
	if err := s.pool.QueryRow(ctx, `SELECT id FROM vendors WHERE user_id=$1`, userID).Scan(&vendorID); err != nil {
		return domain.ErrForbidden
	}
	ct, err := s.pool.Exec(ctx, `UPDATE vendor_orders SET fulfillment_status=$3,updated_at=NOW() WHERE id=$1 AND vendor_id=$2`, vendorOrderID, vendorID, status)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *CheckoutService) AdminListOrders(ctx context.Context, limit, offset int) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id,order_number,payment_reference,total_amount,currency,payment_status,order_status,created_at
		FROM orders ORDER BY created_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, no, ref, currency, payStatus, orderStatus string
		var total int64
		var createdAt time.Time
		if err := rows.Scan(&id, &no, &ref, &total, &currency, &payStatus, &orderStatus, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "order_number": no, "payment_reference": ref, "total_amount": total, "currency": currency, "payment_status": payStatus, "order_status": orderStatus, "created_at": createdAt})
	}
	return out, nil
}

func (s *CheckoutService) AdminGetOrder(ctx context.Context, orderID string) (map[string]any, error) {
	var out map[string]any
	err := db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var id, no, ref, currency, payStatus, orderStatus, customerID, customerEmail string
		var total int64
		err := tx.QueryRow(ctx, `
			SELECT o.id,o.order_number,o.payment_reference,o.currency,o.total_amount,o.payment_status,o.order_status,o.customer_id,COALESCE(u.email,'')
			FROM orders o
			LEFT JOIN users u ON u.id=o.customer_id
			WHERE o.id=$1
		`, orderID).Scan(&id, &no, &ref, &currency, &total, &payStatus, &orderStatus, &customerID, &customerEmail)
		if err != nil {
			return domain.ErrNotFound
		}

		rows, err := tx.Query(ctx, `
			SELECT vo.id,vo.vendor_id,COALESCE(v.business_name,''),vo.subtotal_amount,vo.commission_amount,vo.vendor_net_amount,
			       vo.fulfillment_status,vo.payout_status,
			       voi.product_id,oi.product_name_snapshot,voi.quantity::text,voi.unit_price,voi.line_total
			FROM vendor_orders vo
			JOIN vendors v ON v.id=vo.vendor_id
			LEFT JOIN vendor_order_items voi ON voi.vendor_order_id=vo.id
			LEFT JOIN order_items oi ON oi.id=voi.order_item_id
			WHERE vo.order_id=$1
			ORDER BY vo.created_at ASC
		`, orderID)
		if err != nil {
			return err
		}
		defer rows.Close()

		grouped := map[string]map[string]any{}
		ids := []string{}
		for rows.Next() {
			var voID, vendorID, vendorName, fulfill, payout string
			var subtotal, commission, net int64
			var pid, pname, qty *string
			var unitPrice, lineTotal *int64
			if err := rows.Scan(&voID, &vendorID, &vendorName, &subtotal, &commission, &net, &fulfill, &payout, &pid, &pname, &qty, &unitPrice, &lineTotal); err != nil {
				return err
			}
			entry, ok := grouped[voID]
			if !ok {
				entry = map[string]any{"vendorOrderId": voID, "vendorId": vendorID, "vendorName": vendorName, "subtotal": subtotal, "commissionAmount": commission, "vendorNetAmount": net, "fulfillmentStatus": fulfill, "payoutStatus": payout, "items": []map[string]any{}}
				grouped[voID] = entry
				ids = append(ids, voID)
			}
			if pid != nil && pname != nil && qty != nil && unitPrice != nil && lineTotal != nil {
				items := entry["items"].([]map[string]any)
				items = append(items, map[string]any{"productId": *pid, "name": *pname, "quantity": *qty, "unitPrice": *unitPrice, "lineTotal": *lineTotal})
				entry["items"] = items
			}
		}

		vendors := []map[string]any{}
		for _, id := range ids {
			vendors = append(vendors, grouped[id])
		}

		proofRows, err := tx.Query(ctx, `
			SELECT id,file_url,file_type,review_status,admin_note,created_at
			FROM payment_proofs WHERE order_id=$1 ORDER BY created_at DESC
		`, orderID)
		if err != nil {
			return err
		}
		defer proofRows.Close()
		proofs := []map[string]any{}
		for proofRows.Next() {
			var pid, url, typ, status, note string
			var createdAt time.Time
			if err := proofRows.Scan(&pid, &url, &typ, &status, &note, &createdAt); err != nil {
				return err
			}
			proofs = append(proofs, map[string]any{"id": pid, "fileUrl": url, "fileType": typ, "reviewStatus": status, "adminNote": note, "createdAt": createdAt})
		}

		out = map[string]any{"orderId": id, "orderNumber": no, "customer": map[string]any{"id": customerID, "email": customerEmail}, "paymentReference": ref, "totalAmount": total, "currency": currency, "paymentStatus": payStatus, "orderStatus": orderStatus, "vendors": vendors, "paymentProofs": proofs}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (s *CheckoutService) AdminListPendingPayments(ctx context.Context, limit, offset int) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT o.id,o.order_number,o.payment_reference,o.total_amount,o.currency,o.payment_status,o.created_at,COALESCE(u.email,'')
		FROM orders o
		LEFT JOIN users u ON u.id=o.customer_id
		WHERE o.payment_status IN ('proof_uploaded','pending')
		ORDER BY o.created_at DESC LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, no, ref, currency, status, email string
		var total int64
		var createdAt time.Time
		if err := rows.Scan(&id, &no, &ref, &total, &currency, &status, &createdAt, &email); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "order_number": no, "payment_reference": ref, "total_amount": total, "currency": currency, "payment_status": status, "created_at": createdAt, "customer_email": email})
	}
	return out, nil
}

func (s *CheckoutService) AdminApprovePayment(ctx context.Context, adminID, adminRole, orderID, ip string) error {
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx, `SELECT payment_status FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&status); err != nil {
			return domain.ErrNotFound
		}
		if status == "paid" {
			return nil
		}
		if status != "proof_uploaded" {
			return domain.ErrConflict
		}

		rows, err := tx.Query(ctx, `SELECT id,product_id,quantity::text FROM order_items WHERE order_id=$1`, orderID)
		if err != nil {
			return err
		}
		type item struct{ id, pid, qty string }
		items := []item{}
		for rows.Next() {
			var it item
			if err := rows.Scan(&it.id, &it.pid, &it.qty); err != nil {
				rows.Close()
				return err
			}
			items = append(items, it)
		}
		rows.Close()

		for _, it := range items {
			ct, err := tx.Exec(ctx, `UPDATE products SET stock_quantity = stock_quantity - ($2::numeric) WHERE id=$1 AND stock_quantity >= ($2::numeric)`, it.pid, it.qty)
			if err != nil {
				return err
			}
			if ct.RowsAffected() != 1 {
				return domain.ErrInsufficient
			}
			_, err = tx.Exec(ctx, `
				INSERT INTO inventory_movements (id,product_id,order_id,order_item_id,type,quantity,note,created_at)
				VALUES ($1,$2,$3,$4,'deduct',$5::numeric,'payment_approved',NOW())
			`, util.NewID(), it.pid, orderID, it.id, it.qty)
			if err != nil {
				return err
			}
		}

		_, err = tx.Exec(ctx, `UPDATE orders SET payment_status='paid',bank_transfer_status='approved',order_status='processing',status='paid',updated_at=NOW() WHERE id=$1`, orderID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE vendor_orders SET fulfillment_status='ready_for_fulfillment',payout_status='queued',updated_at=NOW() WHERE order_id=$1`, orderID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE vendor_payouts SET status='queued',updated_at=NOW() WHERE order_id=$1 AND status='unpaid'`, orderID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE payments SET status='success',updated_at=NOW() WHERE order_id=$1`, orderID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE payment_proofs SET review_status='approved',updated_at=NOW() WHERE order_id=$1 AND review_status='pending'`, orderID)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO audit_logs (id,actor_id,actor_role,action,entity_type,entity_id,before_json,after_json,ip_address,created_at)
			VALUES ($1,$2,$3,'approve_payment','order',$4,'{"payment_status":"proof_uploaded"}','{"payment_status":"paid"}',$5,NOW())
		`, util.NewID(), adminID, adminRole, orderID, ip)
		return err
	})
}

func (s *CheckoutService) AdminRejectPayment(ctx context.Context, adminID, adminRole, orderID, reason, ip string) error {
	if strings.TrimSpace(reason) == "" {
		reason = "Payment proof rejected"
	}
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx, `SELECT payment_status FROM orders WHERE id=$1 FOR UPDATE`, orderID).Scan(&status); err != nil {
			return domain.ErrNotFound
		}
		if status == "paid" {
			return domain.ErrConflict
		}
		_, err := tx.Exec(ctx, `UPDATE orders SET payment_status='rejected',bank_transfer_status='rejected',order_status='pending_payment',updated_at=NOW() WHERE id=$1`, orderID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE payments SET status='failed',updated_at=NOW() WHERE order_id=$1`, orderID)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE payment_proofs SET review_status='rejected',admin_note=$2,updated_at=NOW() WHERE order_id=$1 AND review_status='pending'`, orderID, reason)
		if err != nil {
			return err
		}
		afterBytes, _ := json.Marshal(map[string]any{"payment_status": "rejected", "reason": reason})
		_, err = tx.Exec(ctx, `
			INSERT INTO audit_logs (id,actor_id,actor_role,action,entity_type,entity_id,before_json,after_json,ip_address,created_at)
			VALUES ($1,$2,$3,'reject_payment','order',$4,$5,$6,$7,NOW())
		`, util.NewID(), adminID, adminRole, orderID, fmt.Sprintf(`{"payment_status":"%s"}`, status), string(afterBytes), ip)
		return err
	})
}

func (s *CheckoutService) AdminListVendorPayouts(ctx context.Context, limit, offset int) ([]map[string]any, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT vp.id,vp.vendor_id,COALESCE(v.business_name,''),vp.vendor_order_id,vp.order_id,
		       vp.gross_amount,vp.commission_amount,vp.net_amount,vp.status,vp.paid_at,vp.reference,vp.created_at
		FROM vendor_payouts vp
		JOIN vendors v ON v.id=vp.vendor_id
		ORDER BY vp.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, vendorID, vendorName, vendorOrderID, orderID, status, ref string
		var gross, commission, net int64
		var paidAt *time.Time
		var createdAt time.Time
		if err := rows.Scan(&id, &vendorID, &vendorName, &vendorOrderID, &orderID, &gross, &commission, &net, &status, &paidAt, &ref, &createdAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "vendor_id": vendorID, "vendor_name": vendorName, "vendor_order_id": vendorOrderID, "order_id": orderID, "gross_amount": gross, "commission_amount": commission, "net_amount": net, "status": status, "paid_at": paidAt, "reference": ref, "created_at": createdAt})
	}
	return out, nil
}

func (s *CheckoutService) AdminMarkVendorPayoutPaid(ctx context.Context, adminID, adminRole, payoutID, reference, ip string) error {
	if strings.TrimSpace(reference) == "" {
		return domain.ErrInvalidInput
	}
	return db.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM vendor_payouts WHERE id=$1 FOR UPDATE`, payoutID).Scan(&status); err != nil {
			return domain.ErrNotFound
		}
		if status == "paid" {
			return nil
		}
		if status != "queued" && status != "processing" && status != "unpaid" {
			return domain.ErrConflict
		}
		_, err := tx.Exec(ctx, `UPDATE vendor_payouts SET status='paid',paid_at=NOW(),reference=$2,updated_at=NOW() WHERE id=$1`, payoutID, reference)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO audit_logs (id,actor_id,actor_role,action,entity_type,entity_id,before_json,after_json,ip_address,created_at)
			VALUES ($1,$2,$3,'mark_vendor_payout_paid','vendor_payout',$4,$5,$6,$7,NOW())
		`, util.NewID(), adminID, adminRole, payoutID, fmt.Sprintf(`{"status":"%s"}`, status), fmt.Sprintf(`{"status":"paid","reference":"%s"}`, reference), ip)
		return err
	})
}
