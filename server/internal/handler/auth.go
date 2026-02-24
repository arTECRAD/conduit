package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/arTECRAD/conduit/server/internal/model"
	"github.com/arTECRAD/conduit/server/internal/service"
)

// AuthServicer is the interface the auth handler depends on.
type AuthServicer interface {
	Register(ctx context.Context, req model.RegisterRequest) (*model.AuthResponse, string, error)
	Login(ctx context.Context, req model.LoginRequest) (*model.AuthResponse, string, error)
	Refresh(ctx context.Context, rawToken string) (*model.RefreshResponse, string, error)
	Logout(ctx context.Context, rawToken string) error
}

// AuthHandler handles auth-related HTTP routes.
type AuthHandler struct {
	svc           AuthServicer
	refreshTTL    time.Duration
	secureCookies bool
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(svc AuthServicer, refreshTTL time.Duration, secureCookies bool) *AuthHandler {
	return &AuthHandler{svc: svc, refreshTTL: refreshTTL, secureCookies: secureCookies}
}

// Register godoc
// POST /api/v1/auth/register
func (h *AuthHandler) Register(c echo.Context) error {
	var req model.RegisterRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	resp, refreshToken, err := h.svc.Register(c.Request().Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrEmailTaken) {
			return respondError(c, http.StatusConflict, "EMAIL_TAKEN", "email already in use")
		}
		return respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "registration failed")
	}

	h.setRefreshCookie(c, refreshToken)
	return respondCreated(c, resp)
}

// Login godoc
// POST /api/v1/auth/login
func (h *AuthHandler) Login(c echo.Context) error {
	var req model.LoginRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	resp, refreshToken, err := h.svc.Login(c.Request().Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return respondError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
		}
		return respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "login failed")
	}

	h.setRefreshCookie(c, refreshToken)
	return respondOK(c, resp)
}

// Refresh godoc
// POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(c echo.Context) error {
	cookie, err := c.Cookie("refresh_token")
	if err != nil {
		return respondError(c, http.StatusUnauthorized, "NO_REFRESH_TOKEN", "refresh token cookie missing")
	}

	resp, newRefreshToken, err := h.svc.Refresh(c.Request().Context(), cookie.Value)
	if err != nil {
		if errors.Is(err, service.ErrTokenInvalid) {
			return respondError(c, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "refresh token invalid or expired")
		}
		return respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "token refresh failed")
	}

	h.setRefreshCookie(c, newRefreshToken)
	return respondOK(c, resp)
}

// Logout godoc
// POST /api/v1/auth/logout
func (h *AuthHandler) Logout(c echo.Context) error {
	cookie, err := c.Cookie("refresh_token")
	if err == nil {
		_ = h.svc.Logout(c.Request().Context(), cookie.Value)
	}
	h.clearRefreshCookie(c)
	return respondOK(c, nil)
}

func (h *AuthHandler) setRefreshCookie(c echo.Context, token string) {
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.refreshTTL.Seconds()),
	}
	c.SetCookie(cookie)
}

func (h *AuthHandler) clearRefreshCookie(c echo.Context) {
	cookie := &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		HttpOnly: true,
		Secure:   h.secureCookies,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
	}
	c.SetCookie(cookie)
}
