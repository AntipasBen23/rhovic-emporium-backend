package handlers

import (
	"context"
	"net/http"
	"strconv"

	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/services"

	"github.com/go-chi/chi/v5"
)

type AnalyticsHandlers struct {
	visits *services.VisitAnalyticsService
}

func NewAnalyticsHandlers(visits *services.VisitAnalyticsService) *AnalyticsHandlers {
	return &AnalyticsHandlers{visits: visits}
}

func (h *AnalyticsHandlers) TrackVisit(w http.ResponseWriter, r *http.Request) {
	var req services.VisitTrackInput
	if err := httpjson.DecodeStrict(r, &req, 65_536); err != nil {
		httpjson.Error(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	tr := services.CaptureRequest(r, req)
	httpjson.Write(w, http.StatusAccepted, map[string]any{"ok": true})

	go func() {
		_ = h.visits.Track(context.Background(), tr)
	}()
}

func (h *AnalyticsHandlers) ListVisitorSessions(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	out, err := h.visits.ListSessions(
		r.Context(),
		r.URL.Query().Get("search"),
		r.URL.Query().Get("country"),
		limit,
		offset,
	)
	if err != nil {
		httpjson.Error(w, http.StatusInternalServerError, "failed", err.Error())
		return
	}
	httpjson.Write(w, http.StatusOK, out)
}

func (h *AnalyticsHandlers) GetVisitorSession(w http.ResponseWriter, r *http.Request) {
	out, err := h.visits.GetSession(r.Context(), chi.URLParam(r, "visitorKey"))
	if err != nil {
		httpjson.Error(w, http.StatusNotFound, "not found", err.Error())
		return
	}
	httpjson.Write(w, http.StatusOK, out)
}
