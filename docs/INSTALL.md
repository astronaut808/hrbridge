# Установка HydraBridge

[English](en/INSTALL.md)

## Требования

- Keenetic с Entware.
- Установленный и настроенный HR Neo `3.11.0-1`.
- Доступ по SSH к роутеру.
- `curl` и `jq` на компьютере для проверки API.
- Go `1.26.3` или новее в ветке `1.26.x` для локальной сборки.

## Вариант 1: автоматическая установка через opkg

На роутере:

```sh
opkg update && opkg install curl && \
  curl -fsSL "https://astronaut808.github.io/hrbridge/keenetic/install.sh" | sh
```

Установщик:

1. Проверяет запуск от `root` и наличие Entware `opkg`.
2. Определяет архитектуру через `opkg print-architecture`.
3. Добавляет feed `hrbridge` в `/opt/etc/opkg/customfeeds.conf`.
4. Выполняет `opkg update` и `opkg install hrbridge`.
5. Запускает `/opt/etc/init.d/S99hrbridge`.
6. Выводит случайный bearer token из конфига.

Поддерживаются Entware-архитектуры:

```text
aarch64-3.10
mipsel-3.4
mips-3.4
```

Для обновления уже установленного HydraBridge:

```sh
opkg update
opkg upgrade hrbridge
/opt/etc/init.d/S99hrbridge restart
```

Перед запуском `curl | sh` можно отдельно скачать и просмотреть установщик:

```sh
curl -fsSL "https://astronaut808.github.io/hrbridge/keenetic/install.sh"
```

## Вариант 2: бинарь

### 1. Соберите нужную архитектуру

Для ARM64:

```sh
make aarch64
```

Для MIPS little-endian:

```sh
make mipsel
```

Для MIPS big-endian:

```sh
make mips
```

### 2. Передайте бинарь на роутер

Если OpenSSH на роутере не содержит SFTP server, используйте legacy SCP:

```sh
scp -O -P 222 build/hrbridge-aarch64 root@192.168.1.1:/tmp/hrbridge
```

### 3. Разместите бинарь

На роутере:

```sh
mkdir -p /opt/etc/hrbridge
mv /tmp/hrbridge /opt/etc/hrbridge/hrbridge
chmod +x /opt/etc/hrbridge/hrbridge
```

### 4. Первый запуск

```sh
/opt/etc/hrbridge/hrbridge \
  -config /opt/etc/HydraRoute/hrbridge.conf
```

При первом запуске HydraBridge создаст конфиг и сохранит случайный bearer token.

### 5. Получите токен

На роутере:

```sh
sed -n 's/^authToken=//p' /opt/etc/HydraRoute/hrbridge.conf
```

### 6. Проверьте API

На компьютере:

```sh
export ROUTER=192.168.1.1:2080
export TOKEN='полученный-токен'

curl -s "http://$ROUTER/api/v1/health" | jq
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://$ROUTER/api/v1/doctor" | jq
```

## Вариант 3: локальный IPK

Соберите пакеты:

```sh
make package
```

Архивы будут находиться в `build/`. IPK устанавливает:

```text
/opt/bin/hrbridge
/opt/etc/HydraRoute/hrbridge.conf
/opt/etc/init.d/S99hrbridge
```

Передайте нужный IPK на роутер и установите его:

```sh
scp -O -P 222 build/hrbridge_0.1.0_aarch64-3.10.ipk \
  root@192.168.1.1:/tmp/hrbridge.ipk

ssh -p 222 root@192.168.1.1
opkg install /tmp/hrbridge.ipk
```

После установки на роутере:

```sh
/opt/etc/init.d/S99hrbridge start
```

## Собственный opkg-feed

Для локальной генерации статического feed:

```sh
make feed
```

Готовая структура появится в `build/feed/`. Она включает установщик, индексы
`Packages`, сжатые индексы `Packages.gz` и IPK для всех поддерживаемых
архитектур. Workflow [../.github/workflows/pages.yml](../.github/workflows/pages.yml)
публикует этот каталог через GitHub Pages.

## Конфиг

Путь по умолчанию:

```text
/opt/etc/HydraRoute/hrbridge.conf
```

Шаблон находится в [../packaging/hrbridge.conf](../packaging/hrbridge.conf).

## Проверка реального роутера

После установки:

```sh
ROUTER=192.168.1.1:2080 TOKEN="$TOKEN" make smoke-router
```

Остальные smoke-профили описаны в [RELEASE.md](RELEASE.md).
