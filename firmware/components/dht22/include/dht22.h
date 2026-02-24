#pragma once
#include "esp_err.h"

typedef struct {
    float temperature_c;
    float humidity_pct;
} dht22_reading_t;

/**
 * @brief  Read temperature and humidity from DHT22 using GPIO bit-bang.
 *
 * Must NOT be called from a task pinned to core 0 unless WiFi is not active,
 * as this briefly disables interrupts. Prefer core 1.
 *
 * @param  gpio_num  GPIO connected to DHT22 data pin
 * @param  out       Populated on success
 * @return ESP_OK on success, ESP_ERR_TIMEOUT or ESP_ERR_INVALID_CRC on error
 */
esp_err_t dht22_read(int gpio_num, dht22_reading_t *out);
