# История изменений

## 0.1.0

Первый публичный выпуск HydraBridge.

- HTTP API управления HR Neo `3.11.0-1`.
- OpenAPI-контракт и codegen Go-моделей.
- Raw и structured config API.
- Точечные byte-preserving patches для domain, GeoSite, CIDR и GeoIP.
- Targets, GeoData, import/export, diagnostics и runtime inspection.
- Backup, restore, audit, logs и service lifecycle.
- Local и live-router smoke-профили.
- Сборка на Go `1.26.3`, CI с `golangci-lint v2.12.2`.
- Установка в `/opt/etc/hrbridge/`, IPK и opkg-feed для GitHub Pages.
- Однострочный установщик для роутера.
- ИБ-hardening: SSRF-защита GeoData download, ограничение GeoData upload
  штатным каталогом, HTTP request limits и безопасный разбор GeoData protobuf.
