# HydraBridge Installation

[Русский](../INSTALL.md)

## Requirements

- Keenetic router with Entware.
- Installed and configured HR Neo `3.11.0-1`.
- SSH access to the router.
- Go `1.26.3` or a newer `1.26.x` patch release for local builds.

## Automated opkg installation

On the router:

```sh
opkg update && opkg install curl && \
  curl -fsSL "https://astronaut808.github.io/hrbridge/keenetic/install.sh" | sh
```

The installer detects the Entware architecture, adds the HydraBridge feed to
`/opt/etc/opkg/customfeeds.conf`, installs the matching package, starts the
service, and prints the bearer token.

To upgrade HydraBridge later:

```sh
opkg update
opkg upgrade hrbridge
/opt/etc/init.d/S99hrbridge restart
```

## Local build

For ARM64:

```sh
make aarch64
```

For MIPS little-endian or big-endian:

```sh
make mipsel
make mips
```

## Copy and run

If the router OpenSSH installation has no SFTP server, use legacy SCP:

```sh
scp -O -P 222 build/hrbridge-aarch64 root@192.168.1.1:/tmp/hrbridge
```

On the router:

```sh
mkdir -p /opt/etc/hrbridge
mv /tmp/hrbridge /opt/etc/hrbridge/hrbridge
chmod +x /opt/etc/hrbridge/hrbridge

/opt/etc/hrbridge/hrbridge \
  -config /opt/etc/HydraRoute/hrbridge.conf
```

HydraBridge generates and persists a random bearer token on first startup:

```sh
sed -n 's/^authToken=//p' /opt/etc/HydraRoute/hrbridge.conf
```

Check the API from your workstation:

```sh
export ROUTER=192.168.1.1:2080
export TOKEN='your-token'

curl -s "http://$ROUTER/api/v1/health" | jq
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://$ROUTER/api/v1/doctor" | jq
```

For local IPK packages, run:

```sh
make package
```

Copy and install the matching package:

```sh
scp -O -P 222 build/hrbridge_0.1.0_aarch64-3.10.ipk \
  root@192.168.1.1:/tmp/hrbridge.ipk

ssh -p 222 root@192.168.1.1
opkg install /tmp/hrbridge.ipk
/opt/etc/init.d/S99hrbridge start
```

Generate a static opkg feed for GitHub Pages with:

```sh
make feed
```
