# Безопасность

## Модель доступа

HydraBridge предназначен для локальной сети роутера. По умолчанию он слушает
`0.0.0.0:2080` по HTTP и защищает endpoint bearer-токеном.

Не публикуйте порт HydraBridge в интернет. Ограничьте доступ firewall-правилами
роутера или включите TLS через `enableTLS=true`, `certFile=` и `keyFile=`.

## Токен

HydraBridge создает случайный токен при первом запуске, если `authToken=` пуст.
Файл `/opt/etc/HydraRoute/hrbridge.conf` должен быть доступен только root.

Ротация токена:

```sh
curl -s -X POST \
  -H "Authorization: Bearer $TOKEN" \
  "http://$ROUTER/api/v1/auth/token/rotate" | jq
```

Старый токен перестает действовать сразу после успешной ротации.

## GeoData

`POST /api/v1/geodata/files` записывает файлы только в каталог `geofile` рядом
с `hrneo.conf`. `POST /api/v1/geodata/download` принимает только HTTP(S)-URL,
которые разрешаются в публичные IP-адреса. Loopback, private и link-local
адреса запрещены.

## Сообщение об уязвимости

Не публикуйте сведения об уязвимости в открытом issue до согласования
исправления. Используйте private security advisory GitHub-репозитория.
