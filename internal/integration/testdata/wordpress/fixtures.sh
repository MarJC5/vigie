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
# A must-use plugin and a dropin: WP-CLI lists them with "update": false, a boolean.
mkdir -p "$SITES/outdated/wp-content/mu-plugins"
printf '%s\n' '<?php' '/**' ' * Plugin Name: Fixture Must-Use' ' * Version: 1.0.0' ' */' \
	> "$SITES/outdated/wp-content/mu-plugins/fixture-must-use.php"
printf '%s\n' '<?php' '// Fixture dropin: a page cache that does nothing.' > "$SITES/outdated/wp-content/advanced-cache.php"
# French translations made to look old, so translation updates are pending: WordPress compares
# the PO-Revision-Date of the installed .po files with wordpress.org.
wp language core install fr_FR --path="$SITES/outdated" --quiet
wp language plugin install akismet fr_FR --path="$SITES/outdated" --quiet
find "$SITES/outdated/wp-content/languages" -name '*.po' \
	-exec sed -i 's/^"PO-Revision-Date: .*\\n"$/"PO-Revision-Date: 2001-01-01 00:00:00+0000\\n"/' {} +

# The only theme lives in a plugin's folder, registered with register_theme_directory, and
# wp-content/themes is empty: without the plugins, WordPress sees no theme.
new_site "$SITES/plugin-theme" fixture_plugin_theme
wp db create --path="$SITES/plugin-theme" --quiet
wp core install --path="$SITES/plugin-theme" --url=https://plugin-theme.example.test --title="Plugin theme" $ADMIN
FIXTURE_THEMES="$SITES/plugin-theme/wp-content/plugins/fixture-themes"
mkdir -p "$FIXTURE_THEMES/themes/fixture-theme"
printf '%s\n' '<?php' '/**' ' * Plugin Name: Fixture Themes' ' * Version: 1.0.0' ' */' \
	"register_theme_directory( __DIR__ . '/themes' );" > "$FIXTURE_THEMES/fixture-themes.php"
printf '%s\n' '/*' 'Theme Name: Fixture Theme' 'Version: 1.0.0' '*/' > "$FIXTURE_THEMES/themes/fixture-theme/style.css"
printf '%s\n' '<?php' > "$FIXTURE_THEMES/themes/fixture-theme/index.php"
find "$SITES/plugin-theme/wp-content/themes" -mindepth 1 -maxdepth 1 -type d -exec rm -r {} +
wp plugin activate fixture-themes --path="$SITES/plugin-theme" --quiet
wp theme activate fixture-theme --path="$SITES/plugin-theme" --quiet

# Updates, served over HTTP as http://updates.example.test: older versions from wordpress.org, and
# plugins offering their own updates as premium plugins do (WordPress sees an offer only with the
# plugin loaded), their packages served by this container: fixture-pro 1.1.0 installs fine,
# fixture-breaks 2.0.0 throws a fatal error, logged in debug.log, on every page, and the package of
# fixture-missing 9.9.9 does not exist.
new_site "$SITES/updates" fixture_updates
wp db create --path="$SITES/updates" --quiet
wp core install --path="$SITES/updates" --url=http://updates.example.test --title=Updates $ADMIN
for name in WP_DEBUG WP_DEBUG_LOG; do wp config set $name true --raw --path="$SITES/updates" --quiet; done
wp config set WP_DEBUG_DISPLAY false --raw --path="$SITES/updates" --quiet
wp plugin install akismet --version=5.3 --force --activate --path="$SITES/updates" --quiet
wp theme install twentytwentyfour --version=1.0 --force --path="$SITES/updates" --quiet
wp language core install fr_FR --path="$SITES/updates" --quiet
wp language plugin install akismet fr_FR --path="$SITES/updates" --quiet
find "$SITES/updates/wp-content/languages" -name '*.po' \
	-exec sed -i 's/^"PO-Revision-Date: .*\\n"$/"PO-Revision-Date: 2001-01-01 00:00:00+0000\\n"/' {} +

