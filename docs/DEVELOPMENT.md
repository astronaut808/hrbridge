# Разработка HydraBridge

## Требования

- Go `1.26.3` или новее в ветке `1.26.x`.
- `make`, `ar`, `tar`, `gzip` и `shasum` или `sha256sum`.
- `curl` и `jq` для smoke-тестов.

## Основные команды

| Команда | Назначение |
|---------|------------|
| `make generate` | Обновить Go-модели из OpenAPI |
| `make fmt-check` | Проверить форматирование Go-кода через `gofmt` |
| `make test` | Запустить unit-тесты |
| `make lint` | Запустить `golangci-lint v2.12.2` |
| `make ci` | Выполнить codegen, fmt-check, тесты, native build, shell-check и package-check |
| `make smoke-local` | Проверить локальный HTTP API |
| `make aarch64` | Собрать Linux ARM64 бинарь |
| `make mipsel` | Собрать Linux MIPS little-endian бинарь |
| `make mips` | Собрать Linux MIPS big-endian бинарь |
| `make package` | Собрать IPK для всех архитектур |
| `make package-check` | Собрать IPK и проверить opkg-совместимый tar/gzip формат |
| `make feed` | Собрать IPK и статический opkg-feed |

## Артефакты

Бинарники и IPK появляются в `build/`.

`make feed` создает `build/feed/keenetic/`:

```text
install.sh
aarch64-k3.10/
  Packages
  Packages.gz
  hrbridge_<version>_aarch64-3.10.ipk
mipselsf-k3.4/
  Packages
  Packages.gz
  hrbridge_<version>_mipsel-3.4.ipk
mipssf-k3.4/
  Packages
  Packages.gz
  hrbridge_<version>_mips-3.4.ipk
```

Workflow [../.github/workflows/pages.yml](../.github/workflows/pages.yml)
публикует этот каталог через GitHub Pages после push в `master`.

## Публикация opkg-feed

Перед первым запуском workflow один раз включите GitHub Pages:

1. Откройте `Settings -> Pages`.
2. В разделе `Build and deployment` выберите `Source: GitHub Actions`.
3. Повторно запустите workflow `Publish opkg feed`.

После публикации установщик будет доступен по адресу:

```text
https://astronaut808.github.io/hrbridge/keenetic/install.sh
```

## Ручная проверка бинаря на роутере

Если OpenSSH на роутере не содержит SFTP server, используйте legacy SCP:

```sh
scp -O -P 222 build/hrbridge-aarch64 root@192.168.1.1:/tmp/hrbridge
```

На роутере:

```sh
mkdir -p /opt/etc/hrbridge
mv /tmp/hrbridge /opt/etc/hrbridge/hrbridge
chmod +x /opt/etc/hrbridge/hrbridge

/opt/etc/hrbridge/hrbridge \
  -config /opt/etc/hrbridge/hrbridge.conf
```

## Проверка реального роутера

Read-only профиль:

```sh
ROUTER=192.168.1.1:2080 TOKEN="$TOKEN" make smoke-router
```

Профили, меняющие состояние, запускайте только на своем роутере:

```sh
ROUTER=192.168.1.1:2080 TOKEN="$TOKEN" TARGET=<policy> \
  CONFIRM_ROUTER_WRITE=YES make smoke-router-write

ROUTER=192.168.1.1:2080 TOKEN="$TOKEN" \
  CONFIRM_SERVICE_ACTIONS=YES SERVICE_ACTIONS=reload \
  make smoke-router-service

ROUTER=192.168.1.1:2080 TOKEN="$TOKEN" \
  CONFIRM_RCI_WRITE=YES make smoke-router-rci
```
