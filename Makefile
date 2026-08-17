.PHONY: up down test test-go test-php lint package
up:
	docker compose up --build

down:
	docker compose down

test: test-go test-php

test-go:
	cd game-server && go test ./...

test-php:
	find web -name '*.php' -print0 | xargs -0 -n1 php -l

package:
	rm -f pool-arena.zip && zip -qr pool-arena.zip . -x '.git/*' -x 'node_modules/*'
