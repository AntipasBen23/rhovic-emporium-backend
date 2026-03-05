package handlers

import (
	"net/http"

	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/middleware"
	"rhovic/backend/internal/services"

	"github.com/go-chi/chi/v5"
)

type VendorHandlers struct {
	vendor *services.VendorService
	orders interface {
		ListByVendor(ctx interface{}, vendorID string, limit, offset int) ([]map[string]any, error)
	}
	maxBody int64
}

func NewVendorHandlers(vendor *services.VendorService, maxBody int64) *VendorHandlers {
	return &VendorHandlers{vendor: vendor, maxBody: maxBody}
}

type applyVendorReq struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	ShopName    string `json:"shop_name"`
	ShopURL     string `json:"shop_url"`
	Phone       string `json:"phone"`
	Street      string `json:"street"`
	Street2     string `json:"street2"`
	City        string `json:"city"`
	ZipCode     string `json:"zip_code"`
	Country     string `json:"country"`
	State       string `json:"state"`
	CompanyName string `json:"company_name"`
	CompanyID   string `json:"company_id"`
	VatID       string `json:"vat_id"`
	BankName    string `json:"bank_name"`
	AccountIBAN string `json:"account_iban"`
}

func (h *VendorHandlers) Apply(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	var req applyVendorReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}

	id, err := h.vendor.Apply(r.Context(), u.UserID, domain.VendorRegisterProfile{
		FirstName:   req.FirstName,
		LastName:    req.LastName,
		ShopName:    req.ShopName,
		ShopURL:     req.ShopURL,
		Phone:       req.Phone,
		Street:      req.Street,
		Street2:     req.Street2,
		City:        req.City,
		ZipCode:     req.ZipCode,
		Country:     req.Country,
		State:       req.State,
		CompanyName: req.CompanyName,
		CompanyID:   req.CompanyID,
		VatID:       req.VatID,
		BankName:    req.BankName,
		AccountIBAN: req.AccountIBAN,
	})
	if err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 201, map[string]any{"vendor_id": id, "status": "pending"})
}

func (h *VendorHandlers) Application(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	v, err := h.vendor.GetApplication(r.Context(), u.UserID)
	if err != nil {
		httpjson.Write(w, 200, map[string]any{"has_application": false})
		return
	}
	httpjson.Write(w, 200, map[string]any{
		"has_application": true,
		"vendor_id":       v.ID,
		"status":          v.Status,
		"business_name":   v.BusinessName,
	})
}

func (h *VendorHandlers) CreateProduct(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	var req services.CreateProductReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	id, err := h.vendor.CreateProduct(r.Context(), u.UserID, req)
	if err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 201, map[string]any{"product_id": id})
}

func (h *VendorHandlers) ListProducts(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	limit := 50 // default
	offset := 0

	list, err := h.vendor.ListProducts(r.Context(), u.UserID, limit, offset)
	if err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, list)
}

func (h *VendorHandlers) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	id := chi.URLParam(r, "id")
	var req services.UpdateProductReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	if err := h.vendor.UpdateProduct(r.Context(), u.UserID, id, req); err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}

func (h *VendorHandlers) RequestPayout(w http.ResponseWriter, r *http.Request) {
	u := middleware.MustAuth(r)
	var req struct {
		Amount int64 `json:"amount"`
	}
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	id, err := h.vendor.RequestPayout(r.Context(), u.UserID, req.Amount)
	if err != nil {
		httpjson.Error(w, 400, "failed", err.Error())
		return
	}
	httpjson.Write(w, 201, map[string]any{"payout_id": id})
}

func (h *VendorHandlers) ListVendorOrders(w http.ResponseWriter, r *http.Request) {
	httpjson.Error(w, 501, "not implemented", "vendor orders listing to be wired in server routes with repo")
}
