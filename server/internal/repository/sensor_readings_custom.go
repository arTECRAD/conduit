package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/arTECRAD/conduit/server/internal/model"
)

// SensorReadingsCustom provides hand-written aggregation queries not expressible in sqlc.
type SensorReadingsCustom struct {
	pool *pgxpool.Pool
}

// NewSensorReadingsCustom creates a new SensorReadingsCustom repository.
func NewSensorReadingsCustom(pool *pgxpool.Pool) *SensorReadingsCustom {
	return &SensorReadingsCustom{pool: pool}
}

// QueryAggregated returns aggregated sensor readings for the given resolution.
func (r *SensorReadingsCustom) QueryAggregated(ctx context.Context, q model.TelemetryQuery) ([]model.AggregatedReading, error) {
	var table, timeCol string
	switch q.Resolution {
	case model.Resolution5m:
		table, timeCol = "sensor_readings_5m", "bucket"
	case model.Resolution1h:
		table, timeCol = "sensor_readings_1h", "bucket"
	case model.Resolution1d:
		// 1d is queried from the 1h aggregate with a day bucket grouping
		return r.queryDailyAggregated(ctx, q)
	default:
		return nil, fmt.Errorf("unsupported resolution for aggregation: %s", q.Resolution)
	}

	query := fmt.Sprintf(`
		SELECT
			%s AS bucket, device_id,
			avg_temperature_c, min_temperature_c, max_temperature_c,
			avg_humidity_pct,  min_humidity_pct,  max_humidity_pct,
			avg_eco2_ppm,      min_eco2_ppm,      max_eco2_ppm,
			avg_tvoc_ppb,      min_tvoc_ppb,      max_tvoc_ppb,
			avg_pm25_ugm3,     min_pm25_ugm3,     max_pm25_ugm3,
			avg_pm10_ugm3,     min_pm10_ugm3,     max_pm10_ugm3
		FROM %s
		WHERE device_id = $1 AND %s >= $2 AND %s <= $3
		ORDER BY %s ASC
	`, timeCol, table, timeCol, timeCol, timeCol)

	rows, err := r.pool.Query(ctx, query, q.DeviceID, q.From, q.To)
	if err != nil {
		return nil, fmt.Errorf("query aggregated readings: %w", err)
	}
	defer rows.Close()

	return scanAggregatedRows(rows)
}

func (r *SensorReadingsCustom) queryDailyAggregated(ctx context.Context, q model.TelemetryQuery) ([]model.AggregatedReading, error) {
	const query = `
		SELECT
			time_bucket('1 day', bucket) AS bucket,
			device_id,
			AVG(avg_temperature_c)  AS avg_temperature_c, MIN(min_temperature_c)  AS min_temperature_c, MAX(max_temperature_c)  AS max_temperature_c,
			AVG(avg_humidity_pct)   AS avg_humidity_pct,  MIN(min_humidity_pct)   AS min_humidity_pct,  MAX(max_humidity_pct)   AS max_humidity_pct,
			AVG(avg_eco2_ppm)       AS avg_eco2_ppm,      MIN(min_eco2_ppm)       AS min_eco2_ppm,      MAX(max_eco2_ppm)       AS max_eco2_ppm,
			AVG(avg_tvoc_ppb)       AS avg_tvoc_ppb,      MIN(min_tvoc_ppb)       AS min_tvoc_ppb,      MAX(max_tvoc_ppb)       AS max_tvoc_ppb,
			AVG(avg_pm25_ugm3)      AS avg_pm25_ugm3,     MIN(min_pm25_ugm3)      AS min_pm25_ugm3,     MAX(max_pm25_ugm3)      AS max_pm25_ugm3,
			AVG(avg_pm10_ugm3)      AS avg_pm10_ugm3,     MIN(min_pm10_ugm3)      AS min_pm10_ugm3,     MAX(max_pm10_ugm3)      AS max_pm10_ugm3
		FROM sensor_readings_1h
		WHERE device_id = $1 AND bucket >= $2 AND bucket <= $3
		GROUP BY time_bucket('1 day', bucket), device_id
		ORDER BY bucket ASC
	`

	rows, err := r.pool.Query(ctx, query, q.DeviceID, q.From, q.To)
	if err != nil {
		return nil, fmt.Errorf("query daily aggregated readings: %w", err)
	}
	defer rows.Close()

	return scanAggregatedRows(rows)
}

