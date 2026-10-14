#!/bin/sh
set -eu
# psql variable quoting prevents password characters from becoming SQL.
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  --set=app_password="$ORDEN_DATABASE_PASSWORD" <<'SQL'
CREATE ROLE orden LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD :'app_password';
ALTER DATABASE orden OWNER TO orden;
GRANT CREATE ON SCHEMA public TO orden;
SQL
