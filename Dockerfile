FROM node:22-alpine AS ui-builder

WORKDIR /src/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS go-builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
COPY --from=ui-builder /src/web/dist ./web/dist

RUN CGO_ENABLED=0 go build -o /out/dnsmanagerd ./cmd/dnsmanagerd
RUN CGO_ENABLED=0 go build -o /out/dnsmanager ./cmd/dnsmanager

FROM alpine:3.21

RUN apk add --no-cache ca-certificates

WORKDIR /app

ENV DNSMANAGER_HTTP_ADDR=:8080
ENV DNSMANAGER_DATA_DIR=/var/lib/dnsmanager/data
ENV DNSMANAGER_CONFIG_DIR=/var/lib/dnsmanager/config
ENV DNSMANAGER_CONTENT_DIR=/var/lib/dnsmanager/content
ENV DNSMANAGER_UI_DIST_DIR=/app/web/dist
ENV DNSMANAGER_DB_PATH=/var/lib/dnsmanager/data/dnsmanager.db

COPY --from=go-builder /out/dnsmanagerd /app/dnsmanagerd
COPY --from=go-builder /out/dnsmanager /usr/local/bin/dnsmanager
COPY --from=ui-builder /src/web/dist /app/web/dist
COPY db/schema.sql /app/db/schema.sql

VOLUME ["/var/lib/dnsmanager/data", "/var/lib/dnsmanager/config", "/var/lib/dnsmanager/content"]

EXPOSE 8080

ENTRYPOINT ["/app/dnsmanagerd"]
