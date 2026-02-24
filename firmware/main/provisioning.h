#pragma once
#include "esp_err.h"
#include <stdbool.h>
#include <stdint.h>

/* Start the captive portal HTTP server.
   After provisioning completes, reboot the device. */
esp_err_t provisioning_start(void);

/* Stop the HTTP server */
esp_err_t provisioning_stop(void);

/* Returns true once WiFi credentials have been saved to NVS */
bool provisioning_is_complete(void);

/* Block until provisioning completes or timeout_ms elapses.
   Pass portMAX_DELAY to wait indefinitely. */
esp_err_t provisioning_wait(uint32_t timeout_ms);
