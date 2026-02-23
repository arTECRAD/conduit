# CLAUDE.md — Conduit IoT Platform

## Project Overview

Conduit is a consumer-grade indoor air quality monitoring platform. The system consists of ESP32-S3-based sensor devices, a Go backend server, an MQTT broker, and a Next.js web portal. The hardware measures temperature, humidity, eCO2, TVOC, PM2.5, and PM10 using CCS811, DHT22, and SDS011 sensors.

This is a monorepo. All components live under a single repository with the following top-level structure:

```
/
├── firmware/          # ESP-IDF firmware for ESP32-S3
├── server/            # Go backend (REST API + MQTT ingestion)
├── web/               # Next.js web portal
├── docs/              # Architecture docs, API specs, schemas
├── deploy/            # Docker Compose, Mosquitto config, systemd units
├── certs/             # TLS certificates and CA (gitignored, generated locally)
└── CLAUDE.md
```

---

## Architecture Summary

```
┌──────────────┐   MQTTS (8883)    ┌─────────────┐
│  ESP32-S3    │──────────────────▶│  Mosquitto   │
│  (sensors)   │                   │  (MQTT broker)│
└──────┬───────┘                   └──────┬───────┘
       │ HTTPS (REST)                     │ Internal subscribe
       ▼                                  ▼
┌──────────────────────────────────────────────────┐
│                Go Backend (Echo)                  │
│  ┌────────────┐ ┌──────────────┐ ┌─────────────┐│
│  │ REST API   │ │ MQTT Ingester│ │ Auth (JWT)  ││
│  │ (devices,  │ │ (telemetry,  │ │ (access +   ││
│  │  users,    │ │  health,     │ │  refresh)   ││
│  │  portal)   │ │  commands)   │ │             ││
│  └─────┬──────┘ └──────┬───────┘ └─────────────┘│
│        │               │                         │
│        ▼               ▼                         │
│  ┌──────────────────────────┐  ┌───────────────┐│
│  │ PostgreSQL + TimescaleDB │  │     Redis     ││
│  │ (users, devices, config, │  │ (sessions,    ││
│  │  time-series telemetry)  │  │  device status,││
│  │                          │  │  rate limiting)││
│  └──────────────────────────┘  └───────────────┘│
└──────────────────────────────────────────────────┘
       ▲
       │ HTTPS
┌──────┴───────┐
│  Next.js     │
│  Web Portal  │
└──────────────┘
```

### Communication Protocols

- **ESP ↔ Server (telemetry, commands):** MQTT over TLS (MQTTS, port 8883) via Mosquitto broker
- **ESP → Server (registration):** HTTPS REST
- **Web Portal ↔ Server:** HTTPS REST API
- **iOS App ↔ Server (future):** Same HTTPS REST API

### MQTT Topic Structure

```
devices/{device_id}/telemetry       # ESP publishes sensor readings
devices/{device_id}/health          # ESP publishes heartbeat/diagnostics
devices/{device_id}/command         # ESP subscribes; server publishes config changes
devices/{device_id}/command/ack     # ESP publishes command acknowledgements
```

Mosquitto ACLs enforce that each device can only access its own topic subtree.

---

## Server (`/server`)

### Tech Stack

| Component        | Technology                          |
|------------------|-------------------------------------|
| Language         | Go 1.22+                           |
| HTTP Framework   | Echo v4                             |
| Database Driver  | pgx v5 (direct, no ORM)            |
| Query Gen        | sqlc (type-safe SQL → Go)           |
| Migrations       | golang-migrate                      |
| MQTT Client      | eclipse/paho.mqtt.golang            |
| Auth             | JWT (golang-jwt/jwt/v5)             |
| Password Hashing | bcrypt                              |
| Redis Client     | go-redis/redis/v9                   |
| Config           | Environment variables via envconfig  |
| Logging          | slog (stdlib structured logging)    |
| Testing          | stdlib testing + testify            |
| Linting          | golangci-lint                       |

