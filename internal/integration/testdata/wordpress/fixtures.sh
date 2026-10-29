#!/bin/sh
# Creates the test sites in the home of "vigie"; run once as that user, with the database host,
# user and password as arguments. Every name and address is fictional.
set -eu
DB_HOST=$1 DB_USER=$2 DB_PASS=$3
SITES=$HOME/sites
ADMIN="--admin_user=admin --admin_password=fixture-only --admin_email=admin@example.test --skip-email"

# new_site <root> <database> [<config directory>]: WordPress files and a configuration.
new_site() {
	mkdir -p "$1"
	cp -R /usr/src/wordpress/. "$1"
	wp config create --path="$1" --dbname="$2" --dbuser="$DB_USER" --dbpass="$DB_PASS" \
		--dbhost="$DB_HOST" --skip-check --quiet
	if [ -n "${3:-}" ]; then mv "$1/wp-config.php" "$3/wp-config.php"; fi
}

# A plain site, with a stray copy of WordPress files under node_modules that discovery must skip.
new_site "$SITES/plain" fixture_plain
wp db create --path="$SITES/plain" --quiet
wp core install --path="$SITES/plain" --url=https://plain.example.test --title=Plain $ADMIN
mkdir -p "$SITES/plain/node_modules/stray/wp-includes"
cp "$SITES/plain/wp-includes/version.php" "$SITES/plain/node_modules/stray/wp-includes/"

# wp-config.php one level above the WordPress root, which WordPress also loads.
mkdir -p "$SITES/parent-config"
new_site "$SITES/parent-config/public" fixture_parent "$SITES/parent-config"
wp db create --path="$SITES/parent-config/public" --quiet
wp core install --path="$SITES/parent-config/public" --url=https://parent.example.test --title=Parent $ADMIN

# A subdirectory multisite network: the main site and three subsites, one archived.
new_site "$SITES/network" fixture_network
wp db create --path="$SITES/network" --quiet
wp core multisite-install --path="$SITES/network" --url=https://network.example.test --title=Network $ADMIN
for slug in alpha beta gamma; do
	wp site create --path="$SITES/network" --slug=$slug --title="$slug" --quiet
done
wp site archive 4 --path="$SITES/network" --quiet

# Older versions from wordpress.org, so updates are pending.
new_site "$SITES/outdated" fixture_outdated
wp db create --path="$SITES/outdated" --quiet
wp core install --path="$SITES/outdated" --url=https://outdated.example.test --title=Outdated $ADMIN
wp plugin install akismet --version=5.3 --force --path="$SITES/outdated" --quiet
wp theme install twentytwentyfour --version=1.0 --force --path="$SITES/outdated" --quiet
# A plugin that is not on wordpress.org, like a premium one.
mkdir -p "$SITES/outdated/wp-content/plugins/fixture-premium"
printf '%s\n' '<?php' '/**' ' * Plugin Name: Fixture Premium' ' * Version: 1.0.0' ' */' \
	> "$SITES/outdated/wp-content/plugins/fixture-premium/fixture-premium.php"
wp plugin activate fixture-premium --path="$SITES/outdated" --quiet
# French translations made to look old, so translation updates are pending: WordPress compares
# the PO-Revision-Date of the installed .po files with wordpress.org.
wp language core install fr_FR --path="$SITES/outdated" --quiet
wp language plugin install akismet fr_FR --path="$SITES/outdated" --quiet
find "$SITES/outdated/wp-content/languages" -name '*.po' \
	-exec sed -i 's/^"PO-Revision-Date: .*\\n"$/"PO-Revision-Date: 2001-01-01 00:00:00+0000\\n"/' {} +

# A server without outbound HTTP, like hosts that block it: every update check fails.
new_site "$SITES/offline" fixture_offline
wp db create --path="$SITES/offline" --quiet
wp core install --path="$SITES/offline" --url=https://offline.example.test --title=Offline $ADMIN
wp config set WP_HTTP_BLOCK_EXTERNAL true --raw --path="$SITES/offline" --quiet

# WordPress files whose database cannot be reached: WP-CLI cannot load WordPress.
new_site "$SITES/broken" fixture_broken
sed -i "s/define( 'DB_PASSWORD', '[^']*' );/define( 'DB_PASSWORD', 'wrong-password' );/" "$SITES/broken/wp-config.php"
