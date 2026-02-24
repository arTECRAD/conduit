CREATE TYPE device_status AS ENUM ('pending', 'active', 'disabled');

CREATE TABLE devices (
    id                 UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID          REFERENCES users(id) ON DELETE SET NULL,
    device_identifier  TEXT          NOT NULL UNIQUE,
    setup_code         TEXT          NOT NULL UNIQUE,
    display_name       TEXT          NOT NULL DEFAULT '',
    status             device_status NOT NULL DEFAULT 'pending',
    mqtt_username      TEXT          NOT NULL UNIQUE,
    mqtt_password_hash TEXT          NOT NULL,
    poll_interval_ms   BIGINT        NOT NULL DEFAULT 30000,
    registered_at      TIMESTAMPTZ   NOT NULL DEFAULT now(),
    claimed_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX idx_devices_user_id           ON devices (user_id);
CREATE INDEX idx_devices_device_identifier ON devices (device_identifier);
CREATE INDEX idx_devices_setup_code        ON devices (setup_code);
CREATE INDEX idx_devices_mqtt_username     ON devices (mqtt_username);
CREATE INDEX idx_devices_status            ON devices (status);
