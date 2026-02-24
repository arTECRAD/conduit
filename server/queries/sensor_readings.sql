-- name: InsertSensorReading :exec
INSERT INTO sensor_readings (time, device_id, temperature_c, humidity_pct, eco2_ppm, tvoc_ppb, pm25_ugm3, pm10_ugm3)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: GetRawSensorReadings :many
SELECT * FROM sensor_readings
WHERE device_id = $1
  AND time >= $2
  AND time <= $3
ORDER BY time ASC;

-- name: GetLatestSensorReading :one
SELECT * FROM sensor_readings
WHERE device_id = $1
ORDER BY time DESC
LIMIT 1;