# In the web root of the default site: a volume of the image, so made here rather than at build.
PACKAGES=/var/www/html/fixture-packages
mkdir -p "$PACKAGES"
# plugin_file <file> <slug> <name> <version> [<offered version>]: the main file of a fixture plugin;
# with an offered version, it offers that update, its package at $PACKAGES.
plugin_file() {
	printf '%s\n' '<?php' '/**' " * Plugin Name: $3" " * Version: $4" ' */' > "$1"
	[ -n "${5:-}" ] || return 0
	sed -e "s/__SLUG__/$2/g" -e "s/__VERSION__/$5/g" >> "$1" <<'PHP'
// Offers its own update, as premium plugins do: WordPress sees it only with the plugin loaded.
add_filter( 'site_transient_update_plugins', function ( $data ) {
	$data = is_object( $data ) ? $data : new stdClass();
	$data->response['__SLUG__/__SLUG__.php'] = (object) array(
		'slug'        => '__SLUG__',
		'plugin'      => '__SLUG__/__SLUG__.php',
		'new_version' => '__VERSION__',
		'package'     => 'http://127.0.0.1/fixture-packages/__SLUG__-__VERSION__.zip',
	);
	return $data;
} );
// The package comes from this container, which WordPress refuses by default.
add_filter( 'http_request_host_is_external', function ( $external, $host ) {
	return $external || '127.0.0.1' === $host;
}, 10, 2 );
PHP
}
# package <slug> <version> <main file>: the zip of a plugin version, as WordPress installs it.
package() {
	php -r '$z = new ZipArchive(); $z->open($argv[1], ZipArchive::CREATE | ZipArchive::OVERWRITE);
		$z->addFile($argv[3], $argv[2] . "/" . $argv[2] . ".php"); exit($z->close() ? 0 : 1);' \
		"$PACKAGES/$1-$2.zip" "$1" "$3"
}
WORK=$(mktemp -d)
plugin_file "$WORK/pro-1.1.0.php" fixture-pro "Fixture Pro" 1.1.0
package fixture-pro 1.1.0 "$WORK/pro-1.1.0.php"
plugin_file "$WORK/breaks-2.0.0.php" fixture-breaks "Fixture Breaks" 2.0.0
echo 'fixture_breaks_missing_function();' >> "$WORK/breaks-2.0.0.php"
package fixture-breaks 2.0.0 "$WORK/breaks-2.0.0.php"
for slug in fixture-pro fixture-breaks fixture-missing; do
	mkdir -p "$SITES/updates/wp-content/plugins/$slug"
done
plugin_file "$SITES/updates/wp-content/plugins/fixture-pro/fixture-pro.php" fixture-pro "Fixture Pro" 1.0.0 1.1.0
plugin_file "$SITES/updates/wp-content/plugins/fixture-breaks/fixture-breaks.php" fixture-breaks "Fixture Breaks" 1.0.0 2.0.0
# Offers a version whose package does not exist: the update fails to download.
plugin_file "$SITES/updates/wp-content/plugins/fixture-missing/fixture-missing.php" fixture-missing "Fixture Missing" 1.0.0 9.9.9
# Version 1.0.0 of fixture-breaks, to put it back after a test broke the site.
package fixture-breaks 1.0.0 "$SITES/updates/wp-content/plugins/fixture-breaks/fixture-breaks.php"
rm -r "$WORK"
wp plugin activate fixture-pro fixture-breaks fixture-missing --path="$SITES/updates" --quiet

# What a security scan must find. WordPress files: one changed, one added where only WordPress's
# own files belong, one missing. Akismet, from wordpress.org: one file changed, one added. A plugin
# wordpress.org does not know, skipped by the checksums. PHP files among the uploads.
new_site "$SITES/security" fixture_security
wp db create --path="$SITES/security" --quiet
wp core install --path="$SITES/security" --url=https://security.example.test --title=Security $ADMIN
echo '// fixture: changed' >> "$SITES/security/wp-includes/functions.php"
printf '%s\n' '<?php' > "$SITES/security/wp-includes/fixture-added.php"
rm "$SITES/security/wp-admin/freedoms.php"
wp plugin install akismet --force --activate --path="$SITES/security" --quiet
echo '// fixture: changed' >> "$SITES/security/wp-content/plugins/akismet/akismet.php"
printf '%s\n' '<?php' > "$SITES/security/wp-content/plugins/akismet/fixture-added.php"
mkdir -p "$SITES/security/wp-content/plugins/fixture-premium"
printf '%s\n' '<?php' '/**' ' * Plugin Name: Fixture Premium' ' * Version: 1.0.0' ' */' \
	> "$SITES/security/wp-content/plugins/fixture-premium/fixture-premium.php"
