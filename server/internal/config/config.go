package config

import (
	"time"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all application configuration populated from environment variables.
type Config struct {
	DatabaseURL       string        `envconfig:"DATABASE_URL" required:"true"`
	RedisURL          string        `envconfig:"REDIS_URL" required:"true"`
	MQTTBrokerURL     string        `envconfig:"MQTT_BROKER_URL" required:"true"`
	MQTTUsername      string        `envconfig:"MQTT_USERNAME" required:"true"`
	MQTTPassword      string        `envconfig:"MQTT_PASSWORD" required:"true"`
	JWTSecret         string        `envconfig:"JWT_SECRET" required:"true"`
	ProvisioningToken string        `envconfig:"PROVISIONING_TOKEN" required:"true"`
	ServerPort        int           `envconfig:"SERVER_PORT" default:"8080"`
	CORSOrigin        string        `envconfig:"CORS_ORIGIN" required:"true"`
	TLSCACertPath     string        `envconfig:"TLS_CA_CERT_PATH"`
	TLSServerCertPath string        `envconfig:"TLS_SERVER_CERT_PATH"`
	TLSServerKeyPath  string        `envconfig:"TLS_SERVER_KEY_PATH"`
	RunMigrations     bool          `envconfig:"RUN_MIGRATIONS" default:"false"`
	Env               string        `envconfig:"ENV" default:"development"`
	AccessTokenTTL    time.Duration `envconfig:"ACCESS_TOKEN_TTL" default:"15m"`
	RefreshTokenTTL   time.Duration `envconfig:"REFRESH_TOKEN_TTL" default:"720h"`
	BcryptCost        int           `envconfig:"BCRYPT_COST" default:"12"`
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