### Project Layout

Follow the standard Go project layout:

```
server/
├── cmd/
│   └── conduit/
│       └── main.go              # Entrypoint: wires up Echo, MQTT, DB, Redis
├── internal/
│   ├── config/                  # Env var parsing, app configuration struct
│   ├── handler/                 # HTTP handlers grouped by domain
│   │   ├── auth.go              # Register, login, refresh, logout
│   │   ├── device.go            # Device registration, claiming, CRUD
│   │   ├── telemetry.go         # Query sensor data endpoints
│   │   └── user.go              # User profile, settings
│   ├── middleware/               # Echo middleware (auth, rate limiting, logging)
│   ├── model/                   # Go structs for domain objects
│   ├── repository/              # Database access layer (sqlc-generated + custom)
│   ├── mqtt/                    # MQTT client, subscription handlers, command publisher
│   ├── service/                 # Business logic layer between handlers and repositories
│   └── auth/                    # JWT generation, validation, refresh logic
├── migrations/                  # SQL migration files (golang-migrate format)
│   ├── 000001_create_users.up.sql
│   ├── 000001_create_users.down.sql
│   └── ...
├── queries/                     # sqlc query definitions (.sql files)
├── sqlc.yaml                    # sqlc configuration
├── go.mod
├── go.sum
└── Makefile
```

### Coding Conventions (Go)

- **Error handling:** Always check and handle errors explicitly. Never use `_` to discard errors except in deferred `.Close()` calls. Wrap errors with `fmt.Errorf("context: %w", err)` to preserve the error chain.
- **Naming:** Use standard Go naming. Exported types are PascalCase. Unexported are camelCase. Acronyms are all-caps (e.g., `DeviceID`, `MQTT`, `HTTPHandler`).
- **Handlers:** Each handler receives an `echo.Context`, extracts and validates input, calls a service method, and returns a JSON response. Handlers must not contain business logic or direct database calls.
- **Services:** Contain business logic. Accept and return domain model types, not Echo-specific types. Services call repositories for data access.
- **Repositories:** Thin data access layer. Primarily sqlc-generated code. Custom queries go in the same package with clear naming.
- **Context propagation:** Always pass `context.Context` through the call chain (from handler → service → repository). Use the Echo request context.
- **Dependency injection:** Use constructor functions (e.g., `NewDeviceService(repo, mqttClient)`) and wire dependencies in `main.go`. No global state, no init() functions with side effects.
- **Logging:** Use `slog` with structured fields. Log at appropriate levels: `Info` for request lifecycle events, `Warn` for recoverable issues, `Error` for failures that need attention.
- **Tests:** Table-driven tests are preferred for handlers and services. Use testify for assertions. Database tests use a real test database (not mocks) with transaction rollback.

### Database Schema Conventions

- **Primary keys:** Use UUIDs (`gen_random_uuid()`) for all user-facing entities. Use `BIGSERIAL` only for internal-only tables or hypertable time-series rows.
- **Timestamps:** All tables include `created_at TIMESTAMPTZ NOT NULL DEFAULT now()` and `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`. Sensor data uses a `recorded_at TIMESTAMPTZ` column as the hypertable time dimension.
- **Naming:** Snake_case for all table and column names. Plural table names (`users`, `devices`, `sensor_readings`).
- **Enums:** Use PostgreSQL enum types for finite sets (e.g., `device_status` with values `pending`, `active`, `disabled`).
- **Foreign keys:** Always defined with `ON DELETE` behavior explicitly stated.
- **Indexes:** Create indexes for any column used in WHERE clauses or JOIN conditions. TimescaleDB hypertables get composite indexes on `(device_id, recorded_at DESC)`.

### API Conventions

