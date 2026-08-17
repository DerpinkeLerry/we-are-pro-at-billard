FROM php:8.4-apache
RUN apt-get update \
 && apt-get install -y --no-install-recommends libpq-dev libonig-dev \
 && docker-php-ext-install pdo_pgsql mbstring \
 && a2enmod rewrite headers \
 && rm -rf /var/lib/apt/lists/*
ENV APACHE_DOCUMENT_ROOT=/var/www/html/public
RUN sed -ri -e 's!/var/www/html!${APACHE_DOCUMENT_ROOT}!g' /etc/apache2/sites-available/*.conf \
 && sed -ri -e 's!/var/www/!${APACHE_DOCUMENT_ROOT}!g' /etc/apache2/apache2.conf /etc/apache2/conf-available/*.conf
COPY web /var/www/html
COPY config /var/www/html/public/config
COPY database /var/www/database
RUN chown -R www-data:www-data /var/www/html /var/www/database
EXPOSE 80
CMD ["bash", "-lc", "php /var/www/html/bin/migrate.php && exec apache2-foreground"]
