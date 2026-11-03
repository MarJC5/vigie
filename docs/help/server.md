# What Vigie reads on a server, and what it sends

Vigie connects to your hosts over SSH, with the keys of your SSH agent, and never asks for a
password. A scan only reads: nothing is written on the server.

## What a scan reads

The versions of WordPress, PHP and the plugins and themes, the pending updates, the files compared
with wordpress.org, the administrators and their two-factor, a few settings of `wp-config.php`.
**Never** a password, a key of `wp-config.php` or the content of your database.

## What is asked from your computer

Some checks need an outside service: known vulnerabilities, plugins closed on wordpress.org, the
expiry of the domain and of the certificate. Vigie asks you first, before the first scan that
needs them.

- These services receive the names of your plugins and themes, the versions of WordPress and PHP,
  and your domain names. **Never** the versions of your plugins, your SSH details or anything read
  from your databases.
- Each one can be turned off in **Settings, Lookups outside**. Its checks are then "Not checked".

## When WP-CLI is missing

The connection test offers to install it in the account's home, `~/bin/wp`, after you confirm:
downloaded on your computer, checked, then sent. Nothing is written in your sites.

## What an update writes

Only the update itself, and its backups in `~/.vigie/backups/` (see "Update a site, and roll
back").

Learn more: [what Vigie reads, asks and sends](../security.md).
