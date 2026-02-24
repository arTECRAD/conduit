package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/arTECRAD/conduit/server/internal/model"
	"github.com/arTECRAD/conduit/server/internal/service"
)

// TelemetryServicer is the interface the telemetry handler depends on.
type TelemetryServicer interface {
	Query(ctx context.Context, userID uuid.UUID, q model.TelemetryQuery) (*model.TelemetryResponse, error)
}

// TelemetryHandler handles telemetry query HTTP routes.
type TelemetryHandler struct {
	svc TelemetryServicer
}

// NewTelemetryHandler creates a new TelemetryHandler.
func NewTelemetryHandler(svc TelemetryServicer) *TelemetryHandler {
	return &TelemetryHandler{svc: svc}
}

// Get handles GET /api/v1/devices/:id/telemetry
func (h *TelemetryHandler) Get(c echo.Context) error {
	userID, err := getUserID(c)
	if err != nil {
		return respondError(c, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
	}

	deviceID, err := parseUUID(c.Param("id"))
	if err != nil {
		return respondError(c, http.StatusBadRequest, "INVALID_DEVICE_ID", "invalid device ID")
	}

	fromStr := c.QueryParam("from")
	toStr := c.QueryParam("to")
	resolutionStr := c.QueryParam("resolution")

	if fromStr == "" || toStr == "" {
		return respondError(c, http.StatusBadRequest, "MISSING_PARAMS", "from and to query parameters are required")
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		return respondError(c, http.StatusBadRequest, "INVALID_FROM", "from must be ISO8601 (RFC3339)")
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		return respondError(c, http.StatusBadRequest, "INVALID_TO", "to must be ISO8601 (RFC3339)")
	}

	resolution := model.Resolution(resolutionStr)
	if resolution == "" {
		resolution = model.ResolutionRaw
	}

	q := model.TelemetryQuery{
		DeviceID:   deviceID,
		From:       from,
		To:         to,
		Resolution: resolution,
	}

	resp, err := h.svc.Query(c.Request().Context(), userID, q)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDeviceNotFound):
			return respondError(c, http.StatusNotFound, "DEVICE_NOT_FOUND", "device not found")
		case errors.Is(err, service.ErrForbidden):
			return respondError(c, http.StatusForbidden, "FORBIDDEN", "access denied")
		case errors.Is(err, service.ErrInvalidResolution):
			return respondError(c, http.StatusBadRequest, "INVALID_RESOLUTION", err.Error())
		default:
			return respondError(c, http.StatusInternalServerError, "INTERNAL_ERROR", "telemetry query failed")
		}
	}
	return respondOK(c, resp)
}
