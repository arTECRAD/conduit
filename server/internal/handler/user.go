package handler

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/arTECRAD/conduit/server/internal/model"
)

// UserServicer is the interface the user handler depends on.
type UserServicer interface {
	GetUserByID(ctx context.Context, userID uuid.UUID) (*model.UserProfile, error)
}

// UserHandler handles user-related HTTP routes.
type UserHandler struct {
	svc UserServicer
}

// NewUserHandler creates a new UserHandler.
func NewUserHandler(svc UserServicer) *UserHandler {
	return &UserHandler{svc: svc}
}

// GetProfile handles GET /api/v1/users/me
func (h *UserHandler) GetProfile(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	profile, err := h.svc.GetUserByID(c.Request().Context(), userID)
	if err != nil {
		return respondError(c, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
	}
	return respondOK(c, profile)
}
