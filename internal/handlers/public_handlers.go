package handlers

import (
	"net/http"
	"strconv"

	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/services"

	"github.com/go-chi/chi/v5"
)

type PublicHandlers struct {
	products *services.ProductsService
}

func NewPublicHandlers(products *services.ProductsService) *PublicHandlers {
	return &PublicHandlers{products: products}
}

func (h *PublicHandlers) ListProducts(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	items, err := h.products.ListPublished(r.Context(), limit, offset)
	if err != nil {
		httpjson.Error(w, 500, "failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"items": items})
}

func (h *PublicHandlers) GetProduct(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	p, err := h.products.Get(r.Context(), id)
	if err != nil {
		httpjson.Error(w, 404, "not found", "")
		return
	}
	httpjson.Write(w, 200, p)
}