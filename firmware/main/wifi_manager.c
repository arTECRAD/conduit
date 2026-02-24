#include "wifi_manager.h"
#include "device_config.h"
#include <string.h>
#include "esp_log.h"
#include "esp_wifi.h"
#include "esp_netif.h"
#include "esp_event.h"
#include "nvs.h"
#include "freertos/event_groups.h"

static const char *TAG = "wifi_manager";

static EventGroupHandle_t s_wifi_event_group;
static int                s_retry_count = 0;
static bool               s_connected   = false;

static void wifi_event_handler(void *arg, esp_event_base_t base,
                                int32_t id, void *event_data)
{
    if (base == WIFI_EVENT && id == WIFI_EVENT_STA_DISCONNECTED) {
        wifi_event_sta_disconnected_t *disc =
            (wifi_event_sta_disconnected_t *)event_data;
        ESP_LOGW(TAG, "Disconnected: reason=%d (0x%x)", disc->reason, disc->reason);
        s_connected = false;
        if (s_retry_count < WIFI_MAX_RETRY) {
            esp_wifi_connect();
            s_retry_count++;
            ESP_LOGI(TAG, "retry WiFi (%d/%d)", s_retry_count, WIFI_MAX_RETRY);
        } else {
            xEventGroupSetBits(s_wifi_event_group, WIFI_FAIL_BIT);
            ESP_LOGW(TAG, "WiFi connection failed after %d retries", WIFI_MAX_RETRY);
        }
    } else if (base == IP_EVENT && id == IP_EVENT_STA_GOT_IP) {
        ip_event_got_ip_t *event = (ip_event_got_ip_t *)event_data;
        ESP_LOGI(TAG, "Got IP: " IPSTR, IP2STR(&event->ip_info.ip));
        s_retry_count = 0;
        s_connected   = true;
        xEventGroupSetBits(s_wifi_event_group, WIFI_CONNECTED_BIT);
    }
}

esp_err_t wifi_manager_init(void)
{
    s_wifi_event_group = xEventGroupCreate();
    esp_netif_init();
    esp_event_loop_create_default();
    esp_netif_create_default_wifi_sta();

    /* Erase the WiFi driver's internal NVS namespace to prevent stale
     * PMKSA cache from causing auth failures on reboot */
    nvs_handle_t wifi_nvs;
    if (nvs_open("nvs.net80211", NVS_READWRITE, &wifi_nvs) == ESP_OK) {
        nvs_erase_all(wifi_nvs);
        nvs_commit(wifi_nvs);
        nvs_close(wifi_nvs);
    }

    wifi_init_config_t cfg = WIFI_INIT_CONFIG_DEFAULT();
    esp_err_t err = esp_wifi_init(&cfg);
    if (err != ESP_OK) return err;

    esp_event_handler_instance_register(WIFI_EVENT, ESP_EVENT_ANY_ID,
                                        &wifi_event_handler, NULL, NULL);
    esp_event_handler_instance_register(IP_EVENT, IP_EVENT_STA_GOT_IP,
                                        &wifi_event_handler, NULL, NULL);
    return ESP_OK;
}

esp_err_t wifi_manager_connect_station(void)
{
    device_config_t cfg;
    device_config_load(&cfg);
    if (cfg.wifi_ssid[0] == '\0') {
        ESP_LOGE(TAG, "No WiFi credentials in NVS");
        return ESP_ERR_INVALID_STATE;
    }
    ESP_LOGI(TAG, "Connecting to SSID: '%s' (len=%d)",
             cfg.wifi_ssid, (int)strlen(cfg.wifi_ssid));

    xEventGroupClearBits(s_wifi_event_group, WIFI_CONNECTED_BIT | WIFI_FAIL_BIT);
    s_retry_count = 0;
    s_connected   = false;

    wifi_config_t wifi_cfg = {0};
    strlcpy((char *)wifi_cfg.sta.ssid,     cfg.wifi_ssid, sizeof(wifi_cfg.sta.ssid));
    strlcpy((char *)wifi_cfg.sta.password, cfg.wifi_pass, sizeof(wifi_cfg.sta.password));
    wifi_cfg.sta.threshold.authmode = WIFI_AUTH_WPA2_PSK;
    wifi_cfg.sta.pmf_cfg.capable  = true;
    wifi_cfg.sta.pmf_cfg.required = false;

    esp_wifi_set_mode(WIFI_MODE_STA);
    esp_wifi_set_config(WIFI_IF_STA, &wifi_cfg);
    esp_wifi_start();
    esp_wifi_connect();

    EventBits_t bits = xEventGroupWaitBits(s_wifi_event_group,
                                           WIFI_CONNECTED_BIT | WIFI_FAIL_BIT,
                                           pdFALSE, pdFALSE,
                                           pdMS_TO_TICKS(30000));
    if (bits & WIFI_CONNECTED_BIT) return ESP_OK;
    return ESP_FAIL;
}

esp_err_t wifi_manager_start_ap(const char *setup_code)
{
    esp_netif_create_default_wifi_ap();
    wifi_config_t ap_cfg = {0};
    snprintf((char *)ap_cfg.ap.ssid, sizeof(ap_cfg.ap.ssid), "CDT-%s", setup_code);
    ap_cfg.ap.ssid_len       = (uint8_t)strlen((char *)ap_cfg.ap.ssid);
    ap_cfg.ap.channel        = 1;
    ap_cfg.ap.authmode       = WIFI_AUTH_OPEN;
    ap_cfg.ap.max_connection = 4;

    esp_wifi_set_mode(WIFI_MODE_AP);
    esp_wifi_set_config(WIFI_IF_AP, &ap_cfg);
    esp_err_t err = esp_wifi_start();
    if (err == ESP_OK) {
        ESP_LOGI(TAG, "AP started: SSID=CDT-%s", setup_code);
    }
    return err;
}

esp_err_t wifi_manager_stop_ap(void)
{
    return esp_wifi_stop();
}

bool wifi_manager_is_connected(void)
{
    return s_connected;
}

EventGroupHandle_t wifi_manager_get_event_group(void)
{
    return s_wifi_event_group;
}
