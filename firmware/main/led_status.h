#pragma once
#include "esp_err.h"

typedef enum {
    LED_BOOTING,         /* white slow blink */
    LED_AP_MODE,         /* blue slow blink — waiting for user WiFi setup */
    LED_CONNECTING,      /* yellow blink — connecting to WiFi */
    LED_REGISTERING,     /* cyan blink — registering with server */
    LED_PENDING_CLAIM,   /* purple breathe — waiting to be claimed */
    LED_ACTIVE,          /* solid green — healthy and publishing */
    LED_MQTT_DOWN,       /* orange blink — WiFi ok but MQTT disconnected */
    LED_ERROR,           /* red fast blink — unrecoverable error */
} led_state_t;

/** Initialise WS2812 LED strip driver on CONFIG_CONDUIT_LED_GPIO */
esp_err_t led_status_init(void);

/** Thread-safe: change the current display state */
void led_status_set(led_state_t state);

/** FreeRTOS task entry: pin to Core 0, priority 2, stack 2048 */
void led_status_task(void *pv);
