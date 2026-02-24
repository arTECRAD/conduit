# Conduit

Build-your-own IoT monitoring platform built on ESP32-S3 hardware with a Go backend and Next.js web portal.

Current demo firmware monitors temperature, humidity, eCO2, TVOC, PM2.5, and PM10 via CCS811, DHT22, and SDS011 sensors.

> **Status:** Active development

---

## Components

### Firmware (`/firmware`)
ESP-IDF (C) firmware for the ESP32-S3 DevKitC-1. Handles sensor polling, WiFi provisioning via captive portal, MQTTS telemetry publishing, and OTA-ready partition layout. Devices self-register on first boot and receive MQTT credentials from the server.

### Server (`/server`)
Go backend built with Echo v4. Exposes a REST API for device management, user authentication, and telemetry queries. Subscribes to the Mosquitto MQTT broker to ingest sensor readings into PostgreSQL (TimescaleDB). Redis handles sessions and rate limiting. JWT access tokens + opaque refresh tokens for auth.

### Web Portal (`/web`)
Next.js 14 (App Router) dashboard for viewing device status, historical sensor charts, and managing account settings. TypeScript, Tailwind CSS, shadcn/ui, and TanStack Query.

---

## Architecture

```
ESP32-S3 ──MQTTS──▶ Mosquitto ──▶ Go Backend ──▶ PostgreSQL/TimescaleDB
                                       ▲                  ▲
                                  REST API            Redis
                                       ▲
                                  Next.js Web Portal
```

## Infrastructure

Runs on a single server (`conduit.colinshirley.com`). PostgreSQL + TimescaleDB, Redis, and Mosquitto run via Docker Compose. The Go binary runs as a systemd service.

See [`CLAUDE.md`](./CLAUDE.md) for full architecture details, API specs, and coding conventions.
