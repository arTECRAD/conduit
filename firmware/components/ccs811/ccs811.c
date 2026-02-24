#include "ccs811.h"
#include <string.h>
#include "esp_log.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "esp_timer.h"

static const char *TAG = "ccs811";

/* Register addresses */
#define REG_STATUS          0x00
#define REG_MEAS_MODE       0x01
#define REG_ALG_RESULT_DATA 0x02
#define REG_ENV_DATA        0x05
#define REG_HW_ID           0x20
#define REG_APP_START       0xF4
#define REG_SW_RESET        0xFF

#define CCS811_HW_ID_CODE   0x81
#define DRIVE_MODE_1        (1 << 4)   /* measure every 1s */

#define I2C_TIMEOUT_MS      100

static esp_err_t ccs811_write_reg(ccs811_handle_t *h, uint8_t reg, const uint8_t *data, size_t len)
{
    uint8_t buf[9];
    buf[0] = reg;
    if (len > 0 && data) {
        memcpy(&buf[1], data, len);
    }
    return i2c_master_transmit(h->dev_handle, buf, 1 + len, I2C_TIMEOUT_MS);
}

static esp_err_t ccs811_read_reg(ccs811_handle_t *h, uint8_t reg, uint8_t *out, size_t len)
{
    esp_err_t err = i2c_master_transmit(h->dev_handle, &reg, 1, I2C_TIMEOUT_MS);
    if (err != ESP_OK) return err;
    return i2c_master_receive(h->dev_handle, out, len, I2C_TIMEOUT_MS);
}

esp_err_t ccs811_init(int sda_io, int scl_io, ccs811_handle_t *out)
{
    i2c_master_bus_config_t bus_cfg = {
        .i2c_port = I2C_NUM_0,
        .sda_io_num = sda_io,
        .scl_io_num = scl_io,
        .clk_source = I2C_CLK_SRC_DEFAULT,
        .glitch_ignore_cnt = 7,
        .flags.enable_internal_pullup = true,
    };
    i2c_master_bus_handle_t bus_handle;
    esp_err_t err = i2c_new_master_bus(&bus_cfg, &bus_handle);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "i2c_new_master_bus failed: %s", esp_err_to_name(err));
        return err;
    }

    i2c_device_config_t dev_cfg = {
        .dev_addr_length = I2C_ADDR_BIT_LEN_7,
        .device_address = CCS811_I2C_ADDR_DEFAULT,
        .scl_speed_hz = 100000,
    };
    err = i2c_master_bus_add_device(bus_handle, &dev_cfg, &out->dev_handle);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "i2c_master_bus_add_device failed: %s", esp_err_to_name(err));
        return err;
    }

    /* Check HW ID */
    uint8_t hw_id = 0;
    err = ccs811_read_reg(out, REG_HW_ID, &hw_id, 1);
    if (err != ESP_OK || hw_id != CCS811_HW_ID_CODE) {
        ESP_LOGE(TAG, "HW_ID mismatch: 0x%02x (expected 0x%02x)", hw_id, CCS811_HW_ID_CODE);
        return ESP_ERR_NOT_FOUND;
    }

    /* APP_START — transitions from boot mode to app mode */
    err = ccs811_write_reg(out, REG_APP_START, NULL, 0);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "APP_START failed: %s", esp_err_to_name(err));
        return err;
    }
    vTaskDelay(pdMS_TO_TICKS(100));

    /* Set drive mode 1 (measure every 1s) */
    uint8_t mode = DRIVE_MODE_1;
    err = ccs811_write_reg(out, REG_MEAS_MODE, &mode, 1);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "set drive mode failed: %s", esp_err_to_name(err));
        return err;
    }

    out->warmup_start_s = (uint32_t)(esp_timer_get_time() / 1000000);
    ESP_LOGI(TAG, "CCS811 initialised (warmup %d min)", CCS811_WARMUP_SECONDS / 60);
    return ESP_OK;
}

esp_err_t ccs811_read(ccs811_handle_t *h, ccs811_reading_t *out)
{
    out->valid = false;
    uint8_t data[4] = {0};
    esp_err_t err = ccs811_read_reg(h, REG_ALG_RESULT_DATA, data, sizeof(data));
    if (err != ESP_OK) {
        ESP_LOGW(TAG, "read ALG_RESULT_DATA failed: %s", esp_err_to_name(err));
        return err;
    }
    out->eco2_ppm = ((uint16_t)data[0] << 8) | data[1];
    out->tvoc_ppb = ((uint16_t)data[2] << 8) | data[3];

    uint32_t now_s = (uint32_t)(esp_timer_get_time() / 1000000);
    if ((now_s - h->warmup_start_s) < CCS811_WARMUP_SECONDS) {
        /* Still warming up — data is unreliable */
        out->valid = false;
    } else {
        out->valid = true;
    }
    return ESP_OK;
}

esp_err_t ccs811_set_env_data(ccs811_handle_t *h, float temp_c, float hum_pct)
{
    /* Humidity: 16-bit fixed point, 1/512 % per LSB.
       Temperature: 16-bit fixed point offset by 25°C, 1/512°C per LSB. */
    uint16_t hum_raw  = (uint16_t)(hum_pct * 512.0f);
    uint16_t temp_raw = (uint16_t)((temp_c + 25.0f) * 512.0f);
    uint8_t env[4] = {
        (uint8_t)(hum_raw >> 8),  (uint8_t)(hum_raw & 0xFF),
        (uint8_t)(temp_raw >> 8), (uint8_t)(temp_raw & 0xFF),
    };
    return ccs811_write_reg(h, REG_ENV_DATA, env, sizeof(env));
}
