/*
 * Conduit ESP32-S3 firmware — main state machine
 *
 * Boot sequence:
 *   NVS init → device identity → LED task → WiFi init
 *   → if no WiFi config: AP + captive portal → reboot
 *   → WiFi station connect → SNTP (best-effort)
 *   → if unregistered: HTTPS device registration
 *   → MQTT connect → subscribe commands → command handler task
 *   → if not yet claimed: LED purple breathe + health heartbeat → wait ACTIVATE_BIT
 *   → LED green → sensor task (Core 1) → vTaskDelete
 */
#include <string.h>
#include <time.h>
#include "esp_log.h"
#include "esp_system.h"
#include "esp_sntp.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/event_groups.h"

#include "device_config.h"
#include "wifi_manager.h"
#include "provisioning.h"
#include "http_client.h"
#include "conduit_mqtt.h"
#include "command_handler.h"
#include "sensor_task.h"
#include "telemetry.h"
#include "led_status.h"

static const char *TAG = "main";

static EventGroupHandle_t s_app_events = NULL;

/* MQTT command callback: forward payload to command_handler queue */
static void on_mqtt_command(const char *payload, size_t len)
{
    command_handler_enqueue(payload, len);
}

/* Best-effort SNTP sync; non-fatal if it times out */
static void sntp_sync_wait(uint32_t timeout_ms)
{
    esp_sntp_setoperatingmode(SNTP_OPMODE_POLL);
    esp_sntp_setservername(0, "pool.ntp.org");
    esp_sntp_setservername(1, "time.google.com");
    esp_sntp_init();

    uint32_t waited = 0;
    while (sntp_get_sync_status() != SNTP_SYNC_STATUS_COMPLETED && waited < timeout_ms) {
        vTaskDelay(pdMS_TO_TICKS(500));
        waited += 500;
    }
    if (sntp_get_sync_status() == SNTP_SYNC_STATUS_COMPLETED) {
        time_t now = time(NULL);
        ESP_LOGI(TAG, "SNTP synced: epoch=%lld", (long long)now);
    } else {
        ESP_LOGW(TAG, "SNTP sync timeout — timestamps will be omitted from telemetry");
    }
}