func scanAggregatedRows(rows interface {
	Scan(dest ...interface{}) error
	Next() bool
	Err() error
}) ([]model.AggregatedReading, error) {
	var results []model.AggregatedReading
	for rows.Next() {
		var r model.AggregatedReading
		var deviceID uuid.UUID
		if err := rows.Scan(
			&r.Bucket, &deviceID,
			&r.AvgTemperatureC, &r.MinTemperatureC, &r.MaxTemperatureC,
			&r.AvgHumidityPct, &r.MinHumidityPct, &r.MaxHumidityPct,
			&r.AvgEco2Ppm, &r.MinEco2Ppm, &r.MaxEco2Ppm,
			&r.AvgTvocPpb, &r.MinTvocPpb, &r.MaxTvocPpb,
			&r.AvgPm25Ugm3, &r.MinPm25Ugm3, &r.MaxPm25Ugm3,
			&r.AvgPm10Ugm3, &r.MinPm10Ugm3, &r.MaxPm10Ugm3,
		); err != nil {
			return nil, fmt.Errorf("scan aggregated row: %w", err)
		}
		r.DeviceID = deviceID
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate aggregated rows: %w", err)
	}
	return results, nil
}

// InsertSensorReadingRaw inserts a sensor reading using raw pgx (bypasses sqlc for convenience
// in the MQTT ingester path where the model type is used directly).
func (r *SensorReadingsCustom) InsertSensorReadingRaw(ctx context.Context, reading model.SensorReading) error {
	const query = `
		INSERT INTO sensor_readings (time, device_id, temperature_c, humidity_pct, eco2_ppm, tvoc_ppb, pm25_ugm3, pm10_ugm3)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err := r.pool.Exec(ctx, query,
		reading.Time, reading.DeviceID,
		reading.TemperatureC, reading.HumidityPct,
		reading.Eco2Ppm, reading.TvocPpb,
		reading.Pm25Ugm3, reading.Pm10Ugm3,
	)
	if err != nil {
		return fmt.Errorf("insert sensor reading: %w", err)
	}
	return nil
}

// nullFloat64 is a helper for nullable float64 scanning.
type nullFloat64 struct {
	Float64 float64
	Valid   bool
}

func (n *nullFloat64) toPointer() *float64 {
	if !n.Valid {
		return nil
	}
	return &n.Float64
}

// QueryRawReadings returns raw sensor readings for the given device and time range.
func (r *SensorReadingsCustom) QueryRawReadings(ctx context.Context, deviceID uuid.UUID, from, to time.Time) ([]model.SensorReading, error) {
	const query = `
		SELECT time, device_id, temperature_c, humidity_pct, eco2_ppm, tvoc_ppb, pm25_ugm3, pm10_ugm3
		FROM sensor_readings
		WHERE device_id = $1 AND time >= $2 AND time <= $3
		ORDER BY time ASC
	`
	rows, err := r.pool.Query(ctx, query, deviceID, from, to)
	if err != nil {
		return nil, fmt.Errorf("query raw readings: %w", err)
	}
	defer rows.Close()

	var results []model.SensorReading
	for rows.Next() {
		var sr model.SensorReading
		var tc, hp, ep, tp, p25, p10 nullFloat64
		if err := rows.Scan(&sr.Time, &sr.DeviceID, &tc, &hp, &ep, &tp, &p25, &p10); err != nil {
			return nil, fmt.Errorf("scan raw reading: %w", err)
		}
		sr.TemperatureC = tc.toPointer()
		sr.HumidityPct = hp.toPointer()
		sr.Eco2Ppm = ep.toPointer()
		sr.TvocPpb = tp.toPointer()
		sr.Pm25Ugm3 = p25.toPointer()
		sr.Pm10Ugm3 = p10.toPointer()
		results = append(results, sr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate raw readings: %w", err)
	}
	return results, nil
}
