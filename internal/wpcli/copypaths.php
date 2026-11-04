<?php
/*
 * Vigie: where the parts of a site are, for a copy of it on the user's computer (ADR-0041).
 *
 * Sent to `wp eval-file -` before a copy, with plugins and themes not loaded. It only reads:
 * WordPress's constants and the reading functions listed in copy_test.go, and prints one JSON
 * document. wp_upload_dir is asked not to create the uploads folder.
 */

$vigie_uploads = wp_upload_dir( null, false );
echo wp_json_encode(
	array(
		'root'      => untrailingslashit( ABSPATH ),
		'content'   => WP_CONTENT_DIR,
		'plugins'   => WP_PLUGIN_DIR,
		'muPlugins' => WPMU_PLUGIN_DIR,
		'themes'    => get_theme_root(),
		'uploads'   => $vigie_uploads['basedir'],
		'prefix'    => $GLOBALS['wpdb']->base_prefix,
	)
);