- **Base path:** `/api/v1/`
- **Device endpoints (authenticated, user-facing):**
  - `POST   /api/v1/devices/claim`         — Claim a pending device by setup code
  - `GET    /api/v1/devices`                — List user's devices
  - `GET    /api/v1/devices/:id`            — Get device details
  - `PATCH  /api/v1/devices/:id`            — Update device settings (name, polling interval)
  - `DELETE /api/v1/devices/:id`            — Remove device from account
  - `GET    /api/v1/devices/:id/telemetry`  — Query sensor readings (supports time range, resolution)
- **Device endpoints (device-facing, provisioning token auth):**
  - `POST   /api/v1/devices/register`       — Device self-registration on first boot
- **Auth endpoints:**
  - `POST   /api/v1/auth/register`          — Create user account
  - `POST   /api/v1/auth/login`             — Login, returns access + refresh tokens
  - `POST   /api/v1/auth/refresh`           — Exchange refresh token for new access token
  - `POST   /api/v1/auth/logout`            — Invalidate refresh token
- **Response format:** All responses use a consistent envelope:
  ```json
  { "data": { ... }, "error": null }
  { "data": null, "error": { "code": "DEVICE_NOT_FOUND", "message": "..." } }
  ```
- **Pagination:** Cursor-based using `?cursor=<opaque_token>&limit=<int>` for list endpoints.
- **Telemetry queries:** Support `?from=<ISO8601>&to=<ISO8601>&resolution=<raw|5m|1h|1d>` parameters. Raw returns individual readings; aggregated resolutions return avg, min, max per window.

### Authentication & Security

- **User auth:** JWT access tokens (15 min expiry) + opaque refresh tokens (30 day expiry, stored hashed in DB). Access tokens are stateless; refresh tokens are server-validated.
- **Device auth:** Pre-shared provisioning token compiled into firmware for the registration endpoint. Post-registration, each device receives unique MQTT credentials (username + password).
- **MQTT security:** Mosquitto uses TLS (server cert from self-signed CA). Device authentication via username/password. ACLs restrict each device to its own topic subtree. Plan for mTLS (client certificates) in Phase 5.
- **Password storage:** bcrypt with cost factor 12.
- **Rate limiting:** Redis-backed, applied via Echo middleware. Separate limits for auth endpoints (stricter) vs general API.
- **CORS:** Allow web portal origin only. Credentials mode enabled for cookie-based refresh tokens.

---

## Firmware (`/firmware`)

### Tech Stack

| Component        | Technology                          |
|------------------|-------------------------------------|
| Framework        | ESP-IDF v5.x                        |
| Language         | C                                   |
| Target           | ESP32-S3 (DevKitC-1)                |
| MQTT             | esp-mqtt (ESP-IDF component)        |
| TLS              | esp-tls (mbedTLS under the hood)    |
| HTTP Client      | esp_http_client                     |
| WiFi             | esp_wifi (station + AP modes)       |
| Storage          | NVS (non-volatile storage)          |
| OTA (future)     | esp_ota_ops                         |

### Sensors

| Sensor  | Interface | Measures             | Library/Driver                     |
|---------|-----------|----------------------|------------------------------------|
| CCS811  | I2C       | eCO2, TVOC           | Custom or community I2C driver     |
| DHT22   | GPIO      | Temperature, Humidity | ESP-IDF RMT-based or GPIO driver   |
| SDS011  | UART      | PM2.5, PM10          | Custom UART driver                 |

### Project Layout

