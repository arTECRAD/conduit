#pragma once
#include "esp_err.h"
#include "driver/i2c_master.h"

#define CCS811_I2C_ADDR_DEFAULT   0x5A   /* ADDR pin low */
#define CCS811_WARMUP_SECONDS     1200   /* 20 minutes */

typedef struct {
    i2c_master_dev_handle_t dev_handle;
    uint32_t warmup_start_s;   /* unix epoch when app_start issued */
} ccs811_handle_t;

typedef struct {
    uint16_t eco2_ppm;
    uint16_t tvoc_ppb;
    bool     valid;     /* false during warmup or on read error */
} ccs811_reading_t;

/**
 * @brief  Initialise CCS811: probe HW_ID, run APP_START, set drive mode 1.
 *
 * @param  sda_io   SDA GPIO
 * @param  scl_io   SCL GPIO
 * @param  out      Populated handle (caller-allocated)
 * @return ESP_OK on success
 */
esp_err_t ccs811_init(int sda_io, int scl_io, ccs811_handle_t *out);

/**
 * @brief  Read ALG_RESULT_DATA register.  Returns valid=false during warmup.
 */
esp_err_t ccs811_read(ccs811_handle_t *h, ccs811_reading_t *out);

/**
 * @brief  Write ENV_DATA compensation (temperature in 0.5°C units + offset 25°C, humidity in 0.5% units).
 */
esp_err_t ccs811_set_env_data(ccs811_handle_t *h, float temp_c, float hum_pct);
