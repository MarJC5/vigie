# Copies of your sites on this computer

Vigie can copy a site to your computer: its database, and if you want its files. It reads the site
over SSH and writes nothing on the server. A copy is yours: Vigie never removes one on its own, and
never puts one back on a server. It does not replace your host's backups.

## What a copy holds

You choose at each copy, in the **Copies** tab of a site (or **Copy to this computer** in its menu):

- **The database**: posts, pages, settings, accounts and orders.
- **The database and wp-content** (the default): plus the plugins, themes and media. The dialog
  lists the folders of `wp-content` by size; untick those you do not want, such as a cache or the
  archives of another backup plugin.
- **The whole site**: plus WordPress itself and `wp-config.php`, which holds the database
  password.

Several sites selected are copied in one go, one after another on each host, with the same content
and no folder left out. A copy holds the site's personal data (accounts, orders, messages): keep it
as you keep the site.

## Where it goes

In **Settings, Data**, the folder of the copies, `~/Documents/Vigie` by default. Each copy has its
own folder, `<host>/<site>/<date and time>`, readable by you only. In it, the archives and
`copy.json`, which says what the copy holds: the site, its WordPress version and table prefix, what
was left out, and for each archive its size, its SHA-256 digest and when it was read.

Before a copy starts, Vigie compares its size with the free space there, and refuses when it does
not fit.

## Split into archives

Each part of the site has its own archives: `database.sql.gz`, `plugins.tar.gz`, `themes.tar.gz`,
`mu-plugins.tar.gz`, `uploads.tar.gz`, `others.tar.gz` (the rest of `wp-content`) and, for the
whole site, `wordpress.tar.gz`. A part larger than the size set in **Settings, Data** (500 MB by
default, 4 GB at most) is split: `uploads.tar.gz`, `uploads-2.tar.gz`, and so on. The database is
split by whole tables.

On an account without GNU find and GNU tar, each part is one archive, not split.

## A copy cut off

A copy cut off (network, sleep, **Stop**) keeps its complete archives and shows **Incomplete**.
**Resume** reads only the missing archives, the same files as the first time: the site may have
changed meanwhile, and `copy.json` keeps the time of each archive. An archive removed by hand from a
copy's folder counts as missing too.

## Removing a copy

The trash icon of a copy moves it to the Trash, where you can still get it back. Where no trash
takes it (a network disk, a Linux system without a desktop), Vigie says so and offers to delete it
for good, asking again.

## Putting a copy back by hand

Vigie does not put a copy back. On the account of the site, from the copy's folder sent there, each
archive goes back into its part's folder, and the dumps into the database, which they replace:

```sh
cd ~/www/my-site                          # the site's folder
gzip -dc database.sql.gz | wp db import -  # then database-2.sql.gz and the others, if any
tar -xzf plugins.tar.gz -C wp-content/plugins
tar -xzf themes.tar.gz -C wp-content/themes
tar -xzf uploads.tar.gz -C wp-content/uploads
tar -xzf others.tar.gz -C wp-content
```

`mu-plugins` archives go into `wp-content/mu-plugins`, `wordpress` archives into the site's folder.
For a copy run elsewhere (a local site), import the dump into its database, then change the address
with `wp search-replace`.

## From the command line

```sh
vigie copy "Clients/Shared hosting/my-site.example" --content database
vigie copy --tag clients --json
```

`--content` is `database`, `content` (the default) or `site`. The command asks nothing first, since
it only reads the servers; its exit status is 3 when a site could not be copied.

## What a copy reads on the server

A few read-only commands give the sizes and the folders: a short script run by `wp eval-file` that
prints where WordPress keeps its parts, `wp db size`, `du` and `find`. Then the archives: `wp db
export` to its standard output (`--single-transaction`, so InnoDB tables are read without locking
the site), and `tar` of the files listed, both read over the SSH connection. Nothing is written in
the account, and each copy counts in the two commands at a time Vigie runs per host.
