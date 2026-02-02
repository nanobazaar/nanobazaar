GO ?= go
RELAY_DIR := apps/relay
DB_PATH ?= $(if $(NBR_DB_PATH),$(NBR_DB_PATH),./data/relay.db)
SQLC_CGO_FLAGS ?= -DHAVE_STRCHRNUL
GO_BUILD_TAGS ?= sqlite_fts5
GO_BUILD_FLAGS := -tags=$(GO_BUILD_TAGS)

.PHONY: fmt lint test db/migrate db/sqlc run fly/migrate fly/migrate/dry-run fly/migrate/deploy fly/migrate/deploy/dry-run fly/deploy fly/deploy/dry-run

fmt:
	@gofmt -w $$(find $(RELAY_DIR) -name '*.go')

lint:
	@cd $(RELAY_DIR) && $(GO) vet $(GO_BUILD_FLAGS) ./...

test:
	@$(GO) test $(GO_BUILD_FLAGS) ./...

db/migrate:
	@cd $(RELAY_DIR) && $(GO) run $(GO_BUILD_FLAGS) github.com/pressly/goose/v3/cmd/goose -dir db/migrations sqlite3 $(DB_PATH) up

db/sqlc:
	@cd $(RELAY_DIR) && CGO_CFLAGS="${SQLC_CGO_FLAGS}" $(GO) run $(GO_BUILD_FLAGS) github.com/sqlc-dev/sqlc/cmd/sqlc generate -f db/sqlc.yaml

run:
	@cd $(RELAY_DIR) && NBR_DB_PATH=$(DB_PATH) $(GO) run $(GO_BUILD_FLAGS) ./cmd/relay

fly/migrate:
	@scripts/fly_migrate.sh

fly/migrate/dry-run:
	@scripts/fly_migrate.sh --dry-run

fly/migrate/deploy:
	@scripts/fly_migrate_and_deploy.sh

fly/migrate/deploy/dry-run:
	@scripts/fly_migrate_and_deploy.sh --dry-run

fly/deploy:
	@scripts/fly_deploy.sh

fly/deploy/dry-run:
	@scripts/fly_deploy.sh --dry-run
