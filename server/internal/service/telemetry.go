package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/arTECRAD/conduit/server/internal/model"
)

// ErrInvalidResolution is returned for unrecognised resolution values.
var ErrInvalidResolution = errors.New("invalid resolution; use raw, 5m, 1h, or 1d")

// TelemetryReader is the interface for querying telemetry data.
type TelemetryReader interface {
	QueryRawReadings(ctx context.Context, deviceID uuid.UUID, from, to time.Time) ([]model.SensorReading, error)
	QueryAggregated(ctx context.Context, q model.TelemetryQuery) ([]model.AggregatedReading, error)
}

// TelemetryService handles telemetry query business logic.
type TelemetryService struct {
	reader  TelemetryReader
	devices DeviceQuerier
}

// NewTelemetryService creates a new TelemetryService.
func NewTelemetryService(reader TelemetryReader, devices DeviceQuerier) *TelemetryService {
	return &TelemetryService{reader: reader, devices: devices}
}

// Query fetches telemetry for a device, validating ownership and parsing parameters.
func (s *TelemetryService) Query(ctx context.Context, userID uuid.UUID, q model.TelemetryQuery) (*model.TelemetryResponse, error) {
	device, err := s.devices.GetDeviceByID(ctx, q.DeviceID)
	if err != nil {
		return nil, ErrDeviceNotFound
	}
	if device.UserID == nil || *device.UserID != userID {
		return nil, ErrForbidden
	}

	resp := &model.TelemetryResponse{
		DeviceID:   q.DeviceID,
		Resolution: q.Resolution,
		From:       q.From,
		To:         q.To,
	}

	switch q.Resolution {
	case model.ResolutionRaw:
		readings, err := s.reader.QueryRawReadings(ctx, q.DeviceID, q.From, q.To)
		if err != nil {
			return nil, fmt.Errorf("query raw readings: %w", err)
		}
		if readings == nil {
			readings = []model.SensorReading{}
		}
		resp.Raw = readings

	case model.Resolution5m, model.Resolution1h, model.Resolution1d:
		readings, err := s.reader.QueryAggregated(ctx, q)
		if err != nil {
			return nil, fmt.Errorf("query aggregated readings: %w", err)
		}
		if readings == nil {
			readings = []model.AggregatedReading{}
		}
		resp.Aggregated = readings

	default:
		return nil, ErrInvalidResolution
	}

	return resp, nil
}
