package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"github.com/arTECRAD/conduit/server/internal/model"
)

const telemetryChannelSize = 256

// ReadingInserter can insert a sensor reading into the database.
type ReadingInserter interface {
	InsertSensorReadingRaw(ctx context.Context, reading model.SensorReading) error
}

// Ingester subscribes to device telemetry topics and persists readings.
type Ingester struct {
	client      paho.Client
	repo        ReadingInserter
	telemetryCh chan paho.Message
	done        chan struct{}
}

// NewIngester creates a new Ingester and subscribes to wildcard telemetry topic.
func NewIngester(client paho.Client, repo ReadingInserter) (*Ingester, error) {
	ing := &Ingester{
		client:      client,
		repo:        repo,
		telemetryCh: make(chan paho.Message, telemetryChannelSize),
		done:        make(chan struct{}),
	}

	token := client.Subscribe("devices/+/telemetry", 1, ing.handleTelemetry)
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("subscribe telemetry: %w", err)
	}

	go ing.drain()
	return ing, nil
}

// handleTelemetry is called by paho on each incoming telemetry message.
// It dispatches to a buffered channel to avoid blocking paho's receive loop.
func (ing *Ingester) handleTelemetry(_ paho.Client, msg paho.Message) {
	select {
	case ing.telemetryCh <- msg:
	default:
		slog.Warn("telemetry channel full, dropping message", "topic", msg.Topic())
	}
}

// drain processes messages from the telemetry channel.
func (ing *Ingester) drain() {
	defer close(ing.done)
	for msg := range ing.telemetryCh {
		if err := ing.processTelemetry(msg); err != nil {
			slog.Error("process telemetry", "topic", msg.Topic(), "err", err)
		}
	}
}

// Stop unsubscribes and waits for the drain goroutine to finish.
func (ing *Ingester) Stop() {
	ing.client.Unsubscribe("devices/+/telemetry")
	close(ing.telemetryCh)
	<-ing.done
}

func (ing *Ingester) processTelemetry(msg paho.Message) error {
	var payload model.TelemetryPayload
	if err := json.Unmarshal(msg.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal telemetry: %w", err)
	}

	reading := model.SensorReading{
		Time:         time.Unix(payload.Timestamp, 0).UTC(),
		TemperatureC: payload.Sensors.TemperatureC,
		HumidityPct:  payload.Sensors.HumidityPct,
		Eco2Ppm:      payload.Sensors.Eco2Ppm,
		TvocPpb:      payload.Sensors.TvocPpb,
		Pm25Ugm3:     payload.Sensors.Pm25Ugm3,
		Pm10Ugm3:     payload.Sensors.Pm10Ugm3,
	}

	// DeviceID will be resolved from the payload's device_id string
	// For now, defer device lookup to a context with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Parse device_id from payload — stored as string matching device_identifier
	// We store into sensor_readings by UUID; in production the ingester would
	// look up the device UUID from a cache. This is stubbed for Phase 1.
	_ = payload.DeviceID
	_ = ctx
	slog.Debug("telemetry received", "device_id", payload.DeviceID, "timestamp", payload.Timestamp)

	_ = reading
	return nil
}
