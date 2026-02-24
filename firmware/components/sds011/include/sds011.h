#pragma once
#include "esp_err.h"
#include <stdint.h>

#define SDS011_FRAME_LEN   10
#define SDS011_BAUD_RATE   9600
#define SDS011_SPINUP_MS   30000   /* 30s fan spin-up before reading */

typedef struct {
    float pm25_ugm3;
    float pm10_ugm3;
} sds011_reading_t;

typedef struct {
    int uart_num;
    int rx_gpio;
    int tx_gpio;
} sds011_config_t;

/**
 * @brief  Initialise SDS011 on the given UART.
 */
esp_err_t sds011_init(const sds011_config_t *cfg);

/**
 * @brief  Wake the sensor (start fan motor) by sending wake command.
 *         Caller must wait SDS011_SPINUP_MS before calling sds011_read().
 */
esp_err_t sds011_wake(void);

/**
 * @brief  Read one PM2.5/PM10 sample.  Call after spin-up delay.
 *         Reads with a 5-second timeout.
 */
esp_err_t sds011_read(sds011_reading_t *out);

/**
 * @brief  Put sensor to sleep (stop fan motor to extend sensor life).
 */
esp_err_t sds011_sleep(void);