```
firmware/
├── CMakeLists.txt
├── sdkconfig.defaults          # Default ESP-IDF Kconfig settings
├── partitions.csv              # Partition table (NVS, OTA_0, OTA_1, factory)
├── main/
│   ├── CMakeLists.txt
│   ├── main.c                  # app_main(): init NVS, check provisioning, start tasks
│   ├── wifi_manager.c/.h       # Station connect + AP mode + captive portal
│   ├── provisioning.c/.h       # Captive portal HTTP server, NVS credential storage
│   ├── mqtt_client.c/.h        # MQTT connection, publish, subscribe, reconnect
│   ├── http_client.c/.h        # HTTPS registration call to server
│   ├── sensor_task.c/.h        # FreeRTOS task: polls all sensors on interval
│   ├── command_handler.c/.h    # Parses MQTT commands, applies config, sends ACKs
│   ├── device_config.c/.h      # NVS read/write for all config (interval, device ID, etc)
│   ├── led_status.c/.h         # Status LED patterns (booting, AP mode, connected, error)
│   └── telemetry.c/.h          # Formats sensor data into JSON for MQTT publish
├── components/
│   ├── ccs811/                 # CCS811 I2C driver component
│   ├── dht22/                  # DHT22 driver component
│   └── sds011/                 # SDS011 UART driver component
└── certs/
    └── ca_cert.pem             # Root CA cert for TLS validation (embedded in firmware)
```

### Coding Conventions (C / ESP-IDF)

- **Task architecture:** Each major subsystem runs as a dedicated FreeRTOS task: sensor polling, MQTT client, WiFi manager. Tasks communicate via FreeRTOS queues and event groups, not shared global variables.
- **Error handling:** Check every `esp_err_t` return value. Use `ESP_ERROR_CHECK()` only during init for fatal errors. In tasks, log errors and attempt recovery (reconnect, retry with backoff).
- **Logging:** Use `ESP_LOGI`, `ESP_LOGW`, `ESP_LOGE` with a per-file `TAG` constant.
- **Naming:** Functions: `module_action_noun` (e.g., `wifi_manager_start_ap`, `sensor_task_read_all`). Constants/defines: `SCREAMING_SNAKE_CASE`. Structs: `module_name_t` (e.g., `sensor_reading_t`).
- **Memory:** Prefer stack allocation. When heap allocation is needed, always check for NULL and free on all exit paths. Never use `malloc` directly; use ESP-IDF's `heap_caps_malloc` when memory type matters.
- **NVS keys:** Namespace `conduit`. Keys: `wifi_ssid`, `wifi_pass`, `device_id`, `setup_code`, `provisioned`, `poll_interval_ms`, `mqtt_user`, `mqtt_pass`.
- **JSON:** Use cJSON (included in ESP-IDF) for building telemetry payloads and parsing command payloads.

### Device Provisioning Flow

1. Boot → check NVS for `provisioned` flag
2. If unprovisioned:
   - Generate setup code: deterministic 8-char alphanumeric (`XXXX-XXXX`) derived from hash of MAC address
   - Start WiFi AP mode with SSID `CDT-XXXX-XXXX`
   - Run captive portal HTTP server serving a single WiFi config page
   - User connects, submits SSID + password
   - Save credentials to NVS, reboot
3. Connect to WiFi in station mode
4. POST to `https://<server>/api/v1/devices/register` with device ID + setup code + provisioning token
5. Server returns MQTT credentials and status (`pending_claim` or `active`)
6. Connect to MQTT broker, subscribe to `devices/{device_id}/command`
7. If status is `pending_claim`: stay connected but do not publish telemetry; wait for activation command
8. On activation command: save `provisioned = true` to NVS, begin telemetry publishing
9. On subsequent boots: skip AP mode, connect WiFi, connect MQTT, resume telemetry

### Telemetry Payload Format

```json
{
  "device_id": "a1b2c3d4",
  "timestamp": 1700000000,
  "sensors": {
    "temperature_c": 23.5,
    "humidity_pct": 45.2,
    "eco2_ppm": 412,
    "tvoc_ppb": 15,
    "pm25_ugm3": 8.3,
    "pm10_ugm3": 12.1
  }
}
```

### Command Payload Format

```json
{
  "command_id": "uuid-here",
  "type": "set_poll_interval",
  "payload": {
    "interval_ms": 30000
  }
}
```

Supported command types: `set_poll_interval`, `factory_reset`, `reboot`, `activate`, `update_mqtt_credentials`.

---

## Web Portal (`/web`)

### Tech Stack

