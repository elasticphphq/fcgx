FROM php:8.2-fpm

# Install required packages and clean up
RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        net-tools \
        iproute2 \
    && rm -rf /var/lib/apt/lists/*

# Configure PHP-FPM
RUN sed -i '/^listen =/d' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^clear_env *=/c\clear_env = no' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^listen.allowed_clients *=/c\listen.allowed_clients = 127.0.0.1' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/c\php_admin_value[error_log] = /proc/self/fd/2' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^include=/i log_level = debug' /usr/local/etc/php-fpm.conf \
    && sed -i '/^request_terminate_timeout *=/c\request_terminate_timeout = 0' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^request_slowlog_timeout *=/c\request_slowlog_timeout = 0' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[post_max_size] = 8M' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[upload_max_filesize] = 8M' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[max_input_time] = 60' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[max_execution_time] = 60' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[display_errors] = On' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[display_startup_errors] = On' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[error_reporting] = E_ALL' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[log_errors] = On' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[error_log] = /proc/self/fd/2' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[fastcgi.logging] = 1' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[fastcgi.error_log] = /proc/self/fd/2' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[fastcgi.impersonate] = 0' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[fastcgi.fix_pathinfo] = 1' /usr/local/etc/php-fpm.d/www.conf \
    && sed -i '/^php_admin_value\[error_log\] *=/a\php_admin_value[fastcgi.buffering] = 0' /usr/local/etc/php-fpm.d/www.conf

# Ensure www-data user exists and has proper permissions
RUN if ! id www-data > /dev/null 2>&1; then \
        useradd -m -s /bin/bash www-data; \
    fi \
    && chown -R www-data:www-data /var/www/html

# Switch to non-root user
USER www-data

EXPOSE 9000

CMD ["php-fpm", "--nodaemonize", "--force-stderr"]