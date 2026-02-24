package mqtt

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"os"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/google/uuid"
)

// NewClient creates and connects a paho MQTT client.
func NewClient(brokerURL, username, password, caCertPath string) (paho.Client, error) {
	opts := paho.NewClientOptions()
	opts.AddBroker(brokerURL)
	opts.SetClientID("conduit-server-" + uuid.New().String())
	opts.SetUsername(username)
	opts.SetPassword(password)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(false)
	opts.SetConnectTimeout(10 * time.Second)
	opts.SetKeepAlive(30 * time.Second)
	opts.SetCleanSession(true)

	if caCertPath != "" {
		tlsCfg, err := newTLSConfig(caCertPath)
		if err != nil {
			return nil, fmt.Errorf("build TLS config: %w", err)
		}
		opts.SetTLSConfig(tlsCfg)
	}

	opts.SetOnConnectHandler(func(_ paho.Client) {
		slog.Info("MQTT connected")
	})
	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		slog.Warn("MQTT connection lost, reconnecting", "err", err)
	})

	client := paho.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("MQTT connect: %w", err)
	}

	return client, nil
}

func newTLSConfig(caCertPath string) (*tls.Config, error) {
	caCert, err := os.ReadFile(caCertPath)
	if err != nil {
		return nil, fmt.Errorf("read CA cert: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA cert")
	}
	return &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, nil
}
