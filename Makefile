

.PHONY: build up down test retest logs

build:
	docker compose build

up:
	docker compose up -d

down:
	docker compose down

test:
	go test -v ./...

retest: down up test

logs:
	docker compose logs -f