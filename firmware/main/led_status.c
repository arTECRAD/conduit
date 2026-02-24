#include "led_status.h"
#include "esp_log.h"
#include "led_strip.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include <stdatomic.h>

static const char *TAG = "led_status";

static led_strip_handle_t s_strip = NULL;
static atomic_int         s_state = LED_BOOTING;

esp_err_t led_status_init(void)
{
    led_strip_config_t strip_cfg = {
        .strip_gpio_num           = CONFIG_CONDUIT_LED_GPIO,
        .max_leds                 = 1,
        .led_model                = LED_MODEL_WS2812,
        .color_component_format   = LED_STRIP_COLOR_COMPONENT_FMT_GRB,
        .flags.invert_out         = false,
    };
    led_strip_rmt_config_t rmt_cfg = {
        .clk_src       = RMT_CLK_SRC_DEFAULT,
        .resolution_hz = 10000000,  /* 10 MHz */
        .flags.with_dma = false,
    };
    esp_err_t err = led_strip_new_rmt_device(&strip_cfg, &rmt_cfg, &s_strip);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "led_strip_new_rmt_device failed: %s", esp_err_to_name(err));
        return err;
    }
    led_strip_clear(s_strip);
    return ESP_OK;
}

void led_status_set(led_state_t state)
{
    atomic_store(&s_state, (int)state);
}

static void set_rgb(uint8_t r, uint8_t g, uint8_t b)
{
    if (!s_strip) return;
    led_strip_set_pixel(s_strip, 0, r, g, b);
    led_strip_refresh(s_strip);
}

static void led_off(void)
{
    if (!s_strip) return;
    led_strip_clear(s_strip);
}

void led_status_task(void *pv)
{
    uint32_t tick = 0;
    for (;;) {
        led_state_t state = (led_state_t)atomic_load(&s_state);
        bool on = (tick % 2) == 0;

        switch (state) {
        case LED_BOOTING:
            /* White slow blink */
            on ? set_rgb(20, 20, 20) : led_off();
            break;

        case LED_AP_MODE:
            /* Blue slow blink (10 ticks on, 10 off = 1 Hz at 50ms tick) */
            (tick % 20 < 10) ? set_rgb(0, 0, 40) : led_off();
            break;

        case LED_CONNECTING:
            /* Yellow blink */
            on ? set_rgb(40, 30, 0) : led_off();
            break;

        case LED_REGISTERING:
            /* Cyan blink */
            on ? set_rgb(0, 40, 40) : led_off();
            break;

        case LED_PENDING_CLAIM: {
            /* Purple breathe: triangle wave 0..40 over 40 ticks */
            uint32_t phase = tick % 40;
            uint8_t v = (uint8_t)(phase < 20 ? phase * 2 : (40 - phase) * 2);
            set_rgb(v, 0, v);
            break;
        }

        case LED_ACTIVE:
            /* Solid green */
            set_rgb(0, 40, 0);
            break;

        case LED_MQTT_DOWN:
            /* Orange blink */
            on ? set_rgb(40, 20, 0) : led_off();
            break;

        case LED_ERROR:
            /* Red fast blink (4-tick period = 10 Hz) */
            (tick % 4 < 2) ? set_rgb(60, 0, 0) : led_off();
            break;
        }

        tick++;
        vTaskDelay(pdMS_TO_TICKS(50));  /* 50ms tick = 20 Hz */
    }
}
