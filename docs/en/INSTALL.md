# HydraBridge Installation

[Русский](../INSTALL.md)

## Requirements

- Keenetic router with Entware.
- Installed and configured HR Neo `3.11.0-1`.
- Root access to the Entware shell.

## Install with opkg

Run on the router:

```sh
opkg update && opkg install curl && \
  curl -fsSL "https://astronaut808.github.io/hrbridge/keenetic/install.sh" | sh
```

The installer detects the Entware architecture, configures the HydraBridge
feed, installs the package, starts the service, and prints the bearer token.

## Upgrade

```sh
opkg update
opkg upgrade hrbridge
/opt/etc/init.d/S99hrbridge restart
```

## Full Removal

These commands remove HydraBridge only. HR Neo configuration and data under
`/opt/etc/HydraRoute` are left intact.

Run on the router:

```sh
/opt/etc/init.d/S99hrbridge stop 2>/dev/null || true
opkg remove hrbridge
rm -rf /opt/etc/hrbridge
rm -f /opt/etc/init.d/S99hrbridge
rm -f /opt/var/log/hrbridge-audit.log
sed -i '/^src\/gz hrbridge /d' /opt/etc/opkg/customfeeds.conf
opkg update
```

Skip the `sed -i` line if you want to keep the feed configured for future
installs.

## Read the token

```sh
sed -n 's/^authToken=//p' /opt/etc/hrbridge/hrbridge.conf
```

## Check the API

```sh
export ROUTER=192.168.1.1:2080
export TOKEN='your-token'

curl -s "http://$ROUTER/api/v1/health" | jq
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://$ROUTER/api/v1/doctor" | jq
```

The primary Russian [installation guide](../INSTALL.md) also documents manual
IPK installation. Local build and feed generation are documented in
[../DEVELOPMENT.md](../DEVELOPMENT.md).
