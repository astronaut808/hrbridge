# Карта сценариев HydraBridge

Документ помогает оценить API как основу для iOS-приложения, Telegram-бота,
нового web UI, CLI и автоматизаций.

## Реализовано

| Область | Возможности |
|---------|-------------|
| Доступ | Bearer token, ротация токена, health, version, capabilities |
| Статус | HR Neo process status, пути, dashboard overview, Doctor, compatibility |
| `hrneo.conf` | Raw и structured read/write, 27 параметров, defaults preview и generation |
| `domain.conf` | Raw и structured read/write, validation, grouped view, точечные domain и GeoSite patches |
| `ip.list` | Raw и structured read/write, validation, grouped view, точечные CIDR и GeoIP patches |
| Targets | Inventory, interface list, Keenetic policy list/create/delete, `PolicyOrder` |
| GeoData | Files, upload, download, references, точный tag index, validation |
| Import/export | CSV export, CSV preview/apply, text preview/apply |
| Diagnostics | Domain match и IP evidence: CIDR, GeoIP, live ipset, firewall, RCI marks |
| Runtime | ipset names, firewall rules, policies, DirectRoute |
| Безопасность записи | Backup перед записью, restore, audit, revision и stale-write rejection |
| HR Neo lifecycle | start, stop, restart, `SIGUSR1` reload |
| Проверки | Unit, local smoke и четыре live-router smoke-профиля |

## Ограничено поверхностью HR Neo

| Сценарий | Причина |
|----------|---------|
| История DNS/CNAME/L7 для уже добавленного IP | HR Neo не сохраняет durable attribution после добавления в ipset |
| L7 runtime counters | HR Neo предоставляет информацию только через logs |
| Доступность VPN и состояние апстримов внутри policy | Это поверхность Keenetic, а не HR Neo |

## Вне области HydraBridge

| Сценарий | Где должен находиться |
|----------|----------------------|
| Управление VPN-подключениями внутри Keenetic policy | Keenetic UI, отдельная интеграция или будущий проект |
| Failover и мониторинг VPN-каналов | Keenetic или отдельный мониторинг |
| XRay, share links и подписки | Отдельный proxy API |
| Каталоги источников GeoData и расписание обновлений | Клиент, cron или отдельная автоматизация поверх API |
| DNS-aware проверка доменов | Опциональный клиентский helper |

## Полезные будущие улучшения

Они не являются блокерами parity с HR Neo `3.11.0-1`:

- diff/preview для bulk-записей;
- скачивание backup archive и diff с текущим состоянием;
- cursor/pagination для logs и audit;
- metadata устройства и более подробное enrichment targets;
- несколько разрешенных CORS origins;
- удобный pairing UX для мобильного приложения;
- tracking pending restart для UI;
- generated Swift SDK из OpenAPI.

## Для клиентов

Клиентским приложениям рекомендуется:

1. Начинать с `/version`, `/status`, `/overview` и `/doctor`.
2. Использовать grouped views для отображения, raw configs для аварийного режима.
3. Отправлять `If-Match` при изменениях.
4. Показывать пользователю `requiredAction`.
5. Хранить bearer token в защищенном хранилище.
6. Не пытаться моделировать VPN failover как функцию HydraBridge.
