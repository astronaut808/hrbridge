#!/bin/sh
set -eu

BASE_URL=${BASE_URL:-}
ROUTER=${ROUTER:-}
TOKEN=${TOKEN:-}
MODE=${MODE:-readonly}
TARGET=${TARGET:-}
DIAG_IP=${DIAG_IP:-}
SERVICE_ACTIONS=${SERVICE_ACTIONS:-reload}
CONFIRM_ROUTER_WRITE=${CONFIRM_ROUTER_WRITE:-}
CONFIRM_SERVICE_ACTIONS=${CONFIRM_SERVICE_ACTIONS:-}
CONFIRM_RCI_WRITE=${CONFIRM_RCI_WRITE:-}
TMP=$(mktemp -d "${TMPDIR:-/tmp}/hrbridge-router-smoke.XXXXXX")
DOMAIN="hrbridge-router-smoke-$$.test"
STALE_DOMAIN="stale-$DOMAIN"
CIDR="203.0.113.$(($$ % 200 + 1))/32"
GROUP_COMMENT="HRBridgeSmokeGroup$$"
POLICY="HRBridgeSmoke$$"
DOMAIN_ADDED=
STALE_DOMAIN_ADDED=
CIDR_ADDED=
POLICY_ADDED=

cleanup() {
	if [ "$MODE" = write ]; then
		[ -z "$DOMAIN_ADDED" ] || delete_rule_best_effort domains domain "$DOMAIN"
		[ -z "$STALE_DOMAIN_ADDED" ] || delete_rule_best_effort domains domain "$STALE_DOMAIN"
		[ -z "$CIDR_ADDED" ] || delete_rule_best_effort cidr cidr "$CIDR"
	fi
	if [ "$MODE" = rci ] && [ -n "$POLICY_ADDED" ]; then
		curl -fsS -X DELETE -H "Authorization: Bearer $TOKEN" \
			"$BASE_URL/targets/policies/$POLICY" >/dev/null 2>&1 || true
	fi
	rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

fail() {
	printf 'router-smoke: FAIL: %s\n' "$*" >&2
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

delete_rule() {
	path=$1
	kind=$2
	value=$3
	revision=${4:-}
	comment=${5:-}
	if [ -n "$revision" ]; then
		if [ -n "$comment" ]; then
			curl -fsS -X DELETE \
				-H "Authorization: Bearer $TOKEN" \
				-H "If-Match: $revision" \
				--get --data-urlencode "kind=$kind" --data-urlencode "value=$value" \
				--data-urlencode "comment=$comment" \
				"$BASE_URL$path"
		else
			curl -fsS -X DELETE \
				-H "Authorization: Bearer $TOKEN" \
				-H "If-Match: $revision" \
				--get --data-urlencode "kind=$kind" --data-urlencode "value=$value" \
				"$BASE_URL$path"
		fi
	else
		if [ -n "$comment" ]; then
			curl -fsS -X DELETE \
				-H "Authorization: Bearer $TOKEN" \
				--get --data-urlencode "kind=$kind" --data-urlencode "value=$value" \
				--data-urlencode "comment=$comment" \
				"$BASE_URL$path"
		else
			curl -fsS -X DELETE \
				-H "Authorization: Bearer $TOKEN" \
				--get --data-urlencode "kind=$kind" --data-urlencode "value=$value" \
				"$BASE_URL$path"
		fi
	fi
}

revision() {
	name=$1
	curl -fsS -D "$TMP/headers" -o /dev/null \
		-H "Authorization: Bearer $TOKEN" "$BASE_URL/config/$name"
	awk 'tolower($1) == "x-config-revision:" { gsub("\r", "", $2); print $2 }' "$TMP/headers"
}

rule_body() {
	jq -cn --arg kind "$1" --arg value "$2" '{kind:$kind,value:$value}'
}

group_rule_body() {
	jq -cn --arg kind "$1" --arg value "$2" --arg comment "$3" '{kind:$kind,value:$value,comment:$comment}'
}

delete_rule_best_effort() {
	name=$1
	kind=$2
	value=$3
	rev=$(revision "$name" 2>/dev/null || true)
	[ -n "$rev" ] || return 0
	delete_rule "/config/$name/targets/$TARGET/rules" \
		"$kind" "$value" "$rev" >/dev/null 2>&1 || true
}

restore_raw_best_effort() {
	name=$1
	path=$2
	rev=$(revision "$name" 2>/dev/null || true)
	[ -n "$rev" ] || return 0
	jq -Rs '{content:.,apply:false}' <"$path" >"$TMP/restore.json"
	curl -fsS -X PUT \
		-H "Authorization: Bearer $TOKEN" \
		-H "Content-Type: application/json" \
		-H "If-Match: $rev" \
		--data-binary "@$TMP/restore.json" \
		"$BASE_URL/config/$name" >/dev/null 2>&1 || true
}

fail_roundtrip() {
	name=$1
	before=$2
	after=$3
	restore_raw_best_effort "$name" "$before"
	printf '%s\n' "--- $name round-trip diff ---" >&2
	if command -v diff >/dev/null 2>&1; then
		diff -u "$before" "$after" >&2 || true
	fi
	fail "$name did not round-trip byte-for-byte; original content restore was attempted"
}

require_confirmation() {
	actual=$1
	expected=$2
	[ "$actual" = "$expected" ] || fail "set $3=$expected to run MODE=$MODE"
}

case "$MODE" in
	readonly|write|service|rci) ;;
	*) fail "MODE must be one of: readonly, write, service, rci" ;;
