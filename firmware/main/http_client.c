#include "http_client.h"
#include "server_config.h"
#include <string.h>
#include <stdio.h>
#include "esp_log.h"
#include "esp_http_client.h"
#include "cJSON.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"

static const char *TAG = "http_client";

#define MAX_RETRIES      3
#define RETRY_DELAY_MS   5000
#define RESP_BUF_SIZE    512

typedef struct {
    char buf[RESP_BUF_SIZE];
    int  len;
} http_ctx_t;

static esp_err_t http_event_handler(esp_http_client_event_t *evt)
{
    http_ctx_t *ctx = (http_ctx_t *)evt->user_data;
    if (!ctx) return ESP_OK;
    if (evt->event_id == HTTP_EVENT_ON_DATA) {
        int remaining = RESP_BUF_SIZE - 1 - ctx->len;
        if (remaining > 0 && evt->data_len > 0) {
            int copy = evt->data_len < remaining ? evt->data_len : remaining;
            memcpy(&ctx->buf[ctx->len], evt->data, copy);
            ctx->len += copy;
            ctx->buf[ctx->len] = '\0';
        }
    }
    return ESP_OK;
}

static esp_err_t do_register(const char *identifier, const char *setup_code,
                              register_response_t *out, int *out_status)
{
    http_ctx_t ctx = {0};

    char url[128];
    snprintf(url, sizeof(url), "https://%s%s", SERVER_HOST, SERVER_REGISTER_PATH);

    esp_http_client_config_t config = {
        .url           = url,
        .method        = HTTP_METHOD_POST,
        .cert_pem      = (const char *)ca_cert_pem_start,
        .event_handler = http_event_handler,
        .user_data     = &ctx,
        .timeout_ms    = 10000,
        .buffer_size   = 1024,
    };

    esp_http_client_handle_t client = esp_http_client_init(&config);
    if (!client) return ESP_FAIL;

    char body[256];
    int blen = snprintf(body, sizeof(body),
        "{\"device_identifier\":\"%s\",\"setup_code\":\"%s\",\"provisioning_token\":\"%s\"}",
        identifier, setup_code, PROVISIONING_TOKEN);

    esp_http_client_set_header(client, "Content-Type", "application/json");
    esp_http_client_set_post_field(client, body, blen);

    esp_err_t err = esp_http_client_perform(client);
    if (err == ESP_OK) {
        *out_status = esp_http_client_get_status_code(client);
    }
    esp_http_client_cleanup(client);

    if (err != ESP_OK) {
        ESP_LOGE(TAG, "HTTP perform failed: %s", esp_err_to_name(err));
        return err;
    }

    if (*out_status == 409) {
        ESP_LOGW(TAG, "Server returned 409 (already registered)");
        return ESP_ERR_INVALID_RESPONSE;
    }
    if (*out_status != 200 && *out_status != 201) {
        ESP_LOGE(TAG, "Unexpected HTTP status %d", *out_status);
        return ESP_FAIL;
    }

    /* Parse: {"data":{"device_id":...,"mqtt_username":...,"mqtt_password":...,"status":...}} */
    cJSON *root = cJSON_Parse(ctx.buf);
    if (!root) {
        ESP_LOGE(TAG, "Failed to parse registration response");
        return ESP_FAIL;
    }

    esp_err_t ret = ESP_FAIL;
    cJSON *data = cJSON_GetObjectItem(root, "data");
    if (data) {
        cJSON *did  = cJSON_GetObjectItem(data, "device_id");
        cJSON *user = cJSON_GetObjectItem(data, "mqtt_username");
        cJSON *pass = cJSON_GetObjectItem(data, "mqtt_password");
        cJSON *stat = cJSON_GetObjectItem(data, "status");
        if (cJSON_IsString(did)  && cJSON_IsString(user) &&
            cJSON_IsString(pass) && cJSON_IsString(stat)) {
            strlcpy(out->device_id_uuid, did->valuestring,  sizeof(out->device_id_uuid));
            strlcpy(out->mqtt_username,  user->valuestring, sizeof(out->mqtt_username));
            strlcpy(out->mqtt_password,  pass->valuestring, sizeof(out->mqtt_password));
            strlcpy(out->status,         stat->valuestring, sizeof(out->status));
            ret = ESP_OK;
        } else {
            ESP_LOGE(TAG, "Response missing required fields");
        }
    }
    cJSON_Delete(root);
    return ret;
}

esp_err_t http_client_register_device(const char *identifier,
                                       const char *setup_code,
                                       register_response_t *out)
{
    for (int attempt = 1; attempt <= MAX_RETRIES; attempt++) {
        int status = 0;
        esp_err_t err = do_register(identifier, setup_code, out, &status);

        if (err == ESP_ERR_INVALID_RESPONSE) {
            /* 409 — do NOT retry */
            return ESP_ERR_INVALID_RESPONSE;
        }
        if (err == ESP_OK) {
            ESP_LOGI(TAG, "Registration successful (attempt %d/%d)", attempt, MAX_RETRIES);
            return ESP_OK;
        }

        ESP_LOGW(TAG, "Registration attempt %d/%d failed: %s",
                 attempt, MAX_RETRIES, esp_err_to_name(err));
        if (attempt < MAX_RETRIES) {
            vTaskDelay(pdMS_TO_TICKS(RETRY_DELAY_MS));
        }
    }
    return ESP_FAIL;
}
