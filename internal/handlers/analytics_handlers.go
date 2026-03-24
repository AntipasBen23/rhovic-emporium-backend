package handlers

import (
	"net/http"

	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/services"
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

	if err := h.visits.Track(r.Context(), r, req); err != nil {
		httpjson.Error(w, http.StatusInternalServerError, "failed", err.Error())
		return
	}

	httpjson.Write(w, http.StatusAccepted, map[string]any{"ok": true})
}