esac

if [ -z "$BASE_URL" ]; then
	[ -n "$ROUTER" ] || fail "set ROUTER=192.168.1.1:2080 or BASE_URL=http://host:port/api/v1"
	BASE_URL="http://$ROUTER/api/v1"
fi
BASE_URL=${BASE_URL%/}
[ -n "$TOKEN" ] || fail "set TOKEN to the HydraBridge bearer token"
command -v curl >/dev/null 2>&1 || fail "curl is required"
command -v jq >/dev/null 2>&1 || fail "jq is required"

case "$MODE" in
	write)
		require_confirmation "$CONFIRM_ROUTER_WRITE" YES CONFIRM_ROUTER_WRITE
		[ -n "$TARGET" ] || fail "set TARGET to an existing HR Neo target for write smoke"
		;;
	service)
		require_confirmation "$CONFIRM_SERVICE_ACTIONS" YES CONFIRM_SERVICE_ACTIONS
		;;
	rci)
		require_confirmation "$CONFIRM_RCI_WRITE" YES CONFIRM_RCI_WRITE
		;;
esac

printf 'router-smoke: read-only API (%s)\n' "$BASE_URL"
printf '  - health\n'
curl -fsS "$BASE_URL/health" | jq -e '.ok == true' >/dev/null
printf '  - version and capability boundary\n'
json_get /version | jq -e '
	.apiVersion == "v1" and
	(.capabilities | index("diagnostics.ip.runtimeEvidence")) != null
' >/dev/null
printf '  - status and overview\n'
json_get /status | jq -e '.hrneo.installed == true and .hrneo.running == true' >/dev/null
json_get /overview | jq -e '.ok == true and .targetCount >= 1' >/dev/null
printf '  - doctor and compatibility\n'
json_get /doctor | jq -e '.ok == true' >/dev/null
json_get /compatibility | jq -e '.ok == true and .supportedHrneoVersion == "3.11.0-1"' >/dev/null
printf '  - hrneo defaults and metadata\n'
json_get /config/hrneo/default | jq -e '.requiredAction == "restart"' >/dev/null
json_get /metadata/hrneo-params | jq -e '.supportedHrneoVersion == "3.11.0-1" and (.params | length == 27)' >/dev/null
printf '  - targets and runtime\n'
json_get /targets | jq -e '.targets | type == "array"' >/dev/null
json_get /runtime/ipsets | jq -e '.available == true and (.referencedSets | type == "array")' >/dev/null
json_get /runtime/firewall | jq -e '.ipv4Available == true and .ipv6Available == true' >/dev/null
json_get /runtime/policies | jq -e '.policies | type == "array"' >/dev/null
json_get /runtime/direct-routes | jq -e '.interfaces | type == "array"' >/dev/null
printf '  - geodata\n'
json_get /geodata/files | jq -e '.files | type == "array"' >/dev/null
json_get /geodata/tags | jq -e '(.geoip | type == "array") and (.geosite | type == "array")' >/dev/null
json_get /geodata/validate | jq -e '.ok == true' >/dev/null
printf '  - backups and audit\n'
json_get /backups | jq -e '.backups | type == "array"' >/dev/null
json_get '/audit?limit=10' | jq -e '.events | type == "array"' >/dev/null

