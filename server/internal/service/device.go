package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/arTECRAD/conduit/server/internal/auth"
	"github.com/arTECRAD/conduit/server/internal/model"
	"github.com/arTECRAD/conduit/server/internal/repository"
)

// ErrDeviceNotFound is returned when a device does not exist.
var ErrDeviceNotFound = errors.New("device not found")

// ErrDeviceAlreadyRegistered is returned when a device_identifier is already in the DB.
var ErrDeviceAlreadyRegistered = errors.New("device already registered")

// ErrDeviceAlreadyClaimed is returned when a device has already been claimed.
var ErrDeviceAlreadyClaimed = errors.New("device already claimed")

// ErrSetupCodeNotFound is returned when no device matches the provided setup code.
var ErrSetupCodeNotFound = errors.New("setup code not found")

// ErrForbidden is returned when a user attempts an action on another user's device.
var ErrForbidden = errors.New("forbidden")

// DeviceQuerier is the subset of repository.Queries used by DeviceService.
type DeviceQuerier interface {
	CreateDevice(ctx context.Context, arg repository.CreateDeviceParams) (repository.Device, error)
	GetDeviceByID(ctx context.Context, id uuid.UUID) (repository.Device, error)
	GetDeviceByIdentifier(ctx context.Context, deviceIdentifier string) (repository.Device, error)
	GetDeviceBySetupCode(ctx context.Context, setupCode string) (repository.Device, error)
	ListDevicesByUserID(ctx context.Context, userID *uuid.UUID) ([]repository.Device, error)
	ClaimDevice(ctx context.Context, arg repository.ClaimDeviceParams) (repository.Device, error)
	UpdateDevice(ctx context.Context, arg repository.UpdateDeviceParams) (repository.Device, error)
	DeleteDevice(ctx context.Context, arg repository.DeleteDeviceParams) error
}

// CommandPublisher publishes MQTT commands to devices.
type CommandPublisher interface {
	Publish(ctx context.Context, deviceID uuid.UUID, cmd model.CommandPayload) error
}

// DeviceService handles device business logic.
type DeviceService struct {
	q          DeviceQuerier
	commander  CommandPublisher
	bcryptCost int
}

// NewDeviceService creates a new DeviceService.
func NewDeviceService(q DeviceQuerier, commander CommandPublisher, bcryptCost int) *DeviceService {
	return &DeviceService{q: q, commander: commander, bcryptCost: bcryptCost}
}

// Register handles ESP32 device self-registration on first boot.
func (s *DeviceService) Register(ctx context.Context, req model.RegisterDeviceRequest) (*model.RegisterDeviceResponse, error) {
	// Check for existing registration
	existing, err := s.q.GetDeviceByIdentifier(ctx, req.DeviceIdentifier)
	if err == nil {
		// Already registered — return existing MQTT credentials would require storing
		// plaintext passwords, which we don't do. Return an error instead.
		_ = existing
		return nil, ErrDeviceAlreadyRegistered
	}

	mqttUsername := "device_" + req.DeviceIdentifier
	mqttPassword, err := generateMQTTPassword()
	if err != nil {
		return nil, fmt.Errorf("generate MQTT password: %w", err)
	}

	mqttPasswordHash, err := auth.HashPassword(mqttPassword, s.bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash MQTT password: %w", err)
	}

	device, err := s.q.CreateDevice(ctx, repository.CreateDeviceParams{
		DeviceIdentifier: req.DeviceIdentifier,
		SetupCode:        req.SetupCode,
		MqttUsername:     mqttUsername,
		MqttPasswordHash: mqttPasswordHash,
	})
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrDeviceAlreadyRegistered
		}
		return nil, fmt.Errorf("create device: %w", err)
	}

	return &model.RegisterDeviceResponse{
		DeviceID:     device.ID,
		MQTTUsername: mqttUsername,
		MQTTPassword: mqttPassword,
		Status:       model.DeviceStatusPending,
	}, nil
}

