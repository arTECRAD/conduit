#include "sensor_task.h"
#include "device_config.h"
#include "telemetry.h"
#include "mqtt_client.h"
#include <string.h>
#include <time.h>
#include <inttypes.h>
#include "esp_log.h"
#include "driver/uart.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "ccs811.h"
#include "dht22.h"
#include "sds011.h"

static const char *TAG = "sensor_task";

static volatile uint32_t s_poll_interval_ms = CONFIG_CONDUIT_DEFAULT_POLL_INTERVAL_MS;
static volatile bool     s_publish_enabled  = false;
static TaskHandle_t      s_task_handle      = NULL;

/* Static JSON buffers — never allocated on task stack */
static char s_tel_buf[512];
static char s_hlt_buf[256];

void sensor_task_notify_poll_interval(uint32_t ms)
{
    s_poll_interval_ms = ms;
    if (s_task_handle) {
        xTaskNotify(s_task_handle, ms, eSetValueWithOverwrite);
    }
}

void sensor_task_enable_publish(void)
{
    s_publish_enabled = true;
}

void sensor_task(void *pv)
{
    s_task_handle = xTaskGetCurrentTaskHandle();

    device_config_t cfg;
    device_config_load(&cfg);
    s_poll_interval_ms = cfg.poll_interval_ms;

    /* Initialise CCS811 */
    ccs811_handle_t ccs = {0};
    bool ccs_ok = (ccs811_init(CONFIG_CONDUIT_CCS811_I2C_SDA,
                               CONFIG_CONDUIT_CCS811_I2C_SCL, &ccs) == ESP_OK);
    if (!ccs_ok) ESP_LOGE(TAG, "CCS811 init failed — no eCO2/TVOC data");

    /* Initialise SDS011 */
    sds011_config_t sds_cfg = {
        .uart_num = UART_NUM_1,
        .rx_gpio  = CONFIG_CONDUIT_SDS011_UART_RX,
        .tx_gpio  = CONFIG_CONDUIT_SDS011_UART_TX,
    };
    bool sds_ok = (sds011_init(&sds_cfg) == ESP_OK);
    if (!sds_ok) ESP_LOGE(TAG, "SDS011 init failed — no PM data");

    ESP_LOGI(TAG, "Sensor task running (interval=%" PRIu32 " ms)", s_poll_interval_ms);

    for (;;) {
        uint32_t interval = s_poll_interval_ms;
        sensor_reading_t reading = {0};
        reading.warming_up = !ccs_ok;  /* also true if CCS811 not yet warmed up */

        /* Current timestamp */
        time_t now = time(NULL);
        reading.ts_unix = (now > 1700000000LL) ? (int64_t)now : 0;

        /* 1. Wake SDS011 fan (30s spin-up starts now) */
        if (sds_ok) sds011_wake();

        /* 2. Read DHT22 (concurrent with SDS011 spin-up) */
        dht22_reading_t dht = {0};
        esp_err_t dht_err = dht22_read(CONFIG_CONDUIT_DHT22_GPIO, &dht);
        if (dht_err == ESP_OK) {
            reading.temp_c  = dht.temperature_c;
            reading.hum_pct = dht.humidity_pct;
            ESP_LOGI(TAG, "DHT22: %.1f°C  %.1f%%RH", reading.temp_c, reading.hum_pct);
        } else {
            ESP_LOGW(TAG, "DHT22 read failed: %s", esp_err_to_name(dht_err));
        }

        /* 3. Read CCS811 (concurrent with SDS011 spin-up) */
        if (ccs_ok) {
            if (dht_err == ESP_OK) {
                ccs811_set_env_data(&ccs, dht.temperature_c, dht.humidity_pct);
            }
            ccs811_reading_t ccs_r = {0};
            if (ccs811_read(&ccs, &ccs_r) == ESP_OK) {
                reading.warming_up = !ccs_r.valid;
                if (ccs_r.valid) {
                    reading.eco2 = ccs_r.eco2_ppm;
                    reading.tvoc = ccs_r.tvoc_ppb;
                    ESP_LOGI(TAG, "CCS811: eCO2=%u ppm  TVOC=%u ppb",
                             reading.eco2, reading.tvoc);
                } else {
                    ESP_LOGD(TAG, "CCS811 still warming up");
                }
            }
        }

        /* 4. Wait remainder of SDS011 30s spin-up, then read */
        if (sds_ok) {
            vTaskDelay(pdMS_TO_TICKS(SDS011_SPINUP_MS));
            sds011_reading_t sds_r = {0};
            if (sds011_read(&sds_r) == ESP_OK) {
                reading.pm25 = sds_r.pm25_ugm3;
                reading.pm10 = sds_r.pm10_ugm3;
                ESP_LOGI(TAG, "SDS011: PM2.5=%.1f  PM10=%.1f µg/m³",
                         reading.pm25, reading.pm10);
            }
            sds011_sleep();
        }

        /* 5. Publish telemetry (only if activated) */
        if (s_publish_enabled) {
            if (telemetry_build_json(&reading, cfg.device_identifier,
                                     s_tel_buf, sizeof(s_tel_buf)) == ESP_OK) {
                mqtt_client_publish_telemetry(s_tel_buf, strlen(s_tel_buf));
            }
        }

        /* 6. Always publish health heartbeat */
        if (telemetry_build_health_json(&reading, cfg.device_identifier,
                                        s_hlt_buf, sizeof(s_hlt_buf)) == ESP_OK) {
            mqtt_client_publish_health(s_hlt_buf, strlen(s_hlt_buf));
        }

        /* 7. Sleep for remainder of poll interval, or wake early on interval change */
        uint32_t elapsed_ms = SDS011_SPINUP_MS + 2000;  /* approx sensor read overhead */
        uint32_t remaining  = (interval > elapsed_ms) ? (interval - elapsed_ms) : 0;
        if (remaining > 0) {
            uint32_t new_interval = 0;
            if (xTaskNotifyWait(0, 0xFFFFFFFF, &new_interval,
                                pdMS_TO_TICKS(remaining)) == pdTRUE) {
                s_poll_interval_ms = new_interval;
                ESP_LOGI(TAG, "Poll interval changed to %" PRIu32 " ms", new_interval);
            }
        }
    }
}
