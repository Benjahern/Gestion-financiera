FROM golang:1.26.10-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=${TARGETARCH:-$(go env GOARCH)} go build -trimpath -ldflags="-s -w" -o /orden ./cmd/orden

FROM alpine:3.23
RUN apk upgrade --no-cache && apk add --no-cache ca-certificates && addgroup -g 10001 orden && adduser -D -u 10001 -G orden orden
COPY --from=build /orden /usr/local/bin/orden
USER 10001:10001
EXPOSE 8080
ENTRYPOINT ["orden"]
CMD ["serve"]
