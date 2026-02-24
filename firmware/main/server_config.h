#pragma once
#include <stdint.h>

#define SERVER_HOST             "conduit.colinshirley.com"
#define SERVER_PORT             443
#define SERVER_REGISTER_PATH    "/api/v1/devices/register"
#define MQTT_BROKER_HOST        "conduit.colinshirley.com"
#define MQTT_BROKER_PORT        8883
#define PROVISIONING_TOKEN      CONFIG_CONDUIT_PROVISIONING_TOKEN
#define CONDUIT_FW_VERSION      "1.0.0"

/* Embedded TLS certificate (ISRG Root X1) — see main/CMakeLists.txt EMBED_TXTFILES */
extern const uint8_t ca_cert_pem_start[] asm("_binary_ca_cert_pem_start");
extern const uint8_t ca_cert_pem_end[]   asm("_binary_ca_cert_pem_end");
