# Update a site, and roll back

Vigie updates WordPress, the plugins, the themes and the translations of a site, or of every site
of a host or folder.

## Update

1. Open the site, tab **Updates**, and check what to update.
2. Read the **preview**: what changes, and what is backed up first.
3. Start the update. Vigie backs up the database and the folders concerned, updates one item at a
   time, and **checks the site** after each.

A plugin or theme that breaks the site goes **back on its own**, and the update goes on. Its
version is **set aside**: **Update all** leaves it out until a newer one. For WordPress, the update
stops at the first problem.

For every site of a host or folder, open it and choose **Update all**: a preview by site, then
each site is updated as above.

A version you keep on purpose: **Ignore** on its row, until the next version or permanently with
a reason. It no longer counts, and **Update all** leaves it out.

## Roll back

When the latest update of a site failed, **Roll back** puts the failed item back as it was. For
WordPress itself, the database goes back too: what was written to it in between (orders,
comments) is lost.

## The backups

Vigie keeps them on the server, in `~/.vigie/backups/`: 30 days (**Settings, Data**), and at most
those of the last 3 updates of each site. A site's **History** tab lists them. They hold the whole
database: deleting a site from Vigie removes them too.

Learn more: [what Vigie writes on a server](../updates.md).
