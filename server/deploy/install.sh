#!/bin/bash
set -euo pipefail

# =============================================================================
# Conduit server install script for Ubuntu 24.04 LTS
#
# Prerequisites (run on your local machine first):
#   make build-linux
#   scp server/bin/conduit-linux  user@server:/opt/conduit/bin/conduit
#   scp server/.env               user@server:/opt/conduit/.env
#   scp -r server/deploy/         user@server:/opt/conduit/deploy/
#
# Then on the server:
#   sudo bash /opt/conduit/deploy/install.sh
# =============================================================================

# -----------------------------------------------------------------------------
# Configuration
# -----------------------------------------------------------------------------
CONDUIT_HOME=/opt/conduit
DOMAIN=conduit.colinshirley.com
CERTBOT_EMAIL=colin@colinshirley.com   # used for Let's Encrypt expiry notices

# -----------------------------------------------------------------------------
# Helpers
# -----------------------------------------------------------------------------
log() { echo ""; echo "==> $*"; }
die() { echo "ERROR: $*" >&2; exit 1; }

[[ $EUID -eq 0 ]] || die "Run as root: sudo bash deploy/install.sh"

[[ -f "$CONDUIT_HOME/bin/conduit" ]] \
    || die "Binary not found at $CONDUIT_HOME/bin/conduit. Run 'make build-linux' and scp it first."

[[ -f "$CONDUIT_HOME/.env" ]] \
    || die ".env not found at $CONDUIT_HOME/.env"

# Load .env so MQTT_PASSWORD etc. are available
set -a; source "$CONDUIT_HOME/.env"; set +a

[[ -n "${MQTT_PASSWORD:-}" ]] \
    || die "MQTT_PASSWORD is not set in .env"

# -----------------------------------------------------------------------------
# 1. Install packages
# -----------------------------------------------------------------------------
log "Installing packages..."
apt-get update -q
apt-get install -y -q nginx certbot python3-certbot-nginx
apt-get install ca-certificates curl
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
chmod a+r /etc/apt/keyrings/docker.asc

tee /etc/apt/sources.list.d/docker.sources <<EOF
Types: deb
URIs: https://download.docker.com/linux/ubuntu
Suites: $(. /etc/os-release && echo "${UBUNTU_CODENAME:-$VERSION_CODENAME}")
Components: stable
Signed-By: /etc/apt/keyrings/docker.asc
EOF

apt-get update -q
apt-get install -y -q docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

systemctl enable --now docker nginx


# -----------------------------------------------------------------------------
# 2. Firewall
# -----------------------------------------------------------------------------
# log "Configuring firewall..."
# ufw allow 22/tcp
# ufw allow 80/tcp
# ufw allow 443/tcp
# ufw allow 8883/tcp
# ufw --force enable

# -----------------------------------------------------------------------------
# 3. Create service account and set permissions
# -----------------------------------------------------------------------------
log "Creating conduit user..."
id -u conduit &>/dev/null || useradd -r -s /sbin/nologin conduit

log "Setting permissions..."
chown -R conduit:conduit "$CONDUIT_HOME"
chmod +x "$CONDUIT_HOME/bin/conduit"

# -----------------------------------------------------------------------------
# 4. Obtain TLS certificate
# -----------------------------------------------------------------------------
if [[ -f "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" ]]; then
    log "Certificate for $DOMAIN already exists, skipping."
else
    log "Obtaining Let's Encrypt certificate for $DOMAIN..."
    systemctl stop nginx
    certbot certonly --standalone \
        -d "$DOMAIN" \
        --non-interactive \
        --agree-tos \
        --email "$CERTBOT_EMAIL"
    systemctl start nginx
fi

# -----------------------------------------------------------------------------
# 5. Copy certs for Mosquitto
# -----------------------------------------------------------------------------
log "Copying certificates for Mosquitto..."
mkdir -p "$CONDUIT_HOME/deploy/mosquitto/certs"
cp "/etc/letsencrypt/live/$DOMAIN/fullchain.pem" "$CONDUIT_HOME/deploy/mosquitto/certs/"
cp "/etc/letsencrypt/live/$DOMAIN/privkey.pem"   "$CONDUIT_HOME/deploy/mosquitto/certs/"
chmod 644 "$CONDUIT_HOME/deploy/mosquitto/certs/fullchain.pem"
chmod 640 "$CONDUIT_HOME/deploy/mosquitto/certs/privkey.pem"

# -----------------------------------------------------------------------------
# 6. Configure nginx
# -----------------------------------------------------------------------------
log "Configuring nginx..."
cp "$CONDUIT_HOME/deploy/nginx/conduit.conf" /etc/nginx/sites-available/conduit
ln -sf /etc/nginx/sites-available/conduit /etc/nginx/sites-enabled/conduit
rm -f /etc/nginx/sites-enabled/default
nginx -t
systemctl reload nginx

# -----------------------------------------------------------------------------
# 7. Generate Mosquitto password file
# -----------------------------------------------------------------------------
log "Generating Mosquitto password file..."
docker run --rm \
    -v "$CONDUIT_HOME/deploy/mosquitto:/mosquitto/config" \
    eclipse-mosquitto:2 \
    mosquitto_passwd -c -b /mosquitto/config/passwd server "$MQTT_PASSWORD"

# -----------------------------------------------------------------------------
# 8. Start infrastructure (Postgres, Redis, Mosquitto)
# -----------------------------------------------------------------------------
log "Starting Docker infrastructure..."
docker compose -f "$CONDUIT_HOME/deploy/docker-compose.yml" up -d

log "Waiting for Postgres to be ready..."
until docker exec conduit-postgres pg_isready -U conduit -d conduit > /dev/null 2>&1; do
    printf '.'; sleep 1
done
echo ""

# -----------------------------------------------------------------------------
# 9. Install and start the Go server
#    RUN_MIGRATIONS=true in .env so it applies migrations on first boot.
# -----------------------------------------------------------------------------
log "Installing systemd service..."
cp "$CONDUIT_HOME/deploy/systemd/conduit-server.service" /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now conduit-server

log "Waiting for server to start..."
sleep 3
systemctl status conduit-server --no-pager

# -----------------------------------------------------------------------------
# 10. Install certbot renewal hook
#     Copies fresh certs to Mosquitto's dir and restarts the container after
#     each renewal. nginx reloads itself — no extra step needed for nginx.
# -----------------------------------------------------------------------------
log "Installing certbot renewal hook..."
cp "$CONDUIT_HOME/deploy/certbot-renew-hook.sh" \
   /etc/letsencrypt/renewal-hooks/deploy/conduit.sh
chmod +x /etc/letsencrypt/renewal-hooks/deploy/conduit.sh

log "Testing certbot auto-renewal..."
certbot renew --dry-run

# -----------------------------------------------------------------------------
# Done
# -----------------------------------------------------------------------------
echo ""
echo "============================================"
echo " Conduit install complete"
echo "============================================"
echo "  API:  https://$DOMAIN"
echo "  MQTT: tls://$DOMAIN:8883"
echo ""
echo " Check server logs:  journalctl -u conduit-server -f"
echo " Check docker infra: docker compose -f $CONDUIT_HOME/deploy/docker-compose.yml ps"
echo "============================================"
