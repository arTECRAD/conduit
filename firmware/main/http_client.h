#pragma once
#include "esp_err.h"

typedef struct {
    char device_id_uuid[37];   /* UUID string from server, stored for reference */
    char mqtt_username[32];    /* e.g. "device_a1b2c3d4e5f6" */
    char mqtt_password[50];
    char status[16];           /* "pending" or "active" */
} register_response_t;

/**
 * @brief  POST /api/v1/devices/register with TLS using embedded CA cert.
 *
 * Retries up to 3 times with 5s delay between retries.
 * Returns ESP_ERR_INVALID_RESPONSE on HTTP 409 (already registered).
 * Caller must NOT retry on 409 — call device_config_factory_reset() instead.
 */
esp_err_t http_client_register_device(const char *identifier,
                                       const char *setup_code,
                                       register_response_t *out);
