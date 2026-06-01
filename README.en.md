# HydraBridge

[Русский](README.md)

HydraBridge is a unified HTTP control API for
[HydraRoute Neo](https://github.com/Ground-Zerro/HydraRoute).

It is a small agent for Keenetic routers with Entware. HydraBridge runs next to
`hrneo`, but does not route traffic itself. It exposes the existing HR Neo
surface to native apps, bots, CLI tools, automations, and web UIs.

## Status

| Component | Version |
|-----------|---------|
| HydraBridge | `0.1.0` |
| Supported HR Neo | `3.11.0-1` |
| API | `/api/v1` |

License: [MIT](LICENSE).

## Guarantees

- `hrneo.conf`, `domain.conf`, and `ip.list` remain the source of truth.
- HydraBridge does not introduce a second routing model.
- Client-friendly grouped views are read-only projections.
- Narrow writes preserve unrelated config bytes.
- Writes create backups and support optimistic concurrency with `If-Match`.

## Documentation

Russian documentation is primary:

| Document | Purpose |
|----------|---------|
| [Installation](docs/INSTALL.md) | opkg, manual installation, upgrades, and first run |
| [API](docs/API.md) | Endpoint groups and client rules |
| [HR Neo compatibility](docs/HRNEO_PARITY.md) | Supported HR Neo `3.11.0-1` boundary |
| [Security](SECURITY.md) | Access model and vulnerability reports |
| [Development](docs/DEVELOPMENT.md) | Build, packaging, and local checks |
| [Contributing](CONTRIBUTING.md) | Build, checks, and change rules |

Additional English references:

- [Installation](docs/en/INSTALL.md)
- [API overview](docs/en/API.md)
- [HR Neo compatibility](docs/en/HRNEO_PARITY.md)
- [OpenAPI contract](api/openapi.yaml)
