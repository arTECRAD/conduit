#include "device_config.h"
#include <string.h>
#include <stdint.h>
#include "esp_log.h"
#include "esp_system.h"
#include "esp_mac.h"
#include "nvs_flash.h"
#include "nvs.h"
#include "mbedtls/sha256.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"

static const char *TAG = "device_config";
#define NVS_NS   "conduit"

/* Custom alphabet: excludes 0,1,O,I to avoid visual ambiguity */
static const char CODE_ALPHABET[] = "23456789ABCDEFGHJKMNPQRSTUVWXYZ";
#define ALPHABET_LEN 31

static nvs_handle_t s_nvs;

esp_err_t device_config_init(void)
{
    esp_err_t err = nvs_flash_init();
    if (err == ESP_ERR_NVS_NO_FREE_PAGES || err == ESP_ERR_NVS_NEW_VERSION_FOUND) {
        ESP_LOGW(TAG, "NVS partition truncated, erasing");
        nvs_flash_erase();
        err = nvs_flash_init();
    }
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "nvs_flash_init failed: %s", esp_err_to_name(err));
        return err;
    }
    err = nvs_open(NVS_NS, NVS_READWRITE, &s_nvs);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "nvs_open failed: %s", esp_err_to_name(err));
    }
    return err;
}

esp_err_t device_config_ensure_identity(char *out_id, char *out_code)
{
    uint8_t mac[6];
    esp_efuse_mac_get_default(mac);

    /* device_identifier = lowercase hex of 6-byte MAC */
    snprintf(out_id, DEVICE_IDENTIFIER_LEN, "%02x%02x%02x%02x%02x%02x",
             mac[0], mac[1], mac[2], mac[3], mac[4], mac[5]);

    /* setup_code: SHA-256 of MAC bytes -> 8 chars from alphabet, formatted XXXX-XXXX */
    uint8_t digest[32];
    mbedtls_sha256(mac, 6, digest, 0);
    out_code[0] = CODE_ALPHABET[digest[0] % ALPHABET_LEN];
    out_code[1] = CODE_ALPHABET[digest[1] % ALPHABET_LEN];
    out_code[2] = CODE_ALPHABET[digest[2] % ALPHABET_LEN];
    out_code[3] = CODE_ALPHABET[digest[3] % ALPHABET_LEN];
    out_code[4] = '-';
    out_code[5] = CODE_ALPHABET[digest[4] % ALPHABET_LEN];
    out_code[6] = CODE_ALPHABET[digest[5] % ALPHABET_LEN];
    out_code[7] = CODE_ALPHABET[digest[6] % ALPHABET_LEN];
    out_code[8] = CODE_ALPHABET[digest[7] % ALPHABET_LEN];
    out_code[9] = '\0';

    ESP_LOGI(TAG, "Device ID: %s  Setup code: %s", out_id, out_code);
    return ESP_OK;
}

static esp_err_t nvs_get_str_safe(const char *key, char *out, size_t maxlen)
{
    size_t len = maxlen;
    esp_err_t err = nvs_get_str(s_nvs, key, out, &len);
    if (err == ESP_ERR_NVS_NOT_FOUND) {
        out[0] = '\0';
        return ESP_OK;
    }
    return err;
}

static esp_err_t nvs_get_u8_safe(const char *key, uint8_t *out)
{
    esp_err_t err = nvs_get_u8(s_nvs, key, out);
    if (err == ESP_ERR_NVS_NOT_FOUND) { *out = 0; return ESP_OK; }
    return err;
}

static esp_err_t nvs_get_u32_safe(const char *key, uint32_t *out, uint32_t def)
{
    esp_err_t err = nvs_get_u32(s_nvs, key, out);
    if (err == ESP_ERR_NVS_NOT_FOUND) { *out = def; return ESP_OK; }
    return err;
}

esp_err_t device_config_load(device_config_t *out)
{
    memset(out, 0, sizeof(*out));
    device_config_ensure_identity(out->device_identifier, out->setup_code);

    nvs_get_str_safe("wifi_ssid",  out->wifi_ssid, sizeof(out->wifi_ssid));
    nvs_get_str_safe("wifi_pass",  out->wifi_pass, sizeof(out->wifi_pass));
    nvs_get_str_safe("mqtt_user",  out->mqtt_user, sizeof(out->mqtt_user));
    nvs_get_str_safe("mqtt_pass",  out->mqtt_pass, sizeof(out->mqtt_pass));

    uint8_t reg = 0, prov = 0;
    nvs_get_u8_safe("registered",  &reg);
    nvs_get_u8_safe("provisioned", &prov);
    out->registered  = (reg != 0);
    out->provisioned = (prov != 0);

    nvs_get_u32_safe("poll_ms", &out->poll_interval_ms,
                     CONFIG_CONDUIT_DEFAULT_POLL_INTERVAL_MS);
    return ESP_OK;
}

esp_err_t device_config_set_wifi(const char *ssid, const char *pass)
{
    esp_err_t err = nvs_set_str(s_nvs, "wifi_ssid", ssid);
    if (err == ESP_OK) err = nvs_set_str(s_nvs, "wifi_pass", pass);
    if (err == ESP_OK) err = nvs_commit(s_nvs);
    return err;
}

esp_err_t device_config_set_mqtt_credentials(const char *user, const char *pass)
{
    esp_err_t err = nvs_set_str(s_nvs, "mqtt_user", user);
    if (err == ESP_OK) err = nvs_set_str(s_nvs, "mqtt_pass", pass);
    if (err == ESP_OK) err = nvs_commit(s_nvs);
    return err;
}

esp_err_t device_config_set_registered(bool v)
{
    esp_err_t err = nvs_set_u8(s_nvs, "registered", v ? 1 : 0);
    if (err == ESP_OK) err = nvs_commit(s_nvs);
    return err;
}

esp_err_t device_config_set_provisioned(bool v)
{
    esp_err_t err = nvs_set_u8(s_nvs, "provisioned", v ? 1 : 0);
    if (err == ESP_OK) err = nvs_commit(s_nvs);
    return err;
}

esp_err_t device_config_set_poll_interval(uint32_t ms)
{
    esp_err_t err = nvs_set_u32(s_nvs, "poll_ms", ms);
    if (err == ESP_OK) err = nvs_commit(s_nvs);
    return err;
}

esp_err_t device_config_factory_reset(void)
{
    ESP_LOGW(TAG, "Factory reset: erasing NVS namespace");
    esp_err_t err = nvs_erase_all(s_nvs);
    if (err == ESP_OK) err = nvs_commit(s_nvs);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "NVS erase failed: %s", esp_err_to_name(err));
        return err;
    }
    vTaskDelay(pdMS_TO_TICKS(500));
    esp_restart();
    return ESP_OK;  /* unreachable */
}
