APP_NAME := dnsmanagerd

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: test
test:
	go test ./...

.PHONY: web-build
web-build:
	cd web && npm run build

.PHONY: compose-config
compose-config:
	docker compose config

