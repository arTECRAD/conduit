#pragma once
#include "esp_err.h"
#include <stdint.h>
#include <stdbool.h>

#define DEVICE_IDENTIFIER_LEN  13   /* 12 hex chars + NUL */
#define SETUP_CODE_LEN         10   /* "XXXX-XXXX" + NUL */

typedef struct {
    char     device_identifier[DEVICE_IDENTIFIER_LEN];  /* e.g. "a1b2c3d4e5f6" */
    char     setup_code[SETUP_CODE_LEN];                 /* e.g. "A3BX-7C9M" */
    char     wifi_ssid[64];
    char     wifi_pass[64];
    char     mqtt_user[32];
    char     mqtt_pass[50];
    bool     registered;     /* mqtt_user/pass have been obtained from server */
    bool     provisioned;    /* device has been claimed and activated */
    uint32_t poll_interval_ms;
} device_config_t;

/* Initialise NVS and open conduit namespace */
esp_err_t device_config_init(void);

/* Load current config from NVS into *out_cfg */
esp_err_t device_config_load(device_config_t *out_cfg);

/* Derive device_identifier (12-char lowercase hex) and setup_code (XXXX-XXXX) from MAC */
esp_err_t device_config_ensure_identity(char *out_id, char *out_code);

esp_err_t device_config_set_wifi(const char *ssid, const char *pass);
esp_err_t device_config_set_mqtt_credentials(const char *user, const char *pass);
esp_err_t device_config_set_registered(bool v);
esp_err_t device_config_set_provisioned(bool v);
esp_err_t device_config_set_poll_interval(uint32_t ms);

/* Erase all NVS keys in conduit namespace then restart */
esp_err_t device_config_factory_reset(void);
