-- sqlc schema: clean DDL only, no TimescaleDB-specific syntax.
-- Actual migrations (with TimescaleDB DDL) live in migrations/.

CREATE TYPE device_status AS ENUM ('pending', 'active', 'disabled');

CREATE TABLE users (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL UNIQUE,
    password_hash TEXT        NOT NULL,
    display_name  TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

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

-- sensor_readings appears as plain table; sqlc does not need hypertable info.
CREATE TABLE sensor_readings (
    time          TIMESTAMPTZ      NOT NULL,
    device_id     UUID             NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    temperature_c DOUBLE PRECISION,
    humidity_pct  DOUBLE PRECISION,
    eco2_ppm      DOUBLE PRECISION,
    tvoc_ppb      DOUBLE PRECISION,
    pm25_ugm3     DOUBLE PRECISION,
    pm10_ugm3     DOUBLE PRECISION
);

CREATE TABLE refresh_tokens (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
