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

SELECT create_hypertable('sensor_readings', by_range('time'));

CREATE INDEX idx_sensor_readings_device_time
    ON sensor_readings (device_id, time DESC);

-- 5-minute continuous aggregate
CREATE MATERIALIZED VIEW sensor_readings_5m
    WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('5 minutes', time) AS bucket,
    device_id,
    AVG(temperature_c)  AS avg_temperature_c, MIN(temperature_c)  AS min_temperature_c, MAX(temperature_c)  AS max_temperature_c,
    AVG(humidity_pct)   AS avg_humidity_pct,  MIN(humidity_pct)   AS min_humidity_pct,  MAX(humidity_pct)   AS max_humidity_pct,
    AVG(eco2_ppm)       AS avg_eco2_ppm,      MIN(eco2_ppm)       AS min_eco2_ppm,      MAX(eco2_ppm)       AS max_eco2_ppm,
    AVG(tvoc_ppb)       AS avg_tvoc_ppb,      MIN(tvoc_ppb)       AS min_tvoc_ppb,      MAX(tvoc_ppb)       AS max_tvoc_ppb,
    AVG(pm25_ugm3)      AS avg_pm25_ugm3,     MIN(pm25_ugm3)      AS min_pm25_ugm3,     MAX(pm25_ugm3)      AS max_pm25_ugm3,
    AVG(pm10_ugm3)      AS avg_pm10_ugm3,     MIN(pm10_ugm3)      AS min_pm10_ugm3,     MAX(pm10_ugm3)      AS max_pm10_ugm3
FROM sensor_readings
GROUP BY bucket, device_id
WITH NO DATA;

SELECT add_continuous_aggregate_policy('sensor_readings_5m',
    start_offset => INTERVAL '1 day',
    end_offset   => INTERVAL '1 minute',
    schedule_interval => INTERVAL '5 minutes');

-- 1-hour continuous aggregate (built on 5m)
CREATE MATERIALIZED VIEW sensor_readings_1h
    WITH (timescaledb.continuous, timescaledb.materialized_only = false) AS
SELECT
    time_bucket('1 hour', bucket) AS bucket,
    device_id,
    AVG(avg_temperature_c)  AS avg_temperature_c, MIN(min_temperature_c)  AS min_temperature_c, MAX(max_temperature_c)  AS max_temperature_c,
    AVG(avg_humidity_pct)   AS avg_humidity_pct,  MIN(min_humidity_pct)   AS min_humidity_pct,  MAX(max_humidity_pct)   AS max_humidity_pct,
    AVG(avg_eco2_ppm)       AS avg_eco2_ppm,      MIN(min_eco2_ppm)       AS min_eco2_ppm,      MAX(max_eco2_ppm)       AS max_eco2_ppm,
    AVG(avg_tvoc_ppb)       AS avg_tvoc_ppb,      MIN(min_tvoc_ppb)       AS min_tvoc_ppb,      MAX(max_tvoc_ppb)       AS max_tvoc_ppb,
    AVG(avg_pm25_ugm3)      AS avg_pm25_ugm3,     MIN(min_pm25_ugm3)      AS min_pm25_ugm3,     MAX(max_pm25_ugm3)      AS max_pm25_ugm3,
    AVG(avg_pm10_ugm3)      AS avg_pm10_ugm3,     MIN(min_pm10_ugm3)      AS min_pm10_ugm3,     MAX(max_pm10_ugm3)      AS max_pm10_ugm3
FROM sensor_readings_5m
GROUP BY time_bucket('1 hour', bucket), device_id
WITH NO DATA;

SELECT add_continuous_aggregate_policy('sensor_readings_1h',
    start_offset => INTERVAL '3 days',
    end_offset   => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour');

SELECT add_retention_policy('sensor_readings',    INTERVAL '30 days');
SELECT add_retention_policy('sensor_readings_5m', INTERVAL '6 months');
