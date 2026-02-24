package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/arTECRAD/conduit/server/internal/middleware"
	"github.com/arTECRAD/conduit/server/internal/model"
	"github.com/arTECRAD/conduit/server/internal/service"
)

// DeviceServicer is the interface the device handler depends on.
type DeviceServicer interface {
	Register(ctx context.Context, req model.RegisterDeviceRequest) (*model.RegisterDeviceResponse, error)
	Claim(ctx context.Context, userID uuid.UUID, req model.ClaimDeviceRequest) (*model.Device, error)
	Get(ctx context.Context, userID, deviceID uuid.UUID) (*model.Device, error)
	List(ctx context.Context, userID uuid.UUID) ([]model.Device, error)
	Update(ctx context.Context, userID, deviceID uuid.UUID, req model.UpdateDeviceRequest) (*model.Device, error)
	Delete(ctx context.Context, userID, deviceID uuid.UUID) error
}

// DeviceHandler handles device-related HTTP routes.
type DeviceHandler struct {
	svc DeviceServicer
}

// NewDeviceHandler creates a new DeviceHandler.
func NewDeviceHandler(svc DeviceServicer) *DeviceHandler {
	return &DeviceHandler{svc: svc}
}

// Register handles POST /api/v1/devices/register (device-facing, provisioning token auth).
func (h *DeviceHandler) Register(c echo.Context) error {
	var req model.RegisterDeviceRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	resp, err := h.svc.Register(c.Request().Context(), req)
	if err != nil {
		if errors.Is(err, service.ErrDeviceAlreadyRegistered) {
			return respondError(c, http.StatusConflict, "DEVICE_ALREADY_REGISTERED", "device already registered")
		}
		return respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "device registration failed")
	}
	return respondCreated(c, resp)
}

// Claim handles POST /api/v1/devices/claim (user-facing).
func (h *DeviceHandler) Claim(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	var req model.ClaimDeviceRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	device, err := h.svc.Claim(c.Request().Context(), userID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrSetupCodeNotFound):
			return respondError(c, http.StatusNotFound, "SETUP_CODE_NOT_FOUND", "setup code not found")
		case errors.Is(err, service.ErrDeviceAlreadyClaimed):
			return respondError(c, http.StatusConflict, "DEVICE_ALREADY_CLAIMED", "device already claimed")
		default:
			return respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "claim failed")
		}
	}
	return respondOK(c, device)
}

// List handles GET /api/v1/devices.
func (h *DeviceHandler) List(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	devices, err := h.svc.List(c.Request().Context(), userID)
	if err != nil {
		return respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list devices")
	}
	return respondOK(c, devices)
}

// Get handles GET /api/v1/devices/:id.
func (h *DeviceHandler) Get(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	deviceID, err := parseUUID(c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusBadRequest, "INVALID_DEVICE_ID", "invalid device ID")
	}

	device, err := h.svc.Get(c.Request().Context(), userID, deviceID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDeviceNotFound):
			return respondError(c, http.StatusNotFound, "DEVICE_NOT_FOUND", "device not found")
		case errors.Is(err, service.ErrForbidden):
			return respondError(c, http.StatusForbidden, "FORBIDDEN", "access denied")
		default:
			return respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get device")
		}
	}
	return respondOK(c, device)
}

// Update handles PATCH /api/v1/devices/:id.
func (h *DeviceHandler) Update(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	deviceID, err := parseUUID(c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusBadRequest, "INVALID_DEVICE_ID", "invalid device ID")
	}

	var req model.UpdateDeviceRequest
	if err := c.Bind(&req); err != nil {
		return respondError(c, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
	}

	device, err := h.svc.Update(c.Request().Context(), userID, deviceID, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDeviceNotFound):
			return respondError(c, http.StatusNotFound, "DEVICE_NOT_FOUND", "device not found")
		case errors.Is(err, service.ErrForbidden):
			return respondError(c, http.StatusForbidden, "FORBIDDEN", "access denied")
		default:
			return respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "update failed")
		}
	}
	return respondOK(c, device)
}

// Delete handles DELETE /api/v1/devices/:id.
func (h *DeviceHandler) Delete(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	deviceID, err := parseUUID(c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusBadRequest, "INVALID_DEVICE_ID", "invalid device ID")
	}

	if err := h.svc.Delete(c.Request().Context(), userID, deviceID); err != nil {
		switch {
		case errors.Is(err, service.ErrDeviceNotFound):
			return respondError(c, http.StatusNotFound, "DEVICE_NOT_FOUND", "device not found")
		case errors.Is(err, service.ErrForbidden):
			return respondError(c, http.StatusForbidden, "FORBIDDEN", "access denied")
		default:
			return respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "delete failed")
		}
	}
	return c.NoContent(http.StatusNoContent)
}

func getUserID(c echo.Context) (uuid.UUID, error) {
	raw, ok := c.Get(middleware.ContextKeyUserID).(string)
	if !ok || raw == "" {
		return uuid.UUID{}, errors.New("no user ID in context")
	}
	return uuid.Parse(raw)
}

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
