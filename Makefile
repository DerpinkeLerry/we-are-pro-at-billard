.PHONY: up down test test-go test-php test-js package

up:
	docker compose up --build

down:
	docker compose down

test: test-go test-php test-js

test-go:
	cd game-server && go test ./...

test-php:
	find web -name '*.php' -print0 | xargs -0 -n1 php -l

test-js:
	find web/public/assets/js -name '*.js' -print0 | xargs -0 -n1 node --check

package:
	rm -f pool-arena-single-service.zip && zip -qr pool-arena-single-service.zip . -x '.git/*' -x 'node_modules/*' -x '*.zip'
