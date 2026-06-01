#!/bin/sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
BINARY=${BINARY:-"$ROOT/build/hrbridge"}
PORT=${PORT:-$((22000 + ($$ % 20000)))}
BASE_URL="http://127.0.0.1:$PORT/api/v1"
TOKEN=smoke-token
TARGET=Finland
DOMAIN=hrbridge-write-smoke.test
CIDR=8.8.8.8/32
TMP=$(mktemp -d "${TMPDIR:-/tmp}/hrbridge-smoke.XXXXXX")
PID=

cleanup() {
	if [ -n "$PID" ]; then
		kill "$PID" 2>/dev/null || true
		wait "$PID" 2>/dev/null || true
	fi
	rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

fail() {
	printf 'smoke: FAIL: %s\n' "$*" >&2
	if [ -f "$TMP/hrbridge.log" ]; then
		printf '%s\n' '--- hrbridge.log ---' >&2
		cat "$TMP/hrbridge.log" >&2
	fi
	exit 1
}

json_get() {
	curl -fsS -H "Authorization: Bearer $TOKEN" "$BASE_URL$1"
}

json_request() {
	method=$1
	path=$2
	body=$3
	revision=${4:-}
	if [ -n "$revision" ]; then
		curl -fsS -X "$method" \
			-H "Authorization: Bearer $TOKEN" \
			-H "Content-Type: application/json" \
			-H "If-Match: $revision" \
			-d "$body" "$BASE_URL$path"
	else
		curl -fsS -X "$method" \
			-H "Authorization: Bearer $TOKEN" \
			-H "Content-Type: application/json" \
			-d "$body" "$BASE_URL$path"
	fi
}

revision() {
	curl -fsS -D "$TMP/headers" -o /dev/null \
		-H "Authorization: Bearer $TOKEN" "$BASE_URL/config/domains"
	awk 'tolower($1) == "x-config-revision:" { gsub("\r", "", $2); print $2 }' "$TMP/headers"
}

[ -x "$BINARY" ] || fail "missing executable $BINARY; run make native"
command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v jq >/dev/null 2>&1 || fail "jq is required"

mkdir -p "$TMP/backups" "$TMP/geofile"
cat >"$TMP/hrneo.conf" <<EOF
autoStart=true
CIDR=true
CIDRfile=$TMP/ip.list
GeoIPFile=$TMP/geofile/geoip.dat
GeoSiteFile=$TMP/geofile/geosite.dat
DirectRouteEnabled=true
EOF
cat >"$TMP/domain.conf" <<'EOF'
example.com/Finland
EOF
cat >"$TMP/ip.list" <<'EOF'
/Finland
1.1.1.1/32
EOF
printf '\012\004\012\002RU\012\004\012\002US' >"$TMP/geofile/geoip.dat"
printf '\012\011\012\007YOUTUBE\012\016\012\014CATEGORY-ADS' >"$TMP/geofile/geosite.dat"
cat >"$TMP/hrneo.log" <<'EOF'
[INFO] local smoke
EOF
cat >"$TMP/hrbridge.conf" <<EOF
listen=127.0.0.1:$PORT
authToken=$TOKEN
allowOrigins=
enableTLS=false
certFile=
keyFile=
backupDir=$TMP/backups
logFile=$TMP/hrneo.log
auditLog=$TMP/audit.log
hrneoConf=$TMP/hrneo.conf
domainConf=$TMP/domain.conf
cidrList=$TMP/ip.list
hrneoPid=$TMP/hrneo.pid
hrneoInit=$TMP/S99hrneo
rciURL=http://127.0.0.1:1
EOF

cp "$TMP/domain.conf" "$TMP/domain.before"
"$BINARY" -config "$TMP/hrbridge.conf" >"$TMP/hrbridge.log" 2>&1 &
PID=$!

i=0
until curl -fsS "$BASE_URL/health" >/dev/null 2>&1; do
	i=$((i + 1))
	[ "$i" -lt 50 ] || fail "agent did not start; see $TMP/hrbridge.log"
	sleep 0.1
done

printf 'smoke: read-only endpoints\n'
printf '  - health\n'
curl -fsS "$BASE_URL/health" | jq -e '.ok == true' >/dev/null
printf '  - version\n'
json_get /version | jq -e '.apiVersion == "v1"' >/dev/null
printf '  - overview\n'
json_get /overview | jq -e '.targetCount == 1 and .domainRuleCount == 1 and .cidrRuleCount == 1' >/dev/null
printf '  - doctor\n'
json_get /doctor | jq -e '.checks | length > 0' >/dev/null
printf '  - compatibility\n'
json_get /compatibility | jq -e '.supportedHrneoVersion == "3.11.0-1"' >/dev/null
printf '  - hrneo defaults and metadata\n'
json_get /config/hrneo/default | jq -e '.requiredAction == "restart" and (.content | contains("l7TcpReasmTtlSec=5"))' >/dev/null
json_get /metadata/hrneo-params | jq -e '.supportedHrneoVersion == "3.11.0-1" and (.params | length == 27)' >/dev/null
printf '  - geodata files\n'
json_get /geodata/files | jq -e '.files | length == 2' >/dev/null
printf '  - geodata tags\n'
json_get /geodata/tags | jq -e '(.geoip | index("RU")) and (.geosite | index("youtube"))' >/dev/null
printf '  - geodata validate\n'
json_get /geodata/validate | jq -e '.issues | type == "array"' >/dev/null
printf '  - grouped views\n'
json_get /views/domains/grouped | jq -e '.targets | length == 1' >/dev/null
json_get /views/cidr/grouped | jq -e '.targets | length == 1' >/dev/null
printf '  - IP diagnostic runtime shape\n'
json_request POST /diagnostics/ip '{"ip":"1.1.1.1"}' \
	| jq -e '.matched == true and (.geoipMatches | type == "array") and (.runtime.memberships | type == "array") and (.runtime.errors | type == "array")' >/dev/null

printf 'smoke: domain patch, revision, stale-write protection\n'
REV_BEFORE=$(revision)
[ -n "$REV_BEFORE" ] || fail "missing config revision"
json_request POST "/config/domains/targets/$TARGET/rules" \
	"{\"kind\":\"domain\",\"value\":\"$DOMAIN\"}" "$REV_BEFORE" \
	| jq -e '.saved == true and .applied == false' >/dev/null
json_get /config/domains/structured \
	| jq -e --arg domain "$DOMAIN" '[.config.targets[].domains[]?] | index($domain)' >/dev/null

STATUS=$(curl -sS -o "$TMP/stale.json" -w '%{http_code}' -X POST \
	-H "Authorization: Bearer $TOKEN" \
	-H "Content-Type: application/json" \
	-H "If-Match: $REV_BEFORE" \
	-d '{"kind":"domain","value":"must-not-be-added.test"}' \
	"$BASE_URL/config/domains/targets/$TARGET/rules")
[ "$STATUS" = 412 ] || fail "stale write returned HTTP $STATUS instead of 412"
jq -e '.error == "config revision mismatch"' "$TMP/stale.json" >/dev/null

REV_AFTER_ADD=$(revision)
json_request DELETE "/config/domains/targets/$TARGET/rules" \
	"{\"kind\":\"domain\",\"value\":\"$DOMAIN\"}" "$REV_AFTER_ADD" \
	| jq -e '.saved == true' >/dev/null
cmp -s "$TMP/domain.before" "$TMP/domain.conf" || fail "domain.conf did not round-trip exactly"

printf 'smoke: CIDR patch round-trip\n'
cp "$TMP/ip.list" "$TMP/ip.before"
json_request POST "/config/cidr/targets/$TARGET/rules" \
	"{\"kind\":\"cidr\",\"value\":\"$CIDR\"}" \
	| jq -e '.saved == true' >/dev/null
json_request DELETE "/config/cidr/targets/$TARGET/rules" \
	"{\"kind\":\"cidr\",\"value\":\"$CIDR\"}" \
	| jq -e '.saved == true' >/dev/null
cmp -s "$TMP/ip.before" "$TMP/ip.list" || fail "ip.list did not round-trip exactly"

printf 'smoke: backups and audit\n'
json_get /backups | jq -e '.backups | length >= 4' >/dev/null
BACKUP_COUNT=$(find "$TMP/backups" -mindepth 1 -maxdepth 1 -type d | wc -l | tr -d ' ')
[ "$BACKUP_COUNT" -ge 4 ] || fail "expected at least 4 unique backup directories, got $BACKUP_COUNT"
json_get '/audit?limit=20' | jq -e '[.events[] | select(.action == "config.patch" and .ok == true)] | length >= 4' >/dev/null

printf 'smoke: PASS (%s)\n' "$BASE_URL"
