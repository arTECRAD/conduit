#!/bin/bash
# Certbot deploy hook — runs after every successful cert renewal.
# Install to: /etc/letsencrypt/renewal-hooks/deploy/conduit.sh

set -e

CERT_DIR=/etc/letsencrypt/live/conduit.colinshirley.com
DEST_DIR=/opt/conduit/deploy/mosquitto/certs

mkdir -p "$DEST_DIR"
cp "$CERT_DIR/fullchain.pem" "$DEST_DIR/fullchain.pem"
cp "$CERT_DIR/privkey.pem"   "$DEST_DIR/privkey.pem"
chmod 644 "$DEST_DIR/fullchain.pem"
chmod 640 "$DEST_DIR/privkey.pem"

# Restart Mosquitto to reload the new certs
docker compose -f /opt/conduit/deploy/docker-compose.yml restart mosquitto

# Reload nginx (zero-downtime)
systemctl reload nginx
