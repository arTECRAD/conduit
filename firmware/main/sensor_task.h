#pragma once
#include <stdint.h>
#include <stdbool.h>

typedef struct {
    float    temp_c;
    float    hum_pct;
    uint16_t eco2;        /* ppm; only valid when warming_up=false */
    uint16_t tvoc;        /* ppb; only valid when warming_up=false */
    float    pm25;        /* µg/m³ */
    float    pm10;        /* µg/m³ */
    bool     warming_up;  /* CCS811 still in 20-min warmup period */
    int64_t  ts_unix;     /* Unix epoch seconds; 0 if SNTP not synced */
} sensor_reading_t;

/** FreeRTOS task entry: pin to Core 1, priority 5, stack 8192 */
void sensor_task(void *pv);

/** Thread-safe: deliver a new poll interval to the running sensor task */
void sensor_task_notify_poll_interval(uint32_t ms);

/** Call once after "activate" command to begin publishing telemetry */
void sensor_task_enable_publish(void);