void app_main(void)
{
    esp_err_t err;

    /* ── 1. NVS init + device identity ─────────────────────────────── */
    ESP_ERROR_CHECK(device_config_init());
    device_config_t cfg;
    device_config_load(&cfg);
    ESP_LOGI(TAG, "Conduit v%s | ID: %s | Setup: %s",
             "1.0.0", cfg.device_identifier, cfg.setup_code);

    /* ── 2. LED status task (Core 0, prio 2) ───────────────────────── */
    err = led_status_init();
    if (err != ESP_OK) {
        ESP_LOGW(TAG, "LED init failed (non-fatal): %s", esp_err_to_name(err));
    }
    xTaskCreatePinnedToCore(led_status_task, "led", 2048, NULL, 2, NULL, 0);
    led_status_set(LED_BOOTING);

    /* ── 3. WiFi init ──────────────────────────────────────────────── */
    ESP_ERROR_CHECK(wifi_manager_init());

    /* ── 4. AP mode if no WiFi credentials ─────────────────────────── */
    if (cfg.wifi_ssid[0] == '\0') {
        ESP_LOGI(TAG, "No WiFi config — starting captive portal (SSID: CDT-%s)",
                 cfg.setup_code);
        led_status_set(LED_AP_MODE);
        ESP_ERROR_CHECK(wifi_manager_start_ap(cfg.setup_code));
        ESP_ERROR_CHECK(provisioning_start());

        /* Block until user submits WiFi credentials */
        provisioning_wait(portMAX_DELAY);
        ESP_LOGI(TAG, "WiFi credentials received — rebooting");
        vTaskDelay(pdMS_TO_TICKS(1000));
        esp_restart();
        return;
    }

    /* ── 5. WiFi station connect ────────────────────────────────────── */
    led_status_set(LED_CONNECTING);
    err = wifi_manager_connect_station();
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "WiFi connection failed — rebooting in 5s");
        led_status_set(LED_ERROR);
        vTaskDelay(pdMS_TO_TICKS(5000));
        esp_restart();
        return;
    }
    ESP_LOGI(TAG, "WiFi connected");

    /* ── 6. SNTP (best-effort, 10s) ─────────────────────────────────── */
    sntp_sync_wait(10000);

    /* ── 7. Device registration (if not already done) ───────────────── */
    if (!cfg.registered) {
        led_status_set(LED_REGISTERING);
        ESP_LOGI(TAG, "Registering device with server...");

        register_response_t reg = {0};
        err = http_client_register_device(cfg.device_identifier, cfg.setup_code, &reg);

        if (err == ESP_ERR_INVALID_RESPONSE) {
            /* HTTP 409: identity conflict — factory reset and re-enter AP mode */
            ESP_LOGE(TAG, "Server returned 409 (identity conflict) — factory reset");
            led_status_set(LED_ERROR);
            vTaskDelay(pdMS_TO_TICKS(1000));
            device_config_factory_reset();  /* does not return */
            return;
        }
        if (err != ESP_OK) {
            ESP_LOGE(TAG, "Registration failed: %s — rebooting in 5s",
                     esp_err_to_name(err));
            led_status_set(LED_ERROR);
            vTaskDelay(pdMS_TO_TICKS(5000));
            esp_restart();
            return;
        }

        device_config_set_mqtt_credentials(reg.mqtt_username, reg.mqtt_password);
        device_config_set_registered(true);
        device_config_load(&cfg);  /* reload to pick up mqtt_user/pass */
        ESP_LOGI(TAG, "Registered: status=%s  mqtt_user=%s",
                 reg.status, reg.mqtt_username);
    }

    /* ── 8. MQTT connect ────────────────────────────────────────────── */
    s_app_events = xEventGroupCreate();

    ESP_ERROR_CHECK(mqtt_client_init(cfg.mqtt_user, cfg.mqtt_pass, on_mqtt_command));
    ESP_ERROR_CHECK(mqtt_client_start());

    err = mqtt_client_wait_connected(30000);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "MQTT connect timeout — rebooting in 5s");
        led_status_set(LED_ERROR);
        vTaskDelay(pdMS_TO_TICKS(5000));
        esp_restart();
        return;
    }
    ESP_LOGI(TAG, "MQTT connected as %s", cfg.mqtt_user);

    /* ── 9. Command handler task (Core 0, prio 6) ───────────────────── */
    ESP_ERROR_CHECK(command_handler_init(s_app_events));
    xTaskCreatePinnedToCore(command_handler_task, "cmd_hdlr", 4096,
                            NULL, 6, NULL, 0);

    /* ── 10. Wait for activation if device not yet claimed ──────────── */
    if (!cfg.provisioned) {
        led_status_set(LED_PENDING_CLAIM);
        ESP_LOGI(TAG, "Pending claim — publish health every 30s, waiting for activate");

        static char hlt_buf[256];
        sensor_reading_t empty = {.warming_up = true};

        for (;;) {
            if (telemetry_build_health_json(&empty, cfg.device_identifier,
                                            hlt_buf, sizeof(hlt_buf)) == ESP_OK) {
                mqtt_client_publish_health(hlt_buf, strlen(hlt_buf));
            }
            EventBits_t bits = xEventGroupWaitBits(s_app_events, ACTIVATE_BIT,
                                                   pdFALSE, pdFALSE,
                                                   pdMS_TO_TICKS(30000));
            if (bits & ACTIVATE_BIT) {
                ESP_LOGI(TAG, "Activation command received");
                break;
            }
        }
        device_config_load(&cfg);  /* reload provisioned=true */
    }

    /* ── 11. Start sensor task (Core 1, prio 5) ─────────────────────── */
    led_status_set(LED_ACTIVE);
    sensor_task_enable_publish();

    xTaskCreatePinnedToCore(sensor_task, "sensor", 8192, NULL, 5, NULL, 1);

    ESP_LOGI(TAG, "Startup complete — entering sensor loop");
    vTaskDelete(NULL);
}
