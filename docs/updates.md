# Updates: what Vigie does on a server

Vigie updates a site over SSH with WP-CLI, like the scans. Nothing is installed on the site. This
page lists what an update writes on the server, how long it keeps it, and how to remove it.

## Before anything is written

- Vigie reads the preview first: the updates WordPress offers, with the plugins loaded, so premium
  plugins show their own updates.
- The server must have a database dump tool (`mariadb-dump` or `mysqldump`). Without one, nothing
  is updated: Vigie dumps the database before every update.
- Only one update or rollback runs on a site at a time.
- An update can be **ignored** from the Updates tab, until a newer version is offered, or
  permanently with a reason (a version you keep on purpose). It then counts nowhere (status,
  counts, notifications), **Update all** and `vigie update` leave it out and say so, and the site's
  own update installs it only when you tick it. The ignores stay on your computer, with the others.
- Each update offered links to its changelog when wordpress.org has one: a plugin's Development
  tab, WordPress's release notes. Your browser opens it; Vigie fetches nothing.

## The steps of an update

1. **Backups**, in a new folder of the account (see below): the database dump, then an archive of
   the folder of each plugin or theme to update, and of the WordPress files when WordPress itself is
   updated.
2. **WordPress itself** first, if chosen: in maintenance mode (the `.maintenance` file of WordPress,
   through WP-CLI), then its database upgrade; the maintenance mode is always turned off at the end.
3. **The plugins, then the themes**, one at a time.
4. **The translations** last.

After each item, Vigie **checks the site**: the home page, `wp-login.php` and the subsites pinned in
a network must not answer a server error (5xx), WordPress's critical error page, or nothing at all,
and no new `PHP Fatal error` may appear in `debug.log` (when WordPress keeps one). Redirects and
4xx answers pass. The first check comes 3 seconds after the item, then one each second for 3
seconds, so PHP's OPcache serves the new code.

The update **stops at the first failure**: the next items are not started. Stopping a run from the
window or with Ctrl-C lets the current item end; an item is never left halfway.

## What Vigie writes on the server

The backups of one update go in a folder of the SSH account's home directory, outside the site's
folder:

```
~/.vigie/backups/<site ID>/<date and time, UTC>/
    database.sql.gz          the whole database, compressed
    plugin-<slug>.tar.gz     the folder of each plugin updated
    theme-<slug>.tar.gz      the folder of each theme updated
    core.tar.gz              the WordPress files, when WordPress is updated
```

- The folder is created readable by the account only (`umask 077`).
- `core.tar.gz` leaves out `wp-content`, `wp-config.php` (which holds the database password) and
  `.maintenance`.
- **The dump holds the whole database**: user accounts with their password hashes, orders, form
  entries. If the home directory of the account is itself served on the web, move the backups or
  remove them (below).
- The site ID is the one Vigie gave the site; the window shows the folder in the preview of an
  update and in the history of the site.

Besides the backups, an update writes only what the update itself changes (the files of the items,
the database upgrade of WordPress or of a plugin) and, for a WordPress update, the `.maintenance`
file while it runs.

## How long backups are kept

Vigie keeps a site's backups for **30 days**, and at most those of its **last 3 updates**. The time
is set in **Settings, Data**: 7, 30 or 90 days, or no limit (then the last 3 updates only). The
backups hold the data of the site's visitors and customers: they are not kept longer than needed.

At the end of each update, even a stopped one, Vigie removes the folders past these limits, for
every site of this Vigie on the same host, not only the one updated: one listing and the removals,
in its own folder only. A folder named after a site this Vigie does not know is left alone: it can
belong to Vigie on another computer.

Each scan reads a site's backups, the date and size of each update's, and only reads. The site's
**History** tab lists them; those past the limits can be removed at once with **Remove**, after a
confirmation. The history of the updates stays in Vigie's own database, on your computer.

## Rolling back

A rollback is offered on the latest update of a site, when it failed:

- **A plugin or theme**: its folder is removed, then put back from its archive; the site is checked
  again. The database is not touched: a plugin that changed its tables during the update keeps
  them.
- **WordPress itself**: its files and the database are put back as they were before the update, in
  maintenance mode. What was written to the database in between (orders, comments) is lost.

Only the item that failed is put back: the items updated before it keep their new version.

## Removing the backups

Deleting a site or a host from Vigie offers, checked, to remove its sites' backups on the server
too. If the server cannot be reached, the site is deleted from Vigie all the same, and Vigie gives
the command to run there.

The backups are ordinary files: you can also remove them over SSH.

```sh
rm -rf ~/.vigie/backups/<site ID>   # the backups of one site
rm -rf ~/.vigie                     # everything Vigie wrote on this account
```

A rollback is then no longer possible for those updates.
