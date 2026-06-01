# Соответствие HydraBridge и HR Neo

[English](en/HRNEO_PARITY.md)

Документ фиксирует границу поддержки HR Neo `3.11.0-1`. Она построена по
исходному коду HR Neo и подтверждена smoke-тестами на реальном роутере Keenetic.

## Граница ответственности

HydraBridge предоставляет API для поведения, которое уже реализовано HR Neo.
Он не создает отдельную модель маршрутизации.

- `hrneo.conf`, `domain.conf` и `ip.list` остаются источником истины.
- API может агрегировать повторяющиеся записи для удобного отображения.
- Точечные изменения должны сохранять посторонние байты конфигов.
- Raw и structured bulk replacement явно заменяют выбранный файл.
- Состав подключений Keenetic policy, приоритеты VPN, failover, XRay, подписки и
  мониторинг находятся вне области HR Neo и HydraBridge.

## Подтвержденная модель запуска HR Neo

При старте HR Neo:

1. Читает 27 параметров с приоритетом CLI, конфиг, встроенные значения.
2. Сканирует `/sys/class/net`, если включен DirectRoute.
3. Разбирает `domain.conf` и классифицирует target как policy или интерфейс.
4. Читает активные `/Target`-блоки из `ip.list`.
5. Добавляет цели из `geosite:` и применяет `PolicyOrder`.
6. Создает отсутствующие Keenetic policies через RCI `127.0.0.1:79`.
7. Создает IPv4 и IPv6 ipset через netlink.
8. Загружает CIDR и `geoip:`; слишком крупные GeoIP-теги переносит в
   `#/Too-big-geoip-tag`.
9. Раскрывает поддерживаемые GeoSite Domain и Full в карту доменов.
10. Настраивает DirectRoute `ip rule` и routing tables.
11. Устанавливает упорядоченные CONNMARK-правила через
    `iptables-restore --noflush`.
12. Запускает DNS capture и опциональный L7 NFQUEUE capture.

`SIGUSR1` обновляет состояние интерфейсов, DirectRoute, CONNMARK и L7 firewall.
Он не перечитывает конфиги, watchlist, CIDR, GeoIP, GeoSite и `PolicyOrder`.
Поэтому изменения конфигов требуют restart.

Очистка conntrack реализована внутри HR Neo через `NETLINK_NETFILTER`.
Внешняя утилита `conntrack` не является зависимостью HR Neo.

## Матрица покрытия

| Возможность HR Neo | API HydraBridge | Статус |
|--------------------|-----------------|--------|
| Health, версия, процесс, пути | `/health`, `/version`, `/status`, `/overview` | Готово |
| Готовность роутера и baseline | `/doctor`, `/compatibility` | Готово |
| Все 27 ключей `hrneo.conf` | `/config/hrneo`, `/config/hrneo/structured` | Готово |
| Дефолтный конфиг `hrneo --genconfig` | `/config/hrneo/default`, `/config/hrneo/generate-default` | Готово |
| Редактирование domain и `geosite:` | `/config/domains*`, `/views/domains/grouped` | Готово |
| Редактирование CIDR и `geoip:` | `/config/cidr*`, `/views/cidr/grouped` | Готово |
| Цели, policies и `PolicyOrder` | `/targets*` | Готово |
| DirectRoute | `/targets/interfaces`, `/runtime/direct-routes` | Готово |
| Lifecycle HR Neo | `/service/{action}` | Готово |
| Диагностика доменов и IP | `/diagnostics/domain`, `/diagnostics/ip` | Готово |
| Live ipset и firewall | `/runtime/ipsets`, `/runtime/firewall` | Готово |
| Live Keenetic marks | `/runtime/policies` | Готово |
| GeoData metadata, tags и validation | `/geodata/*` | Готово |
| Backup, restore, audit и logs | `/backups*`, `/audit`, `/logs` | Готово |
| Durable DNS/CNAME/L7 attribution | Нет поверхности в HR Neo | Недоступно |
| L7 runtime counters | Только logs HR Neo | Ограничено HR Neo |

## Правила записи

| Операция | Необходимое действие | Правило сохранения |
|----------|----------------------|--------------------|
| Raw config `PUT` | Restart | Явная полная замена |
| Structured config `PUT` | Restart | Явная bulk-замена |
| Точечный domain/CIDR patch | Restart | Сохранить посторонние байты |
| Изменение `PolicyOrder` | Restart | Минимальный patch `hrneo.conf` |
| GeoData без изменения конфига | Restart перед использованием Neo | Не менять конфиги |
| GeoData add-to-config | Restart | Минимальный patch `hrneo.conf` |
| Service reload | Не требуется | Только `SIGUSR1`, без перечитывания конфигов |

## Проверено на реальном роутере

На роутере с `hrneo v3.11.0-1` проверены:

- readiness, overview, compatibility и structured config;
- inventory target, ipset, firewall, RCI policy marks и DirectRoute;
- диагностика IP с live runtime evidence;
- domain и CIDR add-delete с восстановлением byte-for-byte;
- защита от stale revision;
- manual backup, restore, safety backup и audit;
- `SIGUSR1` reload;
- обратимое создание и удаление Keenetic policy через RCI.

Команды воспроизведения live-router проверок находятся в
[DEVELOPMENT.md](DEVELOPMENT.md#проверка-реального-роутера).
