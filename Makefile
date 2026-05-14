.PHONY: default up

default: up

up:
	docker compose down
	docker compose build
	docker compose up -d
