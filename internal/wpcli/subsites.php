<?php
/*
 * Vigie: what each subsite of a network has active, and who administers it (ADR-0029).
 *
 * Sent to `wp eval-file -` once per scan of a network, with plugins and themes not loaded. It only
 * reads: it calls WordPress's reading functions, listed in subsites_test.go, and prints one JSON
 * document. Of two-factor, only which methods a user turned on, never a secret.
 */

$vigie_network = array(
	'plugins' => array_keys( (array) get_site_option( 'active_sitewide_plugins', array() ) ),
	'supers'  => array_values( (array) get_super_admins() ),
	'sites'   => array(),
);

// Every subsite but those archived, marked as spam or deleted; `number` 0 lifts the default 100.
$vigie_ids = get_sites(
	array(
		'number'   => 0,
		'fields'   => 'ids',
		'archived' => 0,
		'spam'     => 0,
		'deleted'  => 0,
	)
);

foreach ( $vigie_ids as $vigie_id ) {
	switch_to_blog( $vigie_id );
	$vigie_theme  = wp_get_theme();
	$vigie_admins = array();
	$vigie_users  = get_users(
		array(
			'blog_id' => $vigie_id,
			'role'    => 'administrator',
			'fields'  => array( 'ID', 'user_login', 'user_email', 'user_registered' ),
		)
	);
	foreach ( $vigie_users as $vigie_user ) {
		$vigie_admins[] = array(
			'id'         => (int) $vigie_user->ID,
			'login'      => $vigie_user->user_login,
			'email'      => $vigie_user->user_email,
			'registered' => $vigie_user->user_registered,
			'providers'  => get_user_meta( $vigie_user->ID, '_two_factor_enabled_providers', true ),
			'wp2fa'      => get_user_meta( $vigie_user->ID, 'wp_2fa_enabled_methods', true ),
		);
	}
	$vigie_network['sites'][] = array(
		'id'      => (int) $vigie_id,
		'plugins' => array_values( (array) get_option( 'active_plugins', array() ) ),
		'theme'   => array(
			'slug'    => $vigie_theme->get_stylesheet(),
			'name'    => $vigie_theme->get( 'Name' ),
			'version' => $vigie_theme->get( 'Version' ),
		),
		'email'   => get_option( 'admin_email' ),
		'admins'  => $vigie_admins,
	);
	restore_current_blog();
}

echo wp_json_encode( $vigie_network );
