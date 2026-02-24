/* Include ESP-IDF MQTT header first — no filename conflict since this file
   is conduit_mqtt.c, not mqtt_client.c */
#include "mqtt_client.h"   /* ESP-IDF: esp_mqtt_client_handle_t, config, events */
#include "conduit_mqtt.h"
#include "server_config.h"
#include <string.h>
#include <stdio.h>
#include "esp_log.h"
#include "esp_event.h"
#include "freertos/FreeRTOS.h"
#include "freertos/event_groups.h"

static const char *TAG = "conduit_mqtt";

static esp_mqtt_client_handle_t s_client    = NULL;
static EventGroupHandle_t       s_evt_group = NULL;
static mqtt_cmd_cb_t            s_cmd_cb    = NULL;

/* Topic strings — built from MQTT username at init time */
static char s_cmd_topic[64];
static char s_tel_topic[64];
static char s_hlt_topic[64];
static char s_ack_topic[64];

static void mqtt_event_handler(void *arg, esp_event_base_t base,
                                int32_t event_id, void *event_data)
{
    esp_mqtt_event_handle_t evt = (esp_mqtt_event_handle_t)event_data;
    switch (evt->event_id) {
    case MQTT_EVENT_CONNECTED:
        ESP_LOGI(TAG, "MQTT connected");
        xEventGroupSetBits(s_evt_group, MQTT_CONNECTED_BIT);
        xEventGroupClearBits(s_evt_group, MQTT_DISCONNECTED_BIT);
        esp_mqtt_client_subscribe(s_client, s_cmd_topic, 1);
        ESP_LOGI(TAG, "Subscribed to %s", s_cmd_topic);
        break;

    case MQTT_EVENT_DISCONNECTED:
        ESP_LOGW(TAG, "MQTT disconnected");
        xEventGroupSetBits(s_evt_group, MQTT_DISCONNECTED_BIT);
        xEventGroupClearBits(s_evt_group, MQTT_CONNECTED_BIT);
        break;

    case MQTT_EVENT_DATA:
        if (s_cmd_cb && evt->data && evt->data_len > 0) {
            s_cmd_cb(evt->data, (size_t)evt->data_len);
        }
        break;

    case MQTT_EVENT_ERROR:
        if (evt->error_handle) {
            ESP_LOGE(TAG, "MQTT error type=%d", evt->error_handle->error_type);
        }
        break;

    default:
        break;
    }
}

esp_err_t mqtt_client_init(const char *username, const char *password, mqtt_cmd_cb_t cb)
{
    s_cmd_cb    = cb;
    s_evt_group = xEventGroupCreate();
    if (!s_evt_group) return ESP_ERR_NO_MEM;

    snprintf(s_cmd_topic, sizeof(s_cmd_topic), "devices/%s/command",     username);
    snprintf(s_tel_topic, sizeof(s_tel_topic), "devices/%s/telemetry",   username);
    snprintf(s_hlt_topic, sizeof(s_hlt_topic), "devices/%s/health",      username);
    snprintf(s_ack_topic, sizeof(s_ack_topic), "devices/%s/command/ack", username);

    char broker_uri[64];
    snprintf(broker_uri, sizeof(broker_uri), "mqtts://%s:%d",
             MQTT_BROKER_HOST, MQTT_BROKER_PORT);

    esp_mqtt_client_config_t config = {
        .broker = {
            .address.uri = broker_uri,
            .verification.certificate = (const char *)ca_cert_pem_start,
        },
        .credentials = {
            .username = username,
            .authentication.password = password,
        },
        .session.keepalive = 60,
        .network = {
            .timeout_ms           = 10000,
            .reconnect_timeout_ms = 5000,
        },
        .buffer.size = 2048,
    };

    s_client = esp_mqtt_client_init(&config);
    if (!s_client) return ESP_FAIL;

    esp_mqtt_client_register_event(s_client, ESP_EVENT_ANY_ID,
                                   mqtt_event_handler, NULL);
    return ESP_OK;
}

esp_err_t mqtt_client_start(void)
{
    if (!s_client) return ESP_ERR_INVALID_STATE;
    return esp_mqtt_client_start(s_client);
}

esp_err_t mqtt_client_stop(void)
{
    if (!s_client) return ESP_ERR_INVALID_STATE;
    return esp_mqtt_client_stop(s_client);
}

static esp_err_t do_publish(const char *topic, const char *json, size_t len)
{
    if (!s_client) return ESP_ERR_INVALID_STATE;
    int msg_id = esp_mqtt_client_publish(s_client, topic, json, (int)len, 1, 0);
    if (msg_id < 0) {
        ESP_LOGW(TAG, "publish to %s failed", topic);
        return ESP_FAIL;
    }
    return ESP_OK;
}

esp_err_t mqtt_client_publish_telemetry(const char *json, size_t len)
{
    return do_publish(s_tel_topic, json, len);
}

esp_err_t mqtt_client_publish_health(const char *json, size_t len)
{
    return do_publish(s_hlt_topic, json, len);
}

esp_err_t mqtt_client_publish_ack(const char *json, size_t len)
{
    return do_publish(s_ack_topic, json, len);
}

esp_err_t mqtt_client_wait_connected(uint32_t timeout_ms)
{
    EventBits_t bits = xEventGroupWaitBits(s_evt_group, MQTT_CONNECTED_BIT,
                                           pdFALSE, pdTRUE,
                                           pdMS_TO_TICKS(timeout_ms));
    return (bits & MQTT_CONNECTED_BIT) ? ESP_OK : ESP_ERR_TIMEOUT;
}

bool mqtt_client_is_connected(void)
{
    if (!s_evt_group) return false;
    return (xEventGroupGetBits(s_evt_group) & MQTT_CONNECTED_BIT) != 0;
}

EventGroupHandle_t mqtt_client_get_event_group(void)
{
    return s_evt_group;
}
