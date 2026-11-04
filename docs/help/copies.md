# Copies

Vigie copies a site to this computer: its database, and if you want its files. It only reads the
server. A copy is yours, and it does not replace your host's backups.

## Copy a site

In the site's **Copies** tab, **Copy**, or **Copy to this computer** in its menu. Choose the
database, the database and `wp-content` (the default), or the whole site; untick the folders of
`wp-content` you do not want, such as a cache. Several sites selected are copied in one go.

The copies go to the folder set in **Settings, Data**, `~/Documents/Vigie` by default, one folder
per site and date. A large part is split into archives of the size set there.

## Cut off, removed

A copy cut off shows **Incomplete**: **Resume** reads only what is missing. The trash icon moves a
copy to the Trash. A copy holds the site's personal data: keep it as you keep the site.

## Put it back

Vigie never puts a copy back on a server. By hand, on the site's account: each archive goes back
into its part's folder with `tar -xzf`, the database with `wp db import`.

Learn more: [what a copy holds, and how to put it back](../copies.md).
