# HydraBridge API Overview

[Русский](../API.md)

The complete machine-readable contract is available in
[../../api/openapi.yaml](../../api/openapi.yaml). Every endpoint except
`/api/v1/health` requires a bearer token.

## Main groups

| Group | Endpoints |
|-------|-----------|
| Status | `/health`, `/version`, `/status`, `/overview`, `/doctor`, `/compatibility` |
| Config | `/config/*`, `/views/*` |
| Targets | `/targets*` |
| GeoData | `/geodata/*` |
| Diagnostics | `/diagnostics/domain`, `/diagnostics/ip` |
| Runtime | `/runtime/*` |
| Lifecycle | `/service/{start|stop|restart|reload}` |
| Safety | `/backups*`, `/audit`, `/logs` |

Config reads return `ETag` and `X-Config-Revision`. Send `If-Match` with writes
to avoid overwriting another client update.

Rule creation uses a JSON request body. Rule deletion uses OpenAPI 3.0
compatible `kind` and `value` query parameters:

```text
DELETE /config/domains/targets/{target}/rules?kind={kind}&value={value}
DELETE /config/cidr/targets/{target}/rules?kind={kind}&value={value}
```

GeoData uploads are limited to the `geofile` directory next to `hrneo.conf`.
GeoData downloads accept HTTP(S) sources that resolve to public IP addresses.

IP diagnostics combines static CIDR matches, GeoIP prefixes, live ipset
membership, firewall rules, and RCI policy marks. It does not fabricate
DNS/CNAME/L7 history that HR Neo does not retain.