| Component        | Technology                          |
|------------------|-------------------------------------|
| Framework        | Next.js 14+ (App Router)            |
| Language         | TypeScript                          |
| Styling          | Tailwind CSS v4                     |
| Component Library| shadcn/ui                           |
| Charts           | Recharts                            |
| HTTP Client      | Fetch API (native) or TanStack Query|
| State Management | TanStack Query (server state) + React state (UI) |
| Auth             | JWT stored in httpOnly cookies (via Next.js API route proxy) |
| Form Handling    | React Hook Form + Zod validation    |
| Linting          | ESLint + Prettier                   |

### Project Layout

```
web/
├── src/
│   ├── app/                    # Next.js App Router pages
│   │   ├── layout.tsx          # Root layout with nav, providers
│   │   ├── page.tsx            # Landing / redirect to dashboard
│   │   ├── (auth)/
│   │   │   ├── login/page.tsx
│   │   │   └── register/page.tsx
│   │   ├── dashboard/
│   │   │   ├── page.tsx        # Device list overview
│   │   │   └── devices/
│   │   │       ├── [id]/page.tsx       # Device detail + charts
│   │   │       ├── [id]/settings/page.tsx
│   │   │       └── add/page.tsx        # Claim device by setup code
│   │   └── settings/
│   │       └── page.tsx        # User account settings
│   ├── components/
│   │   ├── ui/                 # shadcn/ui components (generated)
│   │   ├── charts/             # Recharts wrappers for sensor data
│   │   ├── devices/            # Device card, status badge, etc
│   │   ├── layout/             # Nav, sidebar, header
│   │   └── auth/               # Login form, register form
│   ├── lib/
│   │   ├── api.ts              # API client (fetch wrappers, base URL, error handling)
│   │   ├── auth.ts             # Token management, auth helpers
│   │   └── utils.ts            # General utilities
│   ├── hooks/                  # Custom React hooks (useDevices, useTelemetry, etc)
│   ├── types/                  # TypeScript type definitions matching API responses
│   └── styles/
│       └── globals.css         # Tailwind imports + glassmorphism utility classes
├── public/                     # Static assets
├── next.config.js
├── tailwind.config.ts
├── tsconfig.json
├── package.json
└── .eslintrc.json
```

### Coding Conventions (TypeScript / React)

- **Components:** Functional components only. Use `"use client"` directive only on components that need client-side interactivity. Default to server components.
- **Naming:** Components: PascalCase files and exports. Hooks: `use` prefix (e.g., `useDeviceTelemetry`). Utility functions: camelCase. Types/interfaces: PascalCase with descriptive names (e.g., `DeviceListResponse`, `SensorReading`).
- **Data fetching:** Use TanStack Query for all server state. Define query keys in a central `queryKeys.ts` file. Mutations use `useMutation` with optimistic updates where appropriate.
- **Forms:** React Hook Form with Zod schemas for validation. Define Zod schemas alongside the form component or in a shared `schemas/` directory if reused.
- **Error handling:** All API calls must handle errors gracefully. Show user-facing error messages via toast notifications (shadcn/ui Sonner). Log detailed errors to console in development.
- **TypeScript:** Strict mode enabled. No `any` types. Define types for all API request/response shapes.
- **Styling:** Tailwind utility classes only. No custom CSS except for the glassmorphism base classes in `globals.css`. Follow the design system below.

### Design System

- **Overall aesthetic:** Modern glassmorphism with clean typography. Inspired by Apple's visionOS / macOS Sequoia aesthetic. Dark mode primary, light mode supported.
- **Glass panels:** Use `backdrop-blur-xl bg-white/5 border border-white/10 rounded-2xl` for card surfaces in dark mode. Light mode equivalent: `backdrop-blur-xl bg-white/70 border border-black/5`.
- **Colors:** Neutral grays for backgrounds. Accent colors for sensor data: blue for temperature, cyan for humidity, green for good air quality, yellow for moderate, red for poor. Use CSS custom properties for theme values.
- **Typography:** System font stack (`font-sans` in Tailwind). Headings are `font-semibold`. Body text is `font-normal`. Use `text-muted-foreground` from shadcn for secondary text.
- **Spacing:** Follow Tailwind's default spacing scale. Cards use `p-6`. Page sections use `space-y-6`.
- **Animations:** Subtle transitions only. Use `transition-all duration-200` for hover states. No flashy animations.
- **Responsive:** Mobile-first. Dashboard uses a sidebar on desktop (`lg:` breakpoint) that collapses to a bottom nav or hamburger on mobile.

