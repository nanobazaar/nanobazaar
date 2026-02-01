GO ?= go
RELAY_DIR := apps/relay
DB_PATH ?= $(if $(NBR_DB_PATH),$(NBR_DB_PATH),./data/relay.db)
SQLC_CGO_FLAGS ?= -DHAVE_STRCHRNUL

.PHONY: fmt lint test db/migrate db/sqlc run

fmt:
	@gofmt -w $$(find $(RELAY_DIR) -name '*.go')

lint:
	@$(GO) vet ./...

test:
	@$(GO) test ./...

db/migrate:
	@cd $(RELAY_DIR) && $(GO) run github.com/pressly/goose/v3/cmd/goose -dir db/migrations sqlite3 $(DB_PATH) up

db/sqlc:
	@cd $(RELAY_DIR) && CGO_CFLAGS="${SQLC_CGO_FLAGS}" $(GO) run github.com/sqlc-dev/sqlc/cmd/sqlc generate -f db/sqlc.yaml

run:
	@cd $(RELAY_DIR) && NBR_DB_PATH=$(DB_PATH) $(GO) run ./cmd/relay
