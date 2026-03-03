package handlers

import (
	"net/http"

	"rhovic/backend/internal/domain"
	"rhovic/backend/internal/httpjson"
	"rhovic/backend/internal/services"
)

type AuthHandlers struct {
	auth    *services.AuthService
	maxBody int64
}

func NewAuthHandlers(auth *services.AuthService, maxBody int64) *AuthHandlers {
	return &AuthHandlers{auth: auth, maxBody: maxBody}
}

type registerReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (h *AuthHandlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	role := domain.Role(req.Role)
	if role == "" { role = domain.RoleBuyer }
	if role == domain.RoleAdminSuper || role == domain.RoleAdminOps || role == domain.RoleAdminFin {
		httpjson.Error(w, 403, "forbidden", "cannot self-register as admin")
		return
	}
	id, err := h.auth.Register(r.Context(), req.Email, req.Password, role)
	if err != nil {
		httpjson.Error(w, 400, "registration failed", err.Error())
		return
	}
	httpjson.Write(w, 201, map[string]any{"user_id": id})
}

type loginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	at, rt, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpjson.Error(w, 401, "invalid credentials", "")
		return
	}
	httpjson.Write(w, 200, map[string]any{"access_token": at, "refresh_token": rt})
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandlers) Refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	at, rt, err := h.auth.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		httpjson.Error(w, 401, "invalid refresh token", "")
		return
	}
	httpjson.Write(w, 200, map[string]any{"access_token": at, "refresh_token": rt})
}

type logoutReq struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandlers) Logout(w http.ResponseWriter, r *http.Request) {
	var req logoutReq
	if err := httpjson.DecodeStrict(r, &req, h.maxBody); err != nil {
		httpjson.Error(w, 400, "bad request", err.Error())
		return
	}
	if err := h.auth.Logout(r.Context(), req.RefreshToken); err != nil {
		httpjson.Error(w, 400, "logout failed", err.Error())
		return
	}
	httpjson.Write(w, 200, map[string]any{"ok": true})
}