---

## Deploy (`/deploy`)

### Infrastructure

```
deploy/
├── docker-compose.yml          # PostgreSQL, TimescaleDB, Redis, Mosquitto
├── mosquitto/
│   ├── mosquitto.conf          # Listener, TLS, auth, ACL config
│   ├── acl.conf                # Per-device topic ACL rules
│   └── passwd                  # Mosquitto password file (generated)
├── postgres/
│   └── init.sql                # Enable TimescaleDB extension, create DB
└── systemd/
    └── conduit-server.service # systemd unit for the Go binary
```

### Runtime Dependencies

- **PostgreSQL 16** with **TimescaleDB** extension
- **Redis 7+**
- **Mosquitto 2.x** with TLS and password-file authentication
- All infrastructure runs in Docker Compose for development; Go binary runs directly on host (or in its own container for production)

### Environment Variables (server)

```
DATABASE_URL=postgres://conduit:password@localhost:5432/conduit?sslmode=disable
REDIS_URL=redis://localhost:6379/0
MQTT_BROKER_URL=tls://localhost:8883
MQTT_USERNAME=server
MQTT_PASSWORD=<server-mqtt-password>
JWT_SECRET=<random-256-bit-key>
PROVISIONING_TOKEN=<shared-token-for-device-registration>
SERVER_PORT=8080
CORS_ORIGIN=https://conduit.yourdomain.com
TLS_CA_CERT_PATH=../certs/ca.pem
TLS_SERVER_CERT_PATH=../certs/server.pem
TLS_SERVER_KEY_PATH=../certs/server-key.pem
```

---

## Data Retention Policy

- **Raw sensor readings:** Retained for 30 days
- **5-minute aggregates:** Retained for 6 months (TimescaleDB continuous aggregate)
- **1-hour aggregates:** Retained indefinitely (TimescaleDB continuous aggregate)
- **Aggregates store:** avg, min, max for each metric per window

---

## Conventions for Claude Code

### General

- When generating code, always follow the project layout and conventions defined above.
- Do not introduce new dependencies without explicit approval. If a new library would be beneficial, state the case before adding it.
- Prefer simplicity. If something can be done with stdlib or the existing stack, prefer that over adding a dependency.
- Always include error handling. Never leave errors unchecked or silently swallowed.
- Write tests alongside implementation. At minimum, write tests for business logic in the service layer and for non-trivial handlers.

### Go-Specific

- Run `go vet ./...` and `golangci-lint run` before considering any Go code complete.
- All SQL queries go through sqlc. Do not write raw SQL in Go code outside of migration files and sqlc query files.
- Use `context.Context` everywhere. Never use `context.Background()` in request-handling code.
- Struct field tags: `json:"snake_case"` for API responses, `db:"snake_case"` for database mapping.

### TypeScript-Specific

- Run `npx tsc --noEmit` and `npx eslint .` before considering any TypeScript code complete.
- No default exports except for Next.js pages (which require them).
- All API response types must be defined in `types/` and shared between hooks and components.

### ESP-IDF-Specific

- Build with `idf.py build`. Ensure no compiler warnings (`-Werror` is enabled in sdkconfig).
- All config values that persist across reboots must go through NVS with the `conduit` namespace.
- Never block the main task. All I/O and waiting must happen in dedicated FreeRTOS tasks.
- Pin sensor-polling task to core 1 to avoid contention with WiFi/MQTT on core 0.