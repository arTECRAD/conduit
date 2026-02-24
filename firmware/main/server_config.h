#pragma once
#include <stdint.h>

#define SERVER_HOST             "conduit.colinshirley.com"
#define SERVER_PORT             443
#define SERVER_REGISTER_PATH    "/api/v1/devices/register"
#define MQTT_BROKER_HOST        "conduit.colinshirley.com"
#define MQTT_BROKER_PORT        8883
#define PROVISIONING_TOKEN      CONFIG_CONDUIT_PROVISIONING_TOKEN
#define CONDUIT_FW_VERSION      "1.0.0"

/* TLS verification uses the built-in mbedTLS certificate bundle
 * (CONFIG_MBEDTLS_CERTIFICATE_BUNDLE_DEFAULT_FULL=y in sdkconfig.defaults)
 * via esp_crt_bundle_attach — no manually embedded cert needed. */
