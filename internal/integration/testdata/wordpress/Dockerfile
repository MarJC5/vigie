# A shared hosting account for the integration tests: WordPress files, PHP 8.4, WP-CLI and an SSH
# server accepting one test key for the user "vigie"; Apache serves the fixture sites by name, as
# that user (PHP runs as the account on shared hosting). Never published.
FROM wordpress:7.1.3-php8.4-apache

ENV APACHE_RUN_USER=vigie APACHE_RUN_GROUP=vigie

ARG WP_CLI_VERSION=2.12.0
# Published next to the phar on the WP-CLI release page.
ARG WP_CLI_SHA512=be928f6b8ca1e8dfb9d2f4b75a13aa4aee0896f8a9a0a1c45cd5d2c98605e6172e6d014dda2e27f88c98befc16c040cbb2bd1bfa121510ea5cdf5f6a30fe8832

RUN apt-get update \
 && apt-get install -y --no-install-recommends openssh-server mariadb-client curl ca-certificates less \
 && rm -rf /var/lib/apt/lists/* \
 && curl -fsSL -o /usr/local/bin/wp \
    "https://github.com/wp-cli/wp-cli/releases/download/v${WP_CLI_VERSION}/wp-cli-${WP_CLI_VERSION}.phar" \
 && echo "${WP_CLI_SHA512}  /usr/local/bin/wp" | sha512sum -c - \
 && chmod 0755 /usr/local/bin/wp \
 && useradd --create-home --shell /bin/bash vigie \
 && install -d -m 0700 -o vigie -g vigie /home/vigie/.ssh \
 && mkdir -p /run/sshd \
 && ssh-keygen -A \
 && a2enmod vhost_alias \
 # "nodump": the same account (user ID and home) with a PATH that has every tool but the database
 # dump ones, like a host without mysqldump.
 && useradd --non-unique --uid "$(id -u vigie)" --gid vigie --home-dir /home/vigie --no-create-home \
    --shell /bin/bash nodump \
 && mkdir -p /opt/nodump/bin \
 && for f in /usr/local/bin/* /usr/bin/*; do \
      case "${f##*/}" in mariadb-dump|mysqldump) ;; *) ln -sf "$f" /opt/nodump/bin/ ;; esac; \
    done

COPY sshd.conf /etc/ssh/sshd_config.d/vigie.conf
COPY sites.conf /etc/apache2/sites-available/fixtures.conf
RUN a2ensite fixtures
COPY fixtures.sh /usr/local/bin/vigie-fixtures
COPY start.sh /usr/local/bin/vigie-start
RUN chmod 0755 /usr/local/bin/vigie-fixtures /usr/local/bin/vigie-start

EXPOSE 22 80
CMD ["vigie-start"]
