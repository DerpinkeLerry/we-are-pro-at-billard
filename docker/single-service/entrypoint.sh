#!/usr/bin/env bash
set -euo pipefail

PUBLIC_PORT="${PORT:-10000}"
export APP_ENV="${APP_ENV:-production}"
export CONFIG_DIR="${CONFIG_DIR:-/app/config}"
export SQLITE_PATH="${SQLITE_PATH:-/var/lib/pool/pool.sqlite}"
export GAME_INTERNAL_URL="http://127.0.0.1:8081"

# No Render environment setup is required. Both secrets are created once for
# this container boot and inherited by PHP and Go.
if [[ -z "${JOIN_TOKEN_SECRET:-}" ]]; then
  export JOIN_TOKEN_SECRET="$(php -r 'echo bin2hex(random_bytes(32));')"
fi
if [[ -z "${GAME_INTERNAL_SECRET:-}" ]]; then
  export GAME_INTERNAL_SECRET="$(php -r 'echo bin2hex(random_bytes(32));')"
fi

mkdir -p "$(dirname "$SQLITE_PATH")"
touch "$SQLITE_PATH"
chown -R www-data:www-data "$(dirname "$SQLITE_PATH")"
chmod 0770 "$(dirname "$SQLITE_PATH")"
chmod 0660 "$SQLITE_PATH"

# Render requires the public process to bind to 0.0.0.0:$PORT.
sed -ri "s/^Listen [0-9]+$/Listen ${PUBLIC_PORT}/" /etc/apache2/ports.conf
sed -ri "s/<VirtualHost \*:[0-9]+>/<VirtualHost *:${PUBLIC_PORT}>/" /etc/apache2/sites-available/000-default.conf

su -s /bin/sh -c 'php /var/www/html/bin/migrate.php' www-data

apache2-foreground &
WEB_PID=$!

# Let Apache become available so the Go persistence adapter can reach the
# internal PHP persistence endpoints immediately.
sleep 0.5
PORT=8081 PERSISTENCE_URL="http://127.0.0.1:${PUBLIC_PORT}" /usr/local/bin/pool-server &
GAME_PID=$!

shutdown() {
  kill -TERM "$GAME_PID" "$WEB_PID" 2>/dev/null || true
  wait "$GAME_PID" 2>/dev/null || true
  wait "$WEB_PID" 2>/dev/null || true
}
trap shutdown TERM INT

set +e
wait -n "$WEB_PID" "$GAME_PID"
STATUS=$?
set -e
shutdown
exit "$STATUS"
