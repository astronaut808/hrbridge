#!/bin/sh

set -eu

FEED_CONF="/opt/etc/opkg/customfeeds.conf"
BASE_URL=${HRBRIDGE_FEED_BASE_URL:-"https://astronaut808.github.io/hrbridge/keenetic"}

if [ "$(id -u)" != "0" ]; then
  echo "HydraBridge must be installed as root."
  exit 1
fi

if ! command -v opkg >/dev/null 2>&1; then
  echo "opkg was not found. Install Entware before HydraBridge."
  exit 1
fi

echo "Detecting Entware architecture..."
ARCH=$(opkg print-architecture | awk '
  /^arch/ && $2 !~ /_kn$/ && $2 ~ /-[0-9]+\.[0-9]+$/ {
    print $2
    exit
  }'
)

case "$ARCH" in
  aarch64-3.10)
    FEED_URL="$BASE_URL/aarch64-k3.10"
    ;;
  mipsel-3.4)
    FEED_URL="$BASE_URL/mipselsf-k3.4"
    ;;
  mips-3.4)
    FEED_URL="$BASE_URL/mipssf-k3.4"
    ;;
  "")
    echo "Failed to detect Entware architecture."
    exit 1
    ;;
  *)
    echo "Unsupported Entware architecture: $ARCH"
    exit 1
    ;;
esac

echo "Architecture: $ARCH"
echo "HydraBridge feed: $FEED_URL"

mkdir -p "$(dirname "$FEED_CONF")"

if [ -f "$FEED_CONF" ] && grep -q '^src/gz hrbridge ' "$FEED_CONF" 2>/dev/null; then
  echo "Updating existing HydraBridge feed entry..."
  sed -i '/^src\/gz hrbridge /d' "$FEED_CONF"
fi

printf 'src/gz hrbridge %s\n' "$FEED_URL" >> "$FEED_CONF"

echo "Updating package lists..."
opkg update

echo "Installing HydraBridge..."
opkg install hrbridge

echo "Starting HydraBridge..."
/opt/etc/init.d/S99hrbridge stop >/dev/null 2>&1 || true
/opt/etc/init.d/S99hrbridge start

echo ""
echo "HydraBridge is installed."
echo "Bearer token:"
sed -n 's/^authToken=//p' /opt/etc/HydraRoute/hrbridge.conf
