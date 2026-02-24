#pragma once
#include "esp_err.h"
#include "freertos/FreeRTOS.h"
#include "freertos/event_groups.h"
#include <stddef.h>

/* App event bits */
#define ACTIVATE_BIT   BIT0

/**
 * @brief  Initialise command handler with the app event group.
 *         Sets ACTIVATE_BIT when "activate" command is received.
 */
esp_err_t command_handler_init(EventGroupHandle_t app_events);

/** FreeRTOS task entry: pin to Core 0, priority 6, stack 4096 */
void command_handler_task(void *pv);

/** Called from MQTT callback to enqueue raw command JSON. Thread-safe. */
void command_handler_enqueue(const char *payload, size_t len);
