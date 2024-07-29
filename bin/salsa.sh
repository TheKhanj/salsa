#!/usr/bin/env bash

PID=$$
PROC_DIR="/var/run/salsa/$PID"
[ "$USER" != 'root' ] && PROC_DIR="/run/user/$(id -u)/salsa/$PID"
SLEEP_INTERVAL=5
CHECK_PROXY=
PROXIES_ADDRESS=()
LISTENING_ADDRESS=
HELP=0

help() {
	exec man salsa
}

error() {
	echo "$1" >&2
}

parse_args() {
	while [ $# -gt 0 ]; do
		case "$1" in
		'-l')
			LISTENING_ADDRESS="$2"
			shift 2
			;;
		'-t')
			SLEEP_INTERVAL="$2"
			shift 2
			;;
		'-c')
			CHECK_PROXY="$2"
			shift 2
			;;
		'-h' | '--help')
			HELP=1
			shift
			;;
		*)
			PROXIES_ADDRESS+=("$1")
			shift
			;;
		esac
	done
}

validate_proxy_address() {
	local address="$1"

	[ -z "$address" ] && {
		error 'empty proxy address.'
		exit 3
	}
	echo "$address" |
		grep -E '^(([0-9a-z]+\.)*[0-9a-z]+|([0-9]{1,3}\.){3}[0-9]{1,3})?:[0-9]{1,5}$' >/dev/null ||
		{
			error "invalid proxy address. ($address)"
			exit 3
		}
}

validate_input() {
	for address in "${PROXIES_ADDRESS[@]}"; do
		validate_proxy_address "$address"
	done

	[ -z "$LISTENING_ADDRESS" ] && help

	echo "$LISTENING_ADDRESS" |
		grep -E '^(([0-9]{1,3}\.){3}[0-9]{1,3})?:[0-9]{1,5}$' >/dev/null ||
		{
			error "invalid listening address. ($LISTENING_ADDRESS)"
			exit 1
		}

	echo "$SLEEP_INTERVAL" |
		grep '^[0-9]\+$' >/dev/null ||
		{
			error "invalid sleep interval. ($SLEEP_INTERVAL)"
			exit 2
		}
}

get_host() {
	local ret="$(echo "$1" |
		grep -oP '^(([0-9a-z]+\.)*[0-9a-z]+|([0-9]{1,3}\.){3}[0-9]{1,3})?')"

	[ -z "$ret" ] && echo -n '0.0.0.0' || echo -n "$ret"
}

get_port() {
	echo "$1" |
		grep -oP '(?<=:)[0-9]{1,5}$'
}

check_proxy() {
	local proxy="$1"

	[ -z "$CHECK_PROXY" ] && {
		local port="$(get_port "$proxy")"
		ss -tnlp | grep :$port >/dev/null
		return $?
	}

	$CHECK_PROXY "$proxy"
}

check_proxies() {
	while true; do
		for proxy in "${PROXIES_ADDRESS[@]}"; do
			if check_proxy "$proxy"; then
				[ -f "$PROC_DIR/${proxy}.unhealthy" ] && {
					echo "proxy $proxy is back up"
					rm "${PROC_DIR}/${proxy}.unhealthy"
				}
			else
				[ -f "$PROC_DIR/${proxy}.unhealthy" ] || {
					echo "proxy $proxy is down"
					touch "${PROC_DIR}/${proxy}.unhealthy"
				}
			fi
		done

		sleep $SLEEP_INTERVAL
	done
}

parse_args "$@"
validate_input

[ $HELP -eq 1 ] && help

mkdir -p "$PROC_DIR"

listening_host="$(get_host "$LISTENING_ADDRESS")"
listening_port="$(get_port "$LISTENING_ADDRESS")"

error "process id is $PID"
error "listening on address $listening_host:$listening_port"
printf "forwarding requests to proxies %s\n" "${PROXIES_ADDRESS[@]}"

command="salsa-handler '$PROC_DIR' '${PROXIES_ADDRESS[@]}'"

check_proxies &
CHECK_PROXIES_PID=$!

socat TCP-LISTEN:$listening_port,bind=$listening_host,fork,reuseaddr \
	EXEC:"$command" >/dev/null 2>&1

kill -9 $CHECK_PROXIES_PID
rm -rf $PROC_DIR
