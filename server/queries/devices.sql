-- name: CreateDevice :one
INSERT INTO devices (device_identifier, setup_code, mqtt_username, mqtt_password_hash)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetDeviceByID :one
SELECT * FROM devices
WHERE id = $1;

-- name: GetDeviceByIdentifier :one
SELECT * FROM devices
WHERE device_identifier = $1;

-- name: GetDeviceBySetupCode :one
SELECT * FROM devices
WHERE setup_code = $1;

-- name: GetDeviceByMQTTUsername :one
SELECT * FROM devices
WHERE mqtt_username = $1;

-- name: ListDevicesByUserID :many
SELECT * FROM devices
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: ClaimDevice :one
UPDATE devices
SET user_id    = $2,
    status     = 'active',
    claimed_at = now(),
    updated_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: UpdateDevice :one
UPDATE devices
SET display_name     = $2,
    poll_interval_ms = $3,
    updated_at       = now()
WHERE id = $1 AND user_id = $4
RETURNING *;

-- name: UpdateDeviceStatus :one
UPDATE devices
SET status     = $2,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateDeviceMQTTCredentials :one
UPDATE devices
SET mqtt_username      = $2,
    mqtt_password_hash = $3,
    updated_at         = now()
WHERE id = $1
RETURNING *;

-- name: DeleteDevice :exec
DELETE FROM devices
WHERE id = $1 AND user_id = $2;
