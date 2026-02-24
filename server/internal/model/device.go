package model

import (
	"time"

	"github.com/google/uuid"
)

// DeviceStatus represents the lifecycle state of a device.
type DeviceStatus string

const (
	DeviceStatusPending  DeviceStatus = "pending"
	DeviceStatusActive   DeviceStatus = "active"
	DeviceStatusDisabled DeviceStatus = "disabled"
)

// Device represents a registered sensor device.
type Device struct {
	ID                uuid.UUID    `json:"id"`
	UserID            *uuid.UUID   `json:"user_id,omitempty"`
	DeviceIdentifier  string       `json:"device_identifier"`
	SetupCode         string       `json:"setup_code"`
	DisplayName       string       `json:"display_name"`
	Status            DeviceStatus `json:"status"`
	MQTTUsername      string       `json:"mqtt_username"`
	MQTTPasswordHash  string       `json:"-"`
	PollIntervalMs    int64        `json:"poll_interval_ms"`
	RegisteredAt      time.Time    `json:"registered_at"`
	ClaimedAt         *time.Time   `json:"claimed_at,omitempty"`
	CreatedAt         time.Time    `json:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at"`
}

// RegisterDeviceRequest is sent by ESP32 on first boot.
type RegisterDeviceRequest struct {
	DeviceIdentifier string `json:"device_identifier" validate:"required"`
	SetupCode        string `json:"setup_code"        validate:"required"`
}

// RegisterDeviceResponse is returned to the ESP32 after registration.
type RegisterDeviceResponse struct {
	DeviceID     uuid.UUID    `json:"device_id"`
	MQTTUsername string       `json:"mqtt_username"`
	MQTTPassword string       `json:"mqtt_password"`
	Status       DeviceStatus `json:"status"`
}

// ClaimDeviceRequest is sent by users to claim a pending device.
type ClaimDeviceRequest struct {
	SetupCode string `json:"setup_code" validate:"required"`
}

// UpdateDeviceRequest is used to update mutable device fields.
type UpdateDeviceRequest struct {
	DisplayName    *string `json:"display_name"`
	PollIntervalMs *int64  `json:"poll_interval_ms"`
}
