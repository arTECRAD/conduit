#include "provisioning.h"
#include "device_config.h"
#include <string.h>
#include <stdlib.h>
#include "esp_log.h"
#include "esp_http_server.h"
#include "freertos/FreeRTOS.h"
#include "freertos/task.h"
#include "freertos/event_groups.h"

static const char *TAG = "provisioning";
static httpd_handle_t     s_server     = NULL;
static EventGroupHandle_t s_done_group = NULL;
#define PROV_DONE_BIT BIT0

static const char *HTML_FORM =
    "<!DOCTYPE html><html><head><meta charset=utf-8>"
    "<meta name=viewport content=\"width=device-width,initial-scale=1\">"
    "<title>Conduit Setup</title>"
    "<style>body{font-family:sans-serif;max-width:400px;margin:40px auto;padding:20px}"
    "input{width:100%;padding:8px;margin:8px 0;box-sizing:border-box}"
    "button{width:100%;padding:10px;background:#0070f3;color:#fff;border:none;"
    "border-radius:4px;cursor:pointer}h1{color:#111}</style></head>"
    "<body><h1>Conduit Setup</h1>"
    "<p>Enter your WiFi credentials to connect this device.</p>"
    "<form method=POST action=/configure>"
    "<label>SSID<input type=text name=ssid maxlength=63 required></label>"
    "<label>Password<input type=password name=password maxlength=63></label>"
    "<button type=submit>Connect</button>"
    "</form></body></html>";

static const char *HTML_OK =
    "<!DOCTYPE html><html><body><h1>Connecting...</h1>"
    "<p>The device will connect to your network and reboot. You may close this page.</p>"
    "</body></html>";

static void url_decode(char *s)
{
    char *w = s;
    while (*s) {
        if (*s == '%' && s[1] && s[2]) {
            char hex[3] = {s[1], s[2], '\0'};
            *w++ = (char)strtol(hex, NULL, 16);
            s += 3;
        } else if (*s == '+') {
            *w++ = ' ';
            s++;
        } else {
            *w++ = *s++;
        }
    }
    *w = '\0';
}

static bool parse_field(const char *body, size_t body_len, const char *key,
                         char *out, size_t out_len)
{
    char needle[32];
    snprintf(needle, sizeof(needle), "%s=", key);
    size_t nlen = strlen(needle);
    const char *p = body;

    while (p < body + body_len) {
        if (strncmp(p, needle, nlen) == 0) {
            p += nlen;
            const char *amp = memchr(p, '&', (size_t)(body + body_len - p));
            size_t vlen = amp ? (size_t)(amp - p) : (size_t)(body + body_len - p);
            if (vlen >= out_len) vlen = out_len - 1;
            memcpy(out, p, vlen);
            out[vlen] = '\0';
            url_decode(out);
            return true;
        }
        const char *next = memchr(p, '&', (size_t)(body + body_len - p));
        if (!next) break;
        p = next + 1;
    }
    return false;
}

static esp_err_t redirect_handler(httpd_req_t *req)
{
    httpd_resp_set_status(req, "302 Found");
    httpd_resp_set_hdr(req, "Location", "http://192.168.4.1/");
    httpd_resp_send(req, NULL, 0);
    return ESP_OK;
}

static esp_err_t root_get_handler(httpd_req_t *req)
{
    httpd_resp_set_type(req, "text/html");
    httpd_resp_send(req, HTML_FORM, HTTPD_RESP_USE_STRLEN);
    return ESP_OK;
}

static esp_err_t configure_post_handler(httpd_req_t *req)
{
    char body[256] = {0};
    int len = httpd_req_recv(req, body, sizeof(body) - 1);
    if (len <= 0) {
        httpd_resp_send_500(req);
        return ESP_FAIL;
    }

    char ssid[64] = {0};
    char pass[64] = {0};
    if (!parse_field(body, (size_t)len, "ssid", ssid, sizeof(ssid))) {
        httpd_resp_send_err(req, HTTPD_400_BAD_REQUEST, "Missing ssid");
        return ESP_FAIL;
    }
    parse_field(body, (size_t)len, "password", pass, sizeof(pass));

    device_config_set_wifi(ssid, pass);
    ESP_LOGI(TAG, "WiFi credentials saved: SSID=%s", ssid);

    httpd_resp_set_type(req, "text/html");
    httpd_resp_send(req, HTML_OK, HTTPD_RESP_USE_STRLEN);
    xEventGroupSetBits(s_done_group, PROV_DONE_BIT);
    return ESP_OK;
}

esp_err_t provisioning_start(void)
{
    s_done_group = xEventGroupCreate();

    httpd_config_t config = HTTPD_DEFAULT_CONFIG();
    config.lru_purge_enable = true;
    config.uri_match_fn     = httpd_uri_match_wildcard;

    esp_err_t err = httpd_start(&s_server, &config);
    if (err != ESP_OK) {
        ESP_LOGE(TAG, "httpd_start failed: %s", esp_err_to_name(err));
        return err;
    }

    static const httpd_uri_t root     = {.uri = "/",          .method = HTTP_GET,  .handler = root_get_handler};
    static const httpd_uri_t cfg_post = {.uri = "/configure", .method = HTTP_POST, .handler = configure_post_handler};
    static const httpd_uri_t wild     = {.uri = "/*",         .method = HTTP_GET,  .handler = redirect_handler};

    httpd_register_uri_handler(s_server, &root);
    httpd_register_uri_handler(s_server, &cfg_post);
    httpd_register_uri_handler(s_server, &wild);

    ESP_LOGI(TAG, "Captive portal started");
    return ESP_OK;
}

esp_err_t provisioning_stop(void)
{
    if (s_server) {
        httpd_stop(s_server);
        s_server = NULL;
    }
    return ESP_OK;
}

bool provisioning_is_complete(void)
{
    if (!s_done_group) return false;
    return (xEventGroupGetBits(s_done_group) & PROV_DONE_BIT) != 0;
}

esp_err_t provisioning_wait(uint32_t timeout_ms)
{
    EventBits_t bits = xEventGroupWaitBits(s_done_group, PROV_DONE_BIT,
                                           pdFALSE, pdTRUE,
                                           pdMS_TO_TICKS(timeout_ms));
    return (bits & PROV_DONE_BIT) ? ESP_OK : ESP_ERR_TIMEOUT;
}
