FROM postgres:17-alpine
# Replace the unused Go runtime in gosu with Alpine's small native equivalent.
# This is the sole user-switch call in the upstream entrypoint.
RUN apk upgrade --no-cache && apk add --no-cache su-exec \
    && grep -q 'exec gosu postgres' /usr/local/bin/docker-entrypoint.sh \
    && sed -i 's/exec gosu postgres/exec su-exec postgres/' /usr/local/bin/docker-entrypoint.sh \
    && rm /usr/local/bin/gosu
COPY deploy/init-db.sh /docker-entrypoint-initdb.d/10-orden.sh
RUN chmod 755 /docker-entrypoint-initdb.d/10-orden.sh
