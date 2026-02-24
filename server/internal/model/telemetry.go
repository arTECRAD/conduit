package model

import (
	"time"

	"github.com/google/uuid"
)

// SensorReading is a single raw telemetry record.
type SensorReading struct {
	Time         time.Time  `json:"time"`
	DeviceID     uuid.UUID  `json:"device_id"`
	TemperatureC *float64   `json:"temperature_c,omitempty"`
	HumidityPct  *float64   `json:"humidity_pct,omitempty"`
	Eco2Ppm      *float64   `json:"eco2_ppm,omitempty"`
	TvocPpb      *float64   `json:"tvoc_ppb,omitempty"`
	Pm25Ugm3     *float64   `json:"pm25_ugm3,omitempty"`
	Pm10Ugm3     *float64   `json:"pm10_ugm3,omitempty"`
}

// AggregatedReading is a time-bucketed aggregate of sensor data.
type AggregatedReading struct {
	Bucket           time.Time `json:"bucket"`
	DeviceID         uuid.UUID `json:"device_id"`
	AvgTemperatureC  *float64  `json:"avg_temperature_c,omitempty"`
	MinTemperatureC  *float64  `json:"min_temperature_c,omitempty"`
	MaxTemperatureC  *float64  `json:"max_temperature_c,omitempty"`
	AvgHumidityPct   *float64  `json:"avg_humidity_pct,omitempty"`
	MinHumidityPct   *float64  `json:"min_humidity_pct,omitempty"`
	MaxHumidityPct   *float64  `json:"max_humidity_pct,omitempty"`
	AvgEco2Ppm       *float64  `json:"avg_eco2_ppm,omitempty"`
	MinEco2Ppm       *float64  `json:"min_eco2_ppm,omitempty"`
	MaxEco2Ppm       *float64  `json:"max_eco2_ppm,omitempty"`
	AvgTvocPpb       *float64  `json:"avg_tvoc_ppb,omitempty"`
	MinTvocPpb       *float64  `json:"min_tvoc_ppb,omitempty"`
	MaxTvocPpb       *float64  `json:"max_tvoc_ppb,omitempty"`
	AvgPm25Ugm3      *float64  `json:"avg_pm25_ugm3,omitempty"`
	MinPm25Ugm3      *float64  `json:"min_pm25_ugm3,omitempty"`
	MaxPm25Ugm3      *float64  `json:"max_pm25_ugm3,omitempty"`
	AvgPm10Ugm3      *float64  `json:"avg_pm10_ugm3,omitempty"`
	MinPm10Ugm3      *float64  `json:"min_pm10_ugm3,omitempty"`
	MaxPm10Ugm3      *float64  `json:"max_pm10_ugm3,omitempty"`
}

// Resolution describes the aggregation level for a telemetry query.
type Resolution string

const (
	ResolutionRaw Resolution = "raw"
	Resolution5m  Resolution = "5m"
	Resolution1h  Resolution = "1h"
	Resolution1d  Resolution = "1d"
)

// TelemetryQuery holds the parsed query parameters for telemetry requests.
type TelemetryQuery struct {
	DeviceID   uuid.UUID
	From       time.Time
	To         time.Time
	Resolution Resolution
}

// TelemetryResponse wraps both raw and aggregated results in a unified shape.
type TelemetryResponse struct {
	DeviceID   uuid.UUID           `json:"device_id"`
	Resolution Resolution          `json:"resolution"`
	From       time.Time           `json:"from"`
	To         time.Time           `json:"to"`
	Raw        []SensorReading     `json:"readings,omitempty"`
	Aggregated []AggregatedReading `json:"readings_aggregated,omitempty"`
}
