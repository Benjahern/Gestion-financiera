FROM node:24-alpine AS build
RUN npm install -g pnpm@11.28.5
WORKDIR /app
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY frontend/package.json ./frontend/package.json
RUN pnpm install --frozen-lockfile
COPY frontend ./frontend
COPY .figma/make/site.json ./.figma/make/site.json
RUN pnpm --filter finance-frontend typecheck && pnpm --filter finance-frontend build

# No Node, Vite, source code, or build-time secrets in the runtime image.
FROM alpine:3.23
RUN apk upgrade --no-cache && apk add --no-cache nginx gettext-envsubst \
    && addgroup -g 10001 orden && adduser -D -u 10001 -G orden orden
COPY --from=build /app/dist /usr/share/nginx/html
COPY deploy/nginx.conf.template /etc/orden/nginx.conf.template
COPY deploy/web-entrypoint.sh /usr/local/bin/orden-web
RUN chmod 755 /usr/local/bin/orden-web
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/orden-web"]
