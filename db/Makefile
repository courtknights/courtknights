.PHONY: migrate rollback status

DB_URL ?= postgres://postgres:postgres@localhost:5432/courtknights?sslmode=disable
MIGRATIONS_DIR := ./migrations

## migrate: apply all pending migrations
migrate:
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" up

## rollback: revert the last applied migration
rollback:
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down 1

## status: show current migration version
status:
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_URL)" version
