# Установка HydraBridge

[English](en/INSTALL.md)

## Требования

- Keenetic с Entware.
- Установленный и настроенный HR Neo `3.11.0-1`.
- Доступ к Entware shell от имени `root`.

## Установка через opkg

На роутере:

```sh
opkg update && opkg install curl && \
  curl -fsSL "https://astronaut808.github.io/hrbridge/keenetic/install.sh" | sh
```

Установщик:

1. Проверяет наличие Entware `opkg`.
2. Определяет архитектуру через `opkg print-architecture`.
3. Добавляет feed `hrbridge` в `/opt/etc/opkg/customfeeds.conf`.
4. Устанавливает подходящий пакет.
5. Запускает `/opt/etc/init.d/S99hrbridge`.
6. Выводит случайный bearer token.

Поддерживаются Entware-архитектуры:

```text
aarch64-3.10
mipsel-3.4
mips-3.4
```

Перед запуском `curl | sh` установщик можно просмотреть отдельно:

```sh
curl -fsSL "https://astronaut808.github.io/hrbridge/keenetic/install.sh"
```

## Обновление

На роутере:

```sh
opkg update
opkg upgrade hrbridge
/opt/etc/init.d/S99hrbridge restart
```

## Получение токена

Установщик выводит bearer token после первого запуска. Повторно получить его
можно на роутере:

```sh
sed -n 's/^authToken=//p' /opt/etc/hrbridge/hrbridge.conf
```

## Проверка API

На компьютере:

```sh
export ROUTER=192.168.1.1:2080
export TOKEN='полученный-токен'

curl -s "http://$ROUTER/api/v1/health" | jq
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://$ROUTER/api/v1/doctor" | jq
```

## Ручная установка

Готовый `.ipk` можно установить без подключения feed:

```sh
opkg install /tmp/hrbridge.ipk
/opt/etc/init.d/S99hrbridge start
```

Пакет устанавливает:

```text
/opt/etc/hrbridge/hrbridge
/opt/etc/hrbridge/hrbridge.conf
/opt/etc/init.d/S99hrbridge
```

Конфиг сохраняется при обновлении пакета. Его шаблон находится в
[../packaging/hrbridge.conf](../packaging/hrbridge.conf).

Инструкции по локальной сборке бинарей, IPK и собственного feed находятся в
[DEVELOPMENT.md](DEVELOPMENT.md).
