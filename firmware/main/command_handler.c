#include "command_handler.h"
#include "device_config.h"
#include "telemetry.h"
#include "mqtt_client.h"
#include "sensor_task.h"
#include <string.h>
#include <inttypes.h>
#include "esp_log.h"
#include "esp_system.h"
#include "cJSON.h"
#include "freertos/FreeRTOS.h"
#include "freertos/queue.h"
#include "freertos/event_groups.h"

static const char *TAG = "cmd_handler";

#define QUEUE_DEPTH    4
#define MAX_CMD_LEN    512

typedef struct {
    char   data[MAX_CMD_LEN];
    size_t len;
} cmd_msg_t;

static QueueHandle_t      s_queue      = NULL;
static EventGroupHandle_t s_app_events = NULL;

esp_err_t command_handler_init(EventGroupHandle_t app_events)
{
    s_app_events = app_events;
    s_queue      = xQueueCreate(QUEUE_DEPTH, sizeof(cmd_msg_t));
    if (!s_queue) return ESP_ERR_NO_MEM;
    return ESP_OK;
}

void command_handler_enqueue(const char *payload, size_t len)
{
    if (!s_queue) return;
    cmd_msg_t msg = {0};
    if (len >= MAX_CMD_LEN) len = MAX_CMD_LEN - 1;
    memcpy(msg.data, payload, len);
    msg.len = len;
    /* Called from MQTT task (not a true ISR), use non-ISR variant */
    xQueueSend(s_queue, &msg, 0);
}

static void send_ack(const char *cmd_id, bool ok, const char *err_msg)
{
    static char ack_buf[256];
    if (telemetry_build_ack_json(cmd_id, ok, err_msg, ack_buf, sizeof(ack_buf)) == ESP_OK) {
        mqtt_client_publish_ack(ack_buf, strlen(ack_buf));
    }
}

static void handle_command(const char *json, size_t len)
{
    cJSON *root = cJSON_ParseWithLength(json, len);
    if (!root) {
        ESP_LOGW(TAG, "Failed to parse command JSON");
        return;
    }

    cJSON *id_j   = cJSON_GetObjectItem(root, "command_id");
    cJSON *type_j = cJSON_GetObjectItem(root, "type");
    if (!cJSON_IsString(id_j) || !cJSON_IsString(type_j)) {
        ESP_LOGW(TAG, "Command missing command_id or type");
        cJSON_Delete(root);
        return;
    }

    const char *cmd_id = id_j->valuestring;
    const char *type   = type_j->valuestring;
    ESP_LOGI(TAG, "Command received: type=%s id=%s", type, cmd_id);

    if (strcmp(type, "activate") == 0) {
        device_config_set_provisioned(true);
        xEventGroupSetBits(s_app_events, ACTIVATE_BIT);
        send_ack(cmd_id, true, NULL);
        ESP_LOGI(TAG, "Device activated");

    } else if (strcmp(type, "set_poll_interval") == 0) {
        cJSON *payload = cJSON_GetObjectItem(root, "payload");
        cJSON *ms_j    = payload ? cJSON_GetObjectItem(payload, "interval_ms") : NULL;
        if (cJSON_IsNumber(ms_j)) {
            uint32_t ms = (uint32_t)ms_j->valuedouble;
            if (ms < 35000) ms = 35000;  /* minimum: SDS011 spin-up overhead */
            device_config_set_poll_interval(ms);
            sensor_task_notify_poll_interval(ms);
            send_ack(cmd_id, true, NULL);
            ESP_LOGI(TAG, "Poll interval set to %" PRIu32 " ms", ms);
        } else {
            send_ack(cmd_id, false, "missing interval_ms");
        }

    } else if (strcmp(type, "reboot") == 0) {
        send_ack(cmd_id, true, NULL);
        vTaskDelay(pdMS_TO_TICKS(500));
        esp_restart();

    } else if (strcmp(type, "factory_reset") == 0) {
        send_ack(cmd_id, true, NULL);
        vTaskDelay(pdMS_TO_TICKS(500));
        device_config_factory_reset();  /* does not return */

    } else if (strcmp(type, "update_mqtt_credentials") == 0) {
        cJSON *payload = cJSON_GetObjectItem(root, "payload");
        cJSON *u_j = payload ? cJSON_GetObjectItem(payload, "username") : NULL;
        cJSON *p_j = payload ? cJSON_GetObjectItem(payload, "password") : NULL;
        if (cJSON_IsString(u_j) && cJSON_IsString(p_j)) {
            device_config_set_mqtt_credentials(u_j->valuestring, p_j->valuestring);
            send_ack(cmd_id, true, NULL);
            ESP_LOGI(TAG, "MQTT credentials updated — rebooting");
            vTaskDelay(pdMS_TO_TICKS(500));
            esp_restart();
        } else {
            send_ack(cmd_id, false, "missing username or password");
        }

    } else {
        ESP_LOGW(TAG, "Unknown command type: %s", type);
        send_ack(cmd_id, false, "unknown command type");
    }

    cJSON_Delete(root);
}

void command_handler_task(void *pv)
{
    cmd_msg_t msg;
    for (;;) {
        if (xQueueReceive(s_queue, &msg, portMAX_DELAY) == pdTRUE) {
            handle_command(msg.data, msg.len);
        }
    }
}
