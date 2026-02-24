SELECT remove_retention_policy('sensor_readings_5m', if_exists => true);
SELECT remove_retention_policy('sensor_readings',    if_exists => true);
SELECT remove_continuous_aggregate_policy('sensor_readings_1h', if_exists => true);
SELECT remove_continuous_aggregate_policy('sensor_readings_5m', if_exists => true);
DROP MATERIALIZED VIEW IF EXISTS sensor_readings_1h CASCADE;
DROP MATERIALIZED VIEW IF EXISTS sensor_readings_5m CASCADE;
DROP TABLE IF EXISTS sensor_readings CASCADE;
