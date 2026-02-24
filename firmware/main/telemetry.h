#pragma once
#include "esp_err.h"
#include "sensor_task.h"
#include <stddef.h>
#include <stdbool.h>

/**
 * @brief  Build telemetry JSON. CCS811 fields are omitted when warming_up=true.
 *         timestamp field is omitted when ts_unix==0 (SNTP not synced).
 *
 * @param r    Sensor reading
 * @param id   device_identifier string (e.g. "a1b2c3d4e5f6")
 * @param buf  Caller-allocated buffer (>= 512 bytes recommended)
 * @param len  Buffer length
 */
esp_err_t telemetry_build_json(const sensor_reading_t *r, const char *id,
                                char *buf, size_t len);

/**
 * @brief  Build health/heartbeat JSON.
 *
 * @param r    Most recent sensor reading
 * @param id   device_identifier string
 * @param buf  Caller-allocated buffer (>= 256 bytes recommended)
 * @param len  Buffer length
 */
esp_err_t telemetry_build_health_json(const sensor_reading_t *r, const char *id,
                                       char *buf, size_t len);

/**
 * @brief  Build command ACK JSON matching model.CommandAck on the server.
 *
 * @param cmd_id   UUID string of the command
 * @param ok       true = success, false = error
 * @param err_msg  Error description (may be NULL)
 * @param buf      Caller-allocated buffer (>= 256 bytes recommended)
 * @param len      Buffer length
 */
esp_err_t telemetry_build_ack_json(const char *cmd_id, bool ok,
                                    const char *err_msg, char *buf, size_t len);
