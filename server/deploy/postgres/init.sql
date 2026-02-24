-- This script runs inside the timescale/timescaledb image on first boot.
-- The image pre-creates the POSTGRES_DB database, so we just need
-- to enable the required extensions.

CREATE EXTENSION IF NOT EXISTS timescaledb;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
