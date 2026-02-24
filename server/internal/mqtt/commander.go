package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"

	"github.com/arTECRAD/conduit/server/internal/model"
)

const commandQoS = 1

// Commander publishes MQTT commands to devices.
type Commander struct {
	client paho.Client
}

// NewCommander creates a new Commander.
func NewCommander(client paho.Client) *Commander {
	return &Commander{client: client}
}

// Publish sends a command to the device's command topic.
func (c *Commander) Publish(ctx context.Context, deviceID uuid.UUID, cmd model.CommandPayload) error {
	topic := fmt.Sprintf("devices/%s/command", deviceID.String())

	payload, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("marshal command: %w", err)
	}

	token := c.client.Publish(topic, commandQoS, false, payload)

	// Respect context deadline/cancellation
	select {
	case <-token.Done():
		if err := token.Error(); err != nil {
			return fmt.Errorf("publish command: %w", err)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("publish command cancelled: %w", ctx.Err())
	case <-time.After(5 * time.Second):
		return fmt.Errorf("publish command timed out")
	}
}
