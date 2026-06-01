# HydraBridge

[English](README.en.md)

HydraBridge - единый HTTP API управления для [HydraRoute Neo](https://github.com/Ground-Zerro/HydraRoute).

Это небольшой агент для роутеров Keenetic с Entware. Он запускается рядом с
`hrneo`, но не маршрутизирует трафик самостоятельно. HydraBridge дает приложениям,
ботам, CLI-инструментам и новым web-интерфейсам стабильный API для настройки и
диагностики существующих возможностей HR Neo.

## Статус

- Версия HydraBridge: `0.1.0`
- Поддерживаемая версия HR Neo: `3.11.0-1`
- API: `/api/v1`
- OpenAPI-контракт: [api/openapi.yaml](api/openapi.yaml)
- Лицензия: должна быть выбрана перед публичным релизом

Подробная карта соответствия HR Neo находится в
[docs/HRNEO_PARITY.md](docs/HRNEO_PARITY.md).

## Принципы

- `hrneo.conf`, `domain.conf` и `ip.list` остаются источником истины.
- HydraBridge не создает вторую модель маршрутизации поверх HR Neo.
- Удобные сгруппированные ответы API доступны только как read-only представления.
- Точечные изменения доменов, GeoSite, CIDR и GeoIP сохраняют посторонние байты
  файлов конфигурации.
- Перед изменениями создаются резервные копии.
- Для конкурентного редактирования используются `ETag`, `X-Config-Revision` и
  `If-Match`.

## Возможности

- Bearer-аутентификация и ротация токена.
- Статус HydraBridge и HR Neo, Doctor и проверка совместимости.
- Raw и structured API для `hrneo.conf`, `domain.conf` и `ip.list`.
- Метаданные всех 27 параметров HR Neo и preview дефолтного конфига.
- Точечное добавление и удаление domain, GeoSite, CIDR и GeoIP-правил.
- Инвентаризация целей, интерфейсов DirectRoute и Keenetic policies.
- Управление `PolicyOrder`.
- Загрузка, скачивание и точная потоковая индексация GeoIP/GeoSite `.dat`.
- CSV и текстовый import/export.
- Диагностика доменов и IP.
- Read-only просмотр live ipset, firewall, policy marks и DirectRoute.
- Резервные копии, восстановление, audit log и tail логов HR Neo.
- Запуск, остановка, перезапуск и `SIGUSR1` reload HR Neo.

Полный список маршрутов описан в [docs/API.md](docs/API.md) и
[api/openapi.yaml](api/openapi.yaml).

## Быстрый старт

Подробная пошаговая инструкция: [docs/INSTALL.md](docs/INSTALL.md).

На роутере с установленным HR Neo:

```sh
opkg update && opkg install curl && \
  curl -fsSL "https://astronaut808.github.io/hrbridge/keenetic/install.sh" | sh
```

Установщик добавит opkg-feed HydraBridge, установит пакет для архитектуры
роутера, запустит сервис и выведет bearer token.

### Локальная сборка

Для сборки используйте Go `1.26.3` или новее в ветке `1.26.x`.

### Сборка для Keenetic ARM64

```sh
make aarch64
```

Бинарь появится в:

```text
build/hrbridge-aarch64
```

### Размещение на роутере

```sh
mkdir -p /opt/etc/hrbridge
cp /tmp/hrbridge-aarch64 /opt/etc/hrbridge/hrbridge
chmod +x /opt/etc/hrbridge/hrbridge
```

Запуск:

```sh
/opt/etc/hrbridge/hrbridge \
  -config /opt/etc/HydraRoute/hrbridge.conf
```

При первом запуске агент создаст конфиг при необходимости, сгенерирует
случайный токен и сохранит его в:

```text
/opt/etc/HydraRoute/hrbridge.conf
```

Получить токен на роутере:

```sh
sed -n 's/^authToken=//p' /opt/etc/HydraRoute/hrbridge.conf
```

Проверить доступ с компьютера:

```sh
export ROUTER=192.168.1.1:2080
export TOKEN='полученный-токен'

curl -s "http://$ROUTER/api/v1/health" | jq
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://$ROUTER/api/v1/overview" | jq
```

## Безопасность

По умолчанию HydraBridge слушает `0.0.0.0:2080` и использует HTTP. Не публикуйте
этот порт в интернет. Для домашней сети ограничьте доступ firewall-правилами
роутера или включите TLS через `enableTLS=true`, `certFile=` и `keyFile=`.

Все endpoint, кроме `/api/v1/health`, требуют:

```http
Authorization: Bearer <token>
```

Подробнее: [SECURITY.md](SECURITY.md).

## Проверки

Локальная проверка:

```sh
make ci
make smoke-local
```

Read-only проверка реального роутера:

```sh
ROUTER=192.168.1.1:2080 TOKEN="$TOKEN" make smoke-router
```

Профили, меняющие состояние, требуют явного подтверждения:

```sh
ROUTER=192.168.1.1:2080 TOKEN="$TOKEN" TARGET=Finland \
  CONFIRM_ROUTER_WRITE=YES make smoke-router-write

ROUTER=192.168.1.1:2080 TOKEN="$TOKEN" \
  CONFIRM_SERVICE_ACTIONS=YES SERVICE_ACTIONS=reload \
  make smoke-router-service

ROUTER=192.168.1.1:2080 TOKEN="$TOKEN" \
  CONFIRM_RCI_WRITE=YES make smoke-router-rci
```

Подробности: [docs/RELEASE.md](docs/RELEASE.md).

## Сборочные цели

```sh
make generate      # обновить Go-модели из OpenAPI
make test          # unit-тесты
make lint          # golangci-lint v2.12.2
make ci            # codegen, тесты и native build
make smoke-local   # локальный HTTP smoke
make aarch64       # Linux ARM64
make mipsel        # Linux MIPS little-endian
make mips          # Linux MIPS big-endian
make package       # Entware/opkg IPK
make feed          # IPK и статический opkg-feed для GitHub Pages
```

## Документация

- [Установка и первый запуск](docs/INSTALL.md)
- [Обзор API](docs/API.md)
- [Соответствие HR Neo 3.11.0-1](docs/HRNEO_PARITY.md)
- [Карта сценариев использования](docs/USE_CASE_COVERAGE.md)
- [Подготовка публичного релиза](docs/RELEASE.md)
- [Политика безопасности](SECURITY.md)
- [Участие в разработке](CONTRIBUTING.md)
- [История изменений](CHANGELOG.md)

## Границы проекта

HydraBridge поддерживает то, что уже реализовано HR Neo. Управление VPN-профилями
Keenetic, составом подключений внутри policy, failover апстримов, XRay,
подписками и мониторингом доступности не относятся к HR Neo и не входят в
область ответственности HydraBridge.
