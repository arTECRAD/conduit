#pragma once
#include "esp_err.h"
#include "freertos/FreeRTOS.h"
#include "freertos/event_groups.h"
#include <stdbool.h>

#define WIFI_CONNECTED_BIT   BIT0
#define WIFI_FAIL_BIT        BIT1
#define WIFI_MAX_RETRY       5

/* Initialise WiFi driver and TCP/IP stack */
esp_err_t wifi_manager_init(void);

/* Connect to saved SSID/password in station mode. Blocks until connected or max retries. */
esp_err_t wifi_manager_connect_station(void);

/* Start softAP with SSID "CDT-<setup_code>" */
esp_err_t wifi_manager_start_ap(const char *setup_code);

/* Stop softAP */
esp_err_t wifi_manager_stop_ap(void);

bool      wifi_manager_is_connected(void);

/* Returns the event group; bits: WIFI_CONNECTED_BIT, WIFI_FAIL_BIT */
EventGroupHandle_t wifi_manager_get_event_group(void);
