#!/bin/sh
set -eu
umask 077
if [ "${#INTERNAL_PROXY_SECRET}" -ne 64 ] || ! printf '%s' "$INTERNAL_PROXY_SECRET" | grep -Eq '^[0-9a-f]{64}$'; then
  echo 'INTERNAL_PROXY_SECRET must be 64 hexadecimal characters' >&2
  exit 1
fi
TRUSTED_PROXY_CIDR=${TRUSTED_PROXY_CIDR:-127.0.0.1/32}
export TRUSTED_PROXY_CIDR
if ! printf '%s' "$TRUSTED_PROXY_CIDR" | grep -Eq '^[0-9a-fA-F:.]+(/[0-9]{1,3})?$'; then
  echo 'TRUSTED_PROXY_CIDR must be one trusted proxy address or CIDR' >&2
  exit 1
fi
# Restrict substitution: nginx's own $variables must remain untouched.
envsubst '${INTERNAL_PROXY_SECRET} ${TRUSTED_PROXY_CIDR}' < /etc/orden/nginx.conf.template > /tmp/nginx.conf
exec nginx -c /tmp/nginx.conf -g 'daemon off;'
