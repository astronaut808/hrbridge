# Обзор API HydraBridge

[English](en/API.md)

Полный машинно-читаемый контракт находится в
[../api/openapi.yaml](../api/openapi.yaml). Все маршруты, кроме `/api/v1/health`,
требуют Bearer token.

## Базовые endpoint

| Метод | Путь | Назначение |
|-------|------|------------|
| `GET` | `/api/v1/health` | Публичная проверка доступности |
| `GET` | `/api/v1/version` | Версия API и capability-флаги |
| `GET` | `/api/v1/status` | Статус HR Neo и пути |
| `GET` | `/api/v1/overview` | Сводка для dashboard |
| `GET` | `/api/v1/doctor` | Диагностика готовности роутера |
| `GET` | `/api/v1/compatibility` | Совместимость с HR Neo `3.11.0-1` |

## Конфигурация

Raw endpoint:

```text
GET|PUT /api/v1/config/{hrneo|domains|cidr}
```

Structured endpoint:

```text
GET|PUT /api/v1/config/hrneo/structured
GET|PUT /api/v1/config/domains/structured
GET|PUT /api/v1/config/cidr/structured
```

Точечные patches:

```text
POST   /api/v1/config/domains/targets/{target}/rules
DELETE /api/v1/config/domains/targets/{target}/rules?kind={kind}&value={value}[&comment={comment}]
POST   /api/v1/config/cidr/targets/{target}/rules
DELETE /api/v1/config/cidr/targets/{target}/rules?kind={kind}&value={value}[&comment={comment}]
```

`POST` принимает JSON body. `DELETE` использует query-параметры, совместимые с
OpenAPI 3.0 и Swagger Editor.

Для сохранения удобной структуры HR Neo можно передать `comment`. Это имя
существующей или новой группы `##...` в `domain.conf` или `ip.list`; формат
файлов HR Neo при этом не меняется.

Пример добавления домена в группу `##Music`:

```json
{
  "kind": "domain",
  "value": "spotify.com",
  "comment": "Music",
  "apply": false
}
```

Если `##Music` уже есть у target `Finland`, домен будет добавлен в этот блок.
Если такой группы нет, HydraBridge создаст новый блок:

```text
##Music
spotify.com/Finland
```

Чтение конфигов возвращает `ETag` и `X-Config-Revision`. Для записи передавайте
`If-Match`, чтобы не перезаписать изменения другого клиента.

## Представления для UI

```text
GET /api/v1/views/domains/grouped
GET /api/v1/views/cidr/grouped
```

Эти endpoint группируют повторяющиеся блоки для отображения и не изменяют файлы.

## Targets и GeoData

```text
GET|POST|DELETE /api/v1/targets/*
GET|PUT         /api/v1/targets/order
GET|POST        /api/v1/geodata/*
```

Точный список см. в OpenAPI.

GeoData upload записывает файлы только в каталог `geofile` рядом с
`hrneo.conf`. GeoData download разрешает только HTTP(S)-источники с публичными
IP-адресами.

## Диагностика

```text
POST /api/v1/diagnostics/domain
POST /api/v1/diagnostics/ip
```

IP diagnostics объединяет статические CIDR, GeoIP prefixes, live membership в
ipset, firewall rules и RCI marks. История происхождения динамического IP не
выдумывается, если HR Neo ее не хранит.

## Runtime и lifecycle

```text
GET  /api/v1/runtime/ipsets
GET  /api/v1/runtime/firewall
GET  /api/v1/runtime/policies
GET  /api/v1/runtime/direct-routes
POST /api/v1/service/{start|stop|restart|reload}
```

## Backup, audit и logs

```text
GET|POST /api/v1/backups
POST     /api/v1/backups/restore
GET      /api/v1/audit?limit=100
GET      /api/v1/logs?limit=300
```

## Пример

```sh
curl -s -H "Authorization: Bearer $TOKEN" \
  "http://$ROUTER/api/v1/overview" | jq
```
