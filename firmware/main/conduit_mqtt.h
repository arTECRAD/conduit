#pragma once
#include "esp_err.h"
#include "freertos/FreeRTOS.h"
#include "freertos/event_groups.h"
#include <stdbool.h>
#include <stddef.h>

#define MQTT_CONNECTED_BIT     BIT0
#define MQTT_DISCONNECTED_BIT  BIT1

typedef void (*mqtt_cmd_cb_t)(const char *payload, size_t len);

/**
 * @brief  Initialise MQTT client with TLS (embedded CA cert).
 * @param  username  Device MQTT username (e.g. "device_a1b2c3d4e5f6")
 * @param  password  Device MQTT password
 * @param  cb        Invoked on command topic message (MQTT task context)
 */
esp_err_t mqtt_client_init(const char *username, const char *password, mqtt_cmd_cb_t cb);

esp_err_t mqtt_client_start(void);
esp_err_t mqtt_client_stop(void);

esp_err_t mqtt_client_publish_telemetry(const char *json, size_t len);
esp_err_t mqtt_client_publish_health(const char *json, size_t len);
esp_err_t mqtt_client_publish_ack(const char *json, size_t len);

/** Block until connected or timeout. Returns ESP_OK or ESP_ERR_TIMEOUT. */
esp_err_t mqtt_client_wait_connected(uint32_t timeout_ms);

bool               mqtt_client_is_connected(void);
EventGroupHandle_t mqtt_client_get_event_group(void);
