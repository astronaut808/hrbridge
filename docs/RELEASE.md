# Публичный выпуск HydraBridge

## Блокер перед публикацией

Выберите и добавьте файл `LICENSE`. Лицензия намеренно не выбрана автоматически:
это решение владельца проекта.

## Рекомендуемые данные GitHub

Название:

```text
HydraBridge
```

Описание:

```text
Unified control-plane API for HydraRoute Neo: configuration, diagnostics, runtime inspection, backups, audit logs, and service management for apps, bots, and UIs.
```

Topics:

```text
hydraroute hrneo keenetic entware router api golang openapi networking
```

## Проверки перед тегом

```sh
make ci
make lint
make smoke-local
make all
make package
make feed
```

На реальном роутере:

```sh
ROUTER=192.168.1.1:2080 TOKEN="$TOKEN" make smoke-router

ROUTER=192.168.1.1:2080 TOKEN="$TOKEN" TARGET=Finland \
  CONFIRM_ROUTER_WRITE=YES make smoke-router-write

ROUTER=192.168.1.1:2080 TOKEN="$TOKEN" \
  CONFIRM_SERVICE_ACTIONS=YES SERVICE_ACTIONS=reload \
  make smoke-router-service

ROUTER=192.168.1.1:2080 TOKEN="$TOKEN" \
  CONFIRM_RCI_WRITE=YES make smoke-router-rci
```

Профили, меняющие состояние, запускайте только на своем роутере. Они требуют
явного подтверждения и пытаются выполнить cleanup при ошибке.

## Артефакты релиза

Для `v0.1.0` приложите:

```text
build/hrbridge-aarch64
build/hrbridge-mipsel
build/hrbridge-mips
build/hrbridge_0.1.0_aarch64-3.10.ipk
build/hrbridge_0.1.0_mipsel-3.4.ipk
build/hrbridge_0.1.0_mips-3.4.ipk
api/openapi.yaml
```

Для каждого бинаря и IPK опубликуйте SHA-256.

## Публикация opkg-feed

Workflow [.github/workflows/pages.yml](../.github/workflows/pages.yml) собирает
`build/feed/` и публикует его через GitHub Pages при изменениях в `master`.

После первого push включите GitHub Pages с источником `GitHub Actions`.
Установщик станет доступен по адресу:

```text
https://astronaut808.github.io/hrbridge/keenetic/install.sh
```

Проверьте опубликованный feed на роутере:

```sh
opkg update && opkg install curl && \
  curl -fsSL "https://astronaut808.github.io/hrbridge/keenetic/install.sh" | sh
```

## Подтвержденная совместимость

HydraBridge `0.1.0` поддерживает HR Neo `3.11.0-1`. При переходе на другую
версию Neo повторите аудит исходников и все live-router smoke-профили.
