#include "sds011.h"
#include <string.h>
#include "driver/uart.h"
#include "esp_log.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"

static const char *TAG = "sds011";
static int s_uart_num = -1;

/* SDS011 command structure: AA B4 [15 bytes data] AB */
#define CMD_HEAD   0xAA
#define CMD_ID     0xB4
#define CMD_TAIL   0xAB
#define CMD_LEN    19

static void build_cmd(uint8_t *buf, uint8_t b2, uint8_t b3, uint8_t b4)
{
    memset(buf, 0, CMD_LEN);
    buf[0] = CMD_HEAD;
    buf[1] = CMD_ID;
    buf[2] = b2;   /* command byte */
    buf[3] = b3;
    buf[4] = b4;
    /* bytes 5-14 are 0x00 (device ID FF FF = broadcast) */
    buf[15] = 0xFF;
    buf[16] = 0xFF;
    /* checksum: sum of bytes 2..16 */
    uint8_t sum = 0;
    for (int i = 2; i <= 16; i++) sum += buf[i];
    buf[17] = sum;
    buf[18] = CMD_TAIL;
}

esp_err_t sds011_init(const sds011_config_t *cfg)
{
    s_uart_num = cfg->uart_num;
    uart_config_t uart_cfg = {
        .baud_rate  = SDS011_BAUD_RATE,
        .data_bits  = UART_DATA_8_BITS,
        .parity     = UART_PARITY_DISABLE,
        .stop_bits  = UART_STOP_BITS_1,
        .flow_ctrl  = UART_HW_FLOWCTRL_DISABLE,
        .source_clk = UART_SCLK_DEFAULT,
    };
    esp_err_t err = uart_param_config(cfg->uart_num, &uart_cfg);
    if (err != ESP_OK) return err;
    err = uart_set_pin(cfg->uart_num, cfg->tx_gpio, cfg->rx_gpio,
                       UART_PIN_NO_CHANGE, UART_PIN_NO_CHANGE);
    if (err != ESP_OK) return err;
    err = uart_driver_install(cfg->uart_num, 256, 0, 0, NULL, 0);
    if (err != ESP_OK) return err;
    ESP_LOGI(TAG, "SDS011 UART%d initialised (RX=%d TX=%d)", cfg->uart_num, cfg->rx_gpio, cfg->tx_gpio);
    return ESP_OK;
}

esp_err_t sds011_wake(void)
{
    if (s_uart_num < 0) return ESP_ERR_INVALID_STATE;
    uint8_t cmd[CMD_LEN];
    /* Work mode command: byte2=0x06, byte3=0x01 (set), byte4=0x01 (work) */
    build_cmd(cmd, 0x06, 0x01, 0x01);
    int written = uart_write_bytes(s_uart_num, cmd, CMD_LEN);
    if (written != CMD_LEN) return ESP_FAIL;
    ESP_LOGD(TAG, "wake sent");
    return ESP_OK;
}

esp_err_t sds011_read(sds011_reading_t *out)
{
    if (s_uart_num < 0) return ESP_ERR_INVALID_STATE;

    /* Flush stale data */
    uart_flush_input(s_uart_num);

    uint8_t frame[SDS011_FRAME_LEN];
    int64_t deadline_ms = (int64_t)(xTaskGetTickCount() * portTICK_PERIOD_MS) + 5000;

    while ((int64_t)(xTaskGetTickCount() * portTICK_PERIOD_MS) < deadline_ms) {
        uint8_t head;
        int n = uart_read_bytes(s_uart_num, &head, 1, pdMS_TO_TICKS(200));
        if (n <= 0 || head != 0xAA) continue;

        /* Read remaining 9 bytes */
        int rem = uart_read_bytes(s_uart_num, &frame[1], SDS011_FRAME_LEN - 1, pdMS_TO_TICKS(500));
        if (rem != SDS011_FRAME_LEN - 1) continue;
        frame[0] = head;

        /* Validate: frame[1]=0xC0, frame[9]=0xAB */
        if (frame[1] != 0xC0 || frame[9] != 0xAB) continue;

        /* Checksum: sum of bytes 2..7 */
        uint8_t sum = 0;
        for (int i = 2; i <= 7; i++) sum += frame[i];
        if (sum != frame[8]) {
            ESP_LOGW(TAG, "checksum mismatch");
            continue;
        }

        out->pm25_ugm3 = (float)(((uint16_t)frame[3] << 8) | frame[2]) / 10.0f;
        out->pm10_ugm3 = (float)(((uint16_t)frame[5] << 8) | frame[4]) / 10.0f;
        ESP_LOGD(TAG, "PM2.5=%.1f PM10=%.1f", out->pm25_ugm3, out->pm10_ugm3);
        return ESP_OK;
    }

    ESP_LOGW(TAG, "read timeout");
    return ESP_ERR_TIMEOUT;
}

esp_err_t sds011_sleep(void)
{
    if (s_uart_num < 0) return ESP_ERR_INVALID_STATE;
    uint8_t cmd[CMD_LEN];
    /* Work mode command: byte2=0x06, byte3=0x01 (set), byte4=0x00 (sleep) */
    build_cmd(cmd, 0x06, 0x01, 0x00);
    int written = uart_write_bytes(s_uart_num, cmd, CMD_LEN);
    if (written != CMD_LEN) return ESP_FAIL;
    ESP_LOGD(TAG, "sleep sent");
    return ESP_OK;
}