if [ -n "$DIAG_IP" ]; then
	printf '  - IP diagnostic (%s)\n' "$DIAG_IP"
	json_request POST /diagnostics/ip "$(jq -cn --arg ip "$DIAG_IP" '{ip:$ip}')" \
		| jq -e '.runtime.ipsetAvailable == true and (.runtime.memberships | type == "array")' >/dev/null
fi

if [ "$MODE" = readonly ]; then
	printf 'router-smoke: PASS (read-only)\n'
	exit 0
fi

if [ "$MODE" = write ]; then
	printf 'router-smoke: reversible config writes (target=%s)\n' "$TARGET"
	json_get /config/domains | jq -j '.content' >"$TMP/domain.before"
	json_get /config/cidr | jq -j '.content' >"$TMP/cidr.before"
	grep -F "$DOMAIN" "$TMP/domain.before" >/dev/null 2>&1 && fail "temporary domain already exists: $DOMAIN"
	grep -F "$STALE_DOMAIN" "$TMP/domain.before" >/dev/null 2>&1 && fail "stale-test domain already exists: $STALE_DOMAIN"
	grep -F "$CIDR" "$TMP/cidr.before" >/dev/null 2>&1 && fail "temporary CIDR already exists: $CIDR"

	rev=$(revision domains)
	DOMAIN_ADDED=1
	json_request POST "/config/domains/targets/$TARGET/rules" \
		"$(rule_body domain "$DOMAIN")" "$rev" | jq -e '.saved == true and .applied == false' >/dev/null
	rev_after_add=$(revision domains)
	status=$(curl -sS -o "$TMP/stale.json" -w '%{http_code}' -X POST \
		-H "Authorization: Bearer $TOKEN" \
		-H "Content-Type: application/json" \
		-H "If-Match: $rev" \
		-d "$(rule_body domain "$STALE_DOMAIN")" \
		"$BASE_URL/config/domains/targets/$TARGET/rules")
	[ "$status" = 412 ] || STALE_DOMAIN_ADDED=1
	[ "$status" = 412 ] || fail "stale domain write returned HTTP $status instead of 412"
	delete_rule "/config/domains/targets/$TARGET/rules" \
		domain "$DOMAIN" "$rev_after_add" | jq -e '.saved == true' >/dev/null
	DOMAIN_ADDED=
	json_get /config/domains | jq -j '.content' >"$TMP/domain.after"
	cmp -s "$TMP/domain.before" "$TMP/domain.after" \
		|| fail_roundtrip domains "$TMP/domain.before" "$TMP/domain.after"

	rev=$(revision domains)
	DOMAIN_ADDED=1
	json_request POST "/config/domains/targets/$TARGET/rules" \
		"$(group_rule_body domain "$DOMAIN" "$GROUP_COMMENT")" "$rev" | jq -e '.saved == true and .applied == false' >/dev/null
	rev_after_group_add=$(revision domains)
	json_get /config/domains | jq -j '.content' >"$TMP/domain.group"
	grep -F "##$GROUP_COMMENT" "$TMP/domain.group" >/dev/null \
		|| fail "grouped domain write did not create comment group"
	grep -F "$DOMAIN/$TARGET" "$TMP/domain.group" >/dev/null \
		|| fail "grouped domain write did not create rule"
	delete_rule "/config/domains/targets/$TARGET/rules" \
		domain "$DOMAIN" "$rev_after_group_add" "$GROUP_COMMENT" | jq -e '.saved == true' >/dev/null
	DOMAIN_ADDED=
	json_get /config/domains | jq -j '.content' >"$TMP/domain.after_group"
	cmp -s "$TMP/domain.before" "$TMP/domain.after_group" \
		|| fail_roundtrip domains "$TMP/domain.before" "$TMP/domain.after_group"

	rev=$(revision cidr)
	CIDR_ADDED=1
	json_request POST "/config/cidr/targets/$TARGET/rules" \
		"$(rule_body cidr "$CIDR")" "$rev" | jq -e '.saved == true and .applied == false' >/dev/null
	rev_after_add=$(revision cidr)
	delete_rule "/config/cidr/targets/$TARGET/rules" \
		cidr "$CIDR" "$rev_after_add" | jq -e '.saved == true' >/dev/null
	CIDR_ADDED=
	json_get /config/cidr | jq -j '.content' >"$TMP/cidr.after"
	cmp -s "$TMP/cidr.before" "$TMP/cidr.after" \
		|| fail_roundtrip cidr "$TMP/cidr.before" "$TMP/cidr.after"

	rev=$(revision cidr)
	CIDR_ADDED=1
	json_request POST "/config/cidr/targets/$TARGET/rules" \
		"$(group_rule_body cidr "$CIDR" "$GROUP_COMMENT")" "$rev" | jq -e '.saved == true and .applied == false' >/dev/null
	rev_after_group_add=$(revision cidr)
	json_get /config/cidr | jq -j '.content' >"$TMP/cidr.group"
	grep -F "##$GROUP_COMMENT" "$TMP/cidr.group" >/dev/null \
		|| fail "grouped CIDR write did not create comment group"
	grep -F "$CIDR" "$TMP/cidr.group" >/dev/null \
		|| fail "grouped CIDR write did not create rule"
	delete_rule "/config/cidr/targets/$TARGET/rules" \
		cidr "$CIDR" "$rev_after_group_add" "$GROUP_COMMENT" | jq -e '.saved == true' >/dev/null
	CIDR_ADDED=
	json_get /config/cidr | jq -j '.content' >"$TMP/cidr.after_group"
	cmp -s "$TMP/cidr.before" "$TMP/cidr.after_group" \
		|| fail_roundtrip cidr "$TMP/cidr.before" "$TMP/cidr.after_group"

	printf '  - manual backup and audit\n'
	json_request POST /backups '{}' | jq -e '.id | length > 0' >/dev/null
	json_get '/audit?limit=20' | jq -e '[.events[] | select(.action == "config.patch" and .ok == true)] | length >= 4' >/dev/null
	printf 'router-smoke: PASS (reversible writes)\n'
	exit 0
