# HydraBridge Compatibility With HR Neo

[Русский](../HRNEO_PARITY.md)

HydraBridge `0.1.0` supports HR Neo `3.11.0-1`.

HydraBridge exposes the behavior already implemented by HR Neo. It does not
introduce a second routing model:

- `hrneo.conf`, `domain.conf`, and `ip.list` remain the source of truth.
- Client-friendly grouped views are read-only projections.
- Narrow writes preserve unrelated config bytes.
- Raw and structured bulk endpoints explicitly replace the selected file.
- Keenetic VPN membership, upstream failover, XRay, subscriptions, and
  monitoring remain outside the HydraBridge parity scope.

## Verified surface

- readiness, status, compatibility, and all 27 `hrneo.conf` keys;
- domain, GeoSite, CIDR, and GeoIP editing;
- target inventory, policies, `PolicyOrder`, and DirectRoute;
- GeoData files, tags, references, and validation;
- live ipsets, firewall rules, policy marks, and IP diagnostics;
- backups, restore, audit, logs, and HR Neo lifecycle actions;
- reversible live-router smoke profiles.

`SIGUSR1` refreshes runtime rules but does not reread configs, watchlists, CIDR,
GeoIP, GeoSite, or `PolicyOrder`. Config writes therefore require restart.
