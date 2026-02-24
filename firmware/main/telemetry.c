#include "telemetry.h"
#include "server_config.h"
#include <string.h>
#include "esp_log.h"
#include "cJSON.h"

static const char *TAG = "telemetry";

esp_err_t telemetry_build_json(const sensor_reading_t *r, const char *id,
                                char *buf, size_t len)
{
    cJSON *root = cJSON_CreateObject();
    if (!root) return ESP_ERR_NO_MEM;

    cJSON_AddStringToObject(root, "device_id", id);
    if (r->ts_unix > 0) {
        cJSON_AddNumberToObject(root, "timestamp", (double)r->ts_unix);
    }

    cJSON *sensors = cJSON_AddObjectToObject(root, "sensors");
    if (!sensors) { cJSON_Delete(root); return ESP_ERR_NO_MEM; }

    cJSON_AddNumberToObject(sensors, "temperature_c", (double)r->temp_c);
    cJSON_AddNumberToObject(sensors, "humidity_pct",  (double)r->hum_pct);

    /* Omit CCS811 fields entirely during warmup (prevents misleading 400ppm baseline) */
    if (!r->warming_up) {
        cJSON_AddNumberToObject(sensors, "eco2_ppm", (double)r->eco2);
        cJSON_AddNumberToObject(sensors, "tvoc_ppb", (double)r->tvoc);
    }

    cJSON_AddNumberToObject(sensors, "pm25_ugm3", (double)r->pm25);
    cJSON_AddNumberToObject(sensors, "pm10_ugm3", (double)r->pm10);

    char *s = cJSON_PrintUnformatted(root);
    cJSON_Delete(root);
    if (!s) return ESP_ERR_NO_MEM;

    if (strlen(s) >= len) {
        cJSON_free(s);
        ESP_LOGE(TAG, "telemetry JSON too large for buffer (%zu >= %zu)", strlen(s), len);
        return ESP_ERR_INVALID_SIZE;
    }
    strlcpy(buf, s, len);
    cJSON_free(s);
    return ESP_OK;
}

esp_err_t telemetry_build_health_json(const sensor_reading_t *r, const char *id,
                                       char *buf, size_t len)
{
    cJSON *root = cJSON_CreateObject();
    if (!root) return ESP_ERR_NO_MEM;

    cJSON_AddStringToObject(root, "device_id",       id);
    cJSON_AddStringToObject(root, "fw_version",      CONDUIT_FW_VERSION);
    cJSON_AddBoolToObject  (root, "ccs811_warming_up", r->warming_up);
    if (r->ts_unix > 0) {
        cJSON_AddNumberToObject(root, "timestamp", (double)r->ts_unix);
    }

    char *s = cJSON_PrintUnformatted(root);
    cJSON_Delete(root);
    if (!s) return ESP_ERR_NO_MEM;

    if (strlen(s) >= len) {
        cJSON_free(s);
        return ESP_ERR_INVALID_SIZE;
    }
    strlcpy(buf, s, len);
    cJSON_free(s);
    return ESP_OK;
}

esp_err_t telemetry_build_ack_json(const char *cmd_id, bool ok,
                                    const char *err_msg, char *buf, size_t len)
{
    cJSON *root = cJSON_CreateObject();
    if (!root) return ESP_ERR_NO_MEM;

    cJSON_AddStringToObject(root, "command_id", cmd_id);
    cJSON_AddStringToObject(root, "status",     ok ? "success" : "error");
    /* Always include "error" field; empty string on success (matches model.CommandAck) */
    cJSON_AddStringToObject(root, "error",
                            (!ok && err_msg && err_msg[0]) ? err_msg : "");

    char *s = cJSON_PrintUnformatted(root);
    cJSON_Delete(root);
    if (!s) return ESP_ERR_NO_MEM;

    if (strlen(s) >= len) {
        cJSON_free(s);
        return ESP_ERR_INVALID_SIZE;
    }
    strlcpy(buf, s, len);
    cJSON_free(s);
    return ESP_OK;
}
