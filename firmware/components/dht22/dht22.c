/*
 * DHT22 / AM2302 GPIO bit-bang driver for ESP-IDF v5.x
 * Runs in critical section to guarantee timing.
 */
#include "dht22.h"
#include <string.h>
#include "driver/gpio.h"
#include "esp_log.h"
#include "esp_timer.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "rom/ets_sys.h"

static const char *TAG = "dht22";

/* Timing thresholds in microseconds */
#define DHT_START_SIGNAL_US   18000   /* host pulls low for 18ms */
#define DHT_RESPONSE_WAIT_US  40
#define DHT_BIT_HIGH_THRESH   50      /* >50us high = '1', else '0' */
#define DHT_BIT_TIMEOUT_US    100

static int64_t pulse_duration_us(int gpio, int level, int64_t timeout_us)
{
    int64_t start = esp_timer_get_time();
    while (gpio_get_level(gpio) == level) {
        if ((esp_timer_get_time() - start) > timeout_us) {
            return -1;
        }
    }
    return esp_timer_get_time() - start;
}

esp_err_t dht22_read(int gpio_num, dht22_reading_t *out)
{
    uint8_t data[5] = {0};

    /* Configure as output, pull low for start signal */
    gpio_set_direction(gpio_num, GPIO_MODE_OUTPUT);
    gpio_set_level(gpio_num, 0);
    ets_delay_us(DHT_START_SIGNAL_US);

    /* Pull high briefly then switch to input */
    gpio_set_level(gpio_num, 1);
    ets_delay_us(DHT_RESPONSE_WAIT_US);
    gpio_set_direction(gpio_num, GPIO_MODE_INPUT);
    gpio_set_pull_mode(gpio_num, GPIO_PULLUP_ONLY);

    /* Enter critical section for timing-sensitive receive */
    portENTER_CRITICAL_SAFE(NULL);   /* disable FreeRTOS scheduler on this core */

    /* Wait for sensor response: low ~80us, high ~80us */
    if (pulse_duration_us(gpio_num, 1, 100) < 0) goto timeout;   /* wait for low */
    if (pulse_duration_us(gpio_num, 0, 100) < 0) goto timeout;   /* wait for high */
    if (pulse_duration_us(gpio_num, 1, 100) < 0) goto timeout;   /* wait for data start */

    /* Read 40 bits */
    for (int i = 0; i < 40; i++) {
        /* Each bit starts with ~50us low pulse */
        if (pulse_duration_us(gpio_num, 0, 100) < 0) goto timeout;
        /* High pulse duration determines bit value */
        int64_t high_us = pulse_duration_us(gpio_num, 1, DHT_BIT_TIMEOUT_US);
        if (high_us < 0) goto timeout;
        data[i / 8] <<= 1;
        if (high_us > DHT_BIT_HIGH_THRESH) {
            data[i / 8] |= 1;
        }
    }

    portEXIT_CRITICAL_SAFE(NULL);

    /* Verify checksum */
    uint8_t checksum = (uint8_t)(data[0] + data[1] + data[2] + data[3]);
    if (checksum != data[4]) {
        ESP_LOGW(TAG, "checksum fail: calc=0x%02x recv=0x%02x", checksum, data[4]);
        return ESP_ERR_INVALID_CRC;
    }

    /* Decode: humidity = (byte0<<8|byte1)/10.0, temp = (byte2&0x7F<<8|byte3)/10.0 */
    out->humidity_pct    = (float)(((uint16_t)data[0] << 8) | data[1]) / 10.0f;
    float temp           = (float)((((uint16_t)(data[2] & 0x7F)) << 8) | data[3]) / 10.0f;
    out->temperature_c   = (data[2] & 0x80) ? -temp : temp;

    return ESP_OK;

timeout:
    portEXIT_CRITICAL_SAFE(NULL);
    ESP_LOGW(TAG, "timeout reading DHT22");
    return ESP_ERR_TIMEOUT;
}