// Claim claims a pending device for the authenticated user.
func (s *DeviceService) Claim(ctx context.Context, userID uuid.UUID, req model.ClaimDeviceRequest) (*model.Device, error) {
	device, err := s.q.GetDeviceBySetupCode(ctx, req.SetupCode)
	if err != nil {
		return nil, ErrSetupCodeNotFound
	}

	if device.Status != repository.DeviceStatusPending {
		return nil, ErrDeviceAlreadyClaimed
	}

	claimed, err := s.q.ClaimDevice(ctx, repository.ClaimDeviceParams{
		ID:     device.ID,
		UserID: &userID,
	})
	if err != nil {
		return nil, fmt.Errorf("claim device: %w", err)
	}

	// Publish activate command
	cmd := model.CommandPayload{
		CommandID: uuid.New(),
		Type:      model.CommandActivate,
	}
	if pubErr := s.commander.Publish(ctx, claimed.ID, cmd); pubErr != nil {
		// Log but don't fail the claim — device will re-poll or reconnect
		fmt.Printf("warn: publish activate command for device %s: %v\n", claimed.ID, pubErr)
	}

	return repoDeviceToModel(claimed), nil
}

// Get returns a device owned by the specified user.
func (s *DeviceService) Get(ctx context.Context, userID, deviceID uuid.UUID) (*model.Device, error) {
	device, err := s.q.GetDeviceByID(ctx, deviceID)
	if err != nil {
		return nil, ErrDeviceNotFound
	}
	if device.UserID == nil || *device.UserID != userID {
		return nil, ErrForbidden
	}
	return repoDeviceToModel(device), nil
}

// List returns all devices owned by the user.
func (s *DeviceService) List(ctx context.Context, userID uuid.UUID) ([]model.Device, error) {
	devices, err := s.q.ListDevicesByUserID(ctx, &userID)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	result := make([]model.Device, len(devices))
	for i, d := range devices {
		result[i] = *repoDeviceToModel(d)
	}
	return result, nil
}

// Update applies mutable field changes to a device.
func (s *DeviceService) Update(ctx context.Context, userID, deviceID uuid.UUID, req model.UpdateDeviceRequest) (*model.Device, error) {
	existing, err := s.q.GetDeviceByID(ctx, deviceID)
	if err != nil {
		return nil, ErrDeviceNotFound
	}
	if existing.UserID == nil || *existing.UserID != userID {
		return nil, ErrForbidden
	}

	displayName := existing.DisplayName
	if req.DisplayName != nil {
		displayName = *req.DisplayName
	}
	pollInterval := existing.PollIntervalMs
	if req.PollIntervalMs != nil {
		pollInterval = *req.PollIntervalMs
	}

	updated, err := s.q.UpdateDevice(ctx, repository.UpdateDeviceParams{
		ID:             deviceID,
		DisplayName:    displayName,
		PollIntervalMs: pollInterval,
		UserID:         &userID,
	})
	if err != nil {
		return nil, fmt.Errorf("update device: %w", err)
	}
	return repoDeviceToModel(updated), nil
}

// Delete removes a device from the user's account.
func (s *DeviceService) Delete(ctx context.Context, userID, deviceID uuid.UUID) error {
	existing, err := s.q.GetDeviceByID(ctx, deviceID)
	if err != nil {
		return ErrDeviceNotFound
	}
	if existing.UserID == nil || *existing.UserID != userID {
		return ErrForbidden
	}
	return s.q.DeleteDevice(ctx, repository.DeleteDeviceParams{
		ID:     deviceID,
		UserID: &userID,
	})
}

func generateMQTTPassword() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func repoDeviceToModel(d repository.Device) *model.Device {
	return &model.Device{
		ID:               d.ID,
		UserID:           d.UserID,
		DeviceIdentifier: d.DeviceIdentifier,
		SetupCode:        d.SetupCode,
		DisplayName:      d.DisplayName,
		Status:           model.DeviceStatus(d.Status),
		MQTTUsername:     d.MqttUsername,
		MQTTPasswordHash: d.MqttPasswordHash,
		PollIntervalMs:   d.PollIntervalMs,
		RegisteredAt:     d.RegisteredAt,
		ClaimedAt:        d.ClaimedAt,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}
}