fi

if [ "$MODE" = service ]; then
	printf 'router-smoke: service actions (%s)\n' "$SERVICE_ACTIONS"
	old_ifs=$IFS
	IFS=,
	for action in $SERVICE_ACTIONS; do
		IFS=$old_ifs
		case "$action" in
			start|stop|restart|reload) ;;
			*) fail "unsupported service action: $action" ;;
		esac
		printf '  - service %s\n' "$action"
		json_request POST "/service/$action" '{}' | jq -e --arg action "$action" '.action == $action and .ok == true' >/dev/null
		IFS=,
	done
	IFS=$old_ifs
	printf 'router-smoke: PASS (service actions)\n'
	exit 0
fi

printf 'router-smoke: reversible RCI policy mutation (%s)\n' "$POLICY"
json_get /runtime/policies | jq -e --arg name "$POLICY" '[.policies[].name] | index($name) == null' >/dev/null \
	|| fail "temporary policy already exists: $POLICY"
POLICY_ADDED=1
json_request POST /targets/policies "$(jq -cn --arg name "$POLICY" '{name:$name}')" \
	| jq -e '.ok == true and .saved == true' >/dev/null
json_get /runtime/policies | jq -e --arg name "$POLICY" '[.policies[].name] | index($name) != null' >/dev/null
curl -fsS -X DELETE -H "Authorization: Bearer $TOKEN" \
	"$BASE_URL/targets/policies/$POLICY" | jq -e '.ok == true and .saved == true' >/dev/null
POLICY_ADDED=
printf 'router-smoke: PASS (RCI policy mutation)\n'
