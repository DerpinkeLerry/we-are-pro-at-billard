FROM golang:1.23-bookworm AS game-build
WORKDIR /src/game-server
COPY game-server/go.mod game-server/go.sum ./
RUN go mod download
COPY game-server ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/pool-server ./cmd/pool-server

FROM php:8.4-apache
RUN apt-get update \
 && apt-get install -y --no-install-recommends libsqlite3-dev libonig-dev ca-certificates \
 && docker-php-ext-install pdo_sqlite mbstring \
 && a2enmod rewrite headers proxy proxy_http proxy_wstunnel \
 && rm -rf /var/lib/apt/lists/*

ENV APP_ENV=production \
    CONFIG_DIR=/app/config \
    SQLITE_PATH=/var/lib/pool/pool.sqlite

COPY web /var/www/html
COPY config /app/config
COPY config /var/www/html/public/config
COPY database /var/www/database
COPY --from=game-build /out/pool-server /usr/local/bin/pool-server
COPY docker/single-service/000-default.conf /etc/apache2/sites-available/000-default.conf
COPY docker/single-service/apache-pool.conf /etc/apache2/conf-available/pool-single.conf
COPY docker/single-service/entrypoint.sh /usr/local/bin/pool-entrypoint

RUN a2enconf pool-single \
 && mkdir -p /var/lib/pool \
 && chown -R www-data:www-data /var/www/html /var/www/database /var/lib/pool \
 && chmod +x /usr/local/bin/pool-entrypoint

EXPOSE 10000
CMD ["/usr/local/bin/pool-entrypoint"]
