# HydraBridge

[Русский](README.md)

HydraBridge is a unified HTTP control API for
[HydraRoute Neo](https://github.com/Ground-Zerro/HydraRoute).

It is a small agent for Keenetic routers with Entware. HydraBridge runs next to
`hrneo`, but does not route traffic itself. It exposes the existing HR Neo
surface to native apps, bots, CLI tools, automations, and new web UIs.

## Status

- HydraBridge version: `0.1.0`
- Supported HR Neo version: `3.11.0-1`
- API prefix: `/api/v1`
- OpenAPI contract: [api/openapi.yaml](api/openapi.yaml)
- License: must be selected before the public release

## Core guarantees

- `hrneo.conf`, `domain.conf`, and `ip.list` remain the source of truth.
- HydraBridge does not introduce a second routing model.
- Client-friendly grouped views are read-only projections.
- Narrow rule patches preserve unrelated config bytes.
- Writes create backups and support optimistic concurrency with `If-Match`.

## Quick start

On a router with HR Neo already installed:

```sh
opkg update && opkg install curl && \
  curl -fsSL "https://astronaut808.github.io/hrbridge/keenetic/install.sh" | sh
```

The installer adds the HydraBridge opkg feed, installs the package matching the
router architecture, starts the service, and prints the bearer token.

### Local build

Use Go `1.26.3` or a newer `1.26.x` patch release.

Build for a Keenetic ARM64 router:

```sh
make aarch64
```

Run on the router:

```sh
/opt/etc/hrbridge/hrbridge \
  -config /opt/etc/HydraRoute/hrbridge.conf
```

HydraBridge generates and persists a random bearer token on first startup.
Read it on the router:

```sh
sed -n 's/^authToken=//p' /opt/etc/HydraRoute/hrbridge.conf
```

Check the API:

```sh
export ROUTER=192.168.1.1:2080
export TOKEN='your-token'

curl -s "http://$ROUTER/api/v1/health" | jq
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://$ROUTER/api/v1/overview" | jq
```

Do not expose the default HTTP listener to the internet. See
[SECURITY.md](SECURITY.md).

## Documentation

Russian documentation is primary:

- [Installation](docs/INSTALL.md)
- [API overview](docs/API.md)
- [HR Neo compatibility boundary](docs/HRNEO_PARITY.md)
- [Public release checklist](docs/RELEASE.md)

Additional English documents:

- [Installation](docs/en/INSTALL.md)
- [API overview](docs/en/API.md)
- [HR Neo compatibility boundary](docs/en/HRNEO_PARITY.md)

The complete machine-readable API contract is available in
[api/openapi.yaml](api/openapi.yaml).
