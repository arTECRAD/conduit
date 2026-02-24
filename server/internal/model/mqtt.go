package model

import "github.com/google/uuid"

// TelemetryPayload is the JSON structure published by ESP32 devices.
type TelemetryPayload struct {
	DeviceID  string          `json:"device_id"`
	Timestamp int64           `json:"timestamp"`
	Sensors   SensorValues    `json:"sensors"`
}

// SensorValues holds all sensor readings from a telemetry payload.
type SensorValues struct {
	TemperatureC *float64 `json:"temperature_c,omitempty"`
	HumidityPct  *float64 `json:"humidity_pct,omitempty"`
	Eco2Ppm      *float64 `json:"eco2_ppm,omitempty"`
	TvocPpb      *float64 `json:"tvoc_ppb,omitempty"`
	Pm25Ugm3     *float64 `json:"pm25_ugm3,omitempty"`
	Pm10Ugm3     *float64 `json:"pm10_ugm3,omitempty"`
}

// CommandType enumerates MQTT command types the server can send to devices.
type CommandType string

const (
	CommandSetPollInterval       CommandType = "set_poll_interval"
	CommandFactoryReset          CommandType = "factory_reset"
	CommandReboot                CommandType = "reboot"
	CommandActivate              CommandType = "activate"
	CommandUpdateMQTTCredentials CommandType = "update_mqtt_credentials"
)

// CommandPayload is the JSON structure published by the server to a device.
type CommandPayload struct {
	CommandID uuid.UUID   `json:"command_id"`
	Type      CommandType `json:"type"`
	Payload   interface{} `json:"payload,omitempty"`
}

// SetPollIntervalPayload is the payload for the set_poll_interval command.
type SetPollIntervalPayload struct {
	IntervalMs int64 `json:"interval_ms"`
}

// CommandAck is published by a device to acknowledge a command.
type CommandAck struct {
	CommandID uuid.UUID `json:"command_id"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
}