mkdir -p "$SITES/security/wp-content/uploads/2026/10"
printf '%s\n' '<?php' > "$SITES/security/wp-content/uploads/2026/10/fixture-shell.php"
printf '%s\n' '<?php' > "$SITES/security/wp-content/uploads/fixture-old.PHTML"
printf 'JFIF' > "$SITES/security/wp-content/uploads/2026/10/photo.jpg"
# Administrators, in this order (IDs 2 to 5), with each way of storing two-factor; their secrets
# are fake, and Vigie must never read them. User 6 is an editor: not an administrator.
for login in marie totp-admin wp2fa-admin wf-admin; do
	wp user create $login $login@example.test --role=administrator --user_pass=fixture-only --path="$SITES/security" --quiet
done
wp user create redacteur redacteur@example.test --role=editor --user_pass=fixture-only --path="$SITES/security" --quiet
# Two Factor (and Solid Security, which uses the same keys).
wp user meta update totp-admin _two_factor_enabled_providers '["Two_Factor_Totp","Two_Factor_Backup_Codes"]' --format=json --path="$SITES/security" --quiet
wp user meta update totp-admin _two_factor_provider Two_Factor_Totp --path="$SITES/security" --quiet
wp user meta update totp-admin _two_factor_totp_key fixture-totp-secret --path="$SITES/security" --quiet
wp user meta update redacteur _two_factor_enabled_providers '["Two_Factor_Email"]' --format=json --path="$SITES/security" --quiet
# WP 2FA.
wp user meta update wp2fa-admin wp_2fa_enabled_methods totp --path="$SITES/security" --quiet
wp user meta update wp2fa-admin wp_2fa_totp_key fixture-totp-secret --path="$SITES/security" --quiet
# Wordfence keeps no user meta: a table of secrets, as Wordfence 10 creates it. Its plugin is a
# stand-in under its slug, active, since Vigie reads the table only when Wordfence is.
mkdir -p "$SITES/security/wp-content/plugins/wordfence"
printf '%s\n' '<?php' '/**' ' * Plugin Name: Wordfence (fixture)' ' * Version: 0.0.1' ' */' \
	> "$SITES/security/wp-content/plugins/wordfence/wordfence.php"
wp plugin activate wordfence --path="$SITES/security" --quiet
wp db query "CREATE TABLE wp_wfls_2fa_secrets (id INT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
	user_id BIGINT UNSIGNED NOT NULL, secret TINYBLOB NOT NULL, recovery BLOB NOT NULL, ctime INT UNSIGNED NOT NULL,
	vtime INT UNSIGNED NOT NULL, mode ENUM('authenticator') NOT NULL DEFAULT 'authenticator')" --path="$SITES/security"
wp db query "INSERT INTO wp_wfls_2fa_secrets (user_id, secret, recovery, ctime, vtime) VALUES (5, 'fixture-totp-secret', '', 0, 0)" --path="$SITES/security"
# Settings: WP_DEBUG on with WP_DEBUG_DISPLAY left to its default (errors shown), file editing
# allowed (DISALLOW_FILE_EDIT not set), hidden from search engines, 900 KB of autoloaded options
# (Site Health's limit is 800,000 bytes), and a WP-Cron event overdue since 2020.
wp config set WP_DEBUG true --raw --path="$SITES/security" --quiet
wp option update blog_public 0 --path="$SITES/security" --quiet
wp eval 'add_option( "fixture_big", str_repeat( "x", 900000 ), "", "yes" );' --path="$SITES/security"
wp cron event schedule fixture_overdue 2020-01-01 --path="$SITES/security" --quiet

# A server without outbound HTTP, like hosts that block it: every update check fails.
new_site "$SITES/offline" fixture_offline
wp db create --path="$SITES/offline" --quiet
wp core install --path="$SITES/offline" --url=https://offline.example.test --title=Offline $ADMIN
wp config set WP_HTTP_BLOCK_EXTERNAL true --raw --path="$SITES/offline" --quiet

# WordPress files whose database cannot be reached: WP-CLI cannot load WordPress.
new_site "$SITES/broken" fixture_broken
sed -i "s/define( 'DB_PASSWORD', '[^']*' );/define( 'DB_PASSWORD', 'wrong-password' );/" "$SITES/broken/wp-config.php"
