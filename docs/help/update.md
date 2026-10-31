# Update a site, and roll back

Vigie updates WordPress, the plugins, the themes and the translations of a site, or of every site
of a host or folder.

## Update

1. Open the site, tab **Updates**, and check what to update.
2. Read the **preview**: what changes, and what is backed up first.
3. Start the update. Vigie backs up the database and the folders concerned, updates one item at a
   time, and **checks the site** after each.

At the first problem, the update stops: the next items are not started.

For every site of a host or folder, open it and choose **Update all**: a preview by site, then
each site is updated as above.

## Roll back

When the latest update of a site failed, **Roll back** puts the failed item back as it was. For
WordPress itself, the database goes back too: what was written to it in between (orders,
comments) is lost.

## The backups

Vigie keeps the ones of the last 3 updates of each site on the server, in `~/.vigie/backups/`.
They hold the whole database: if that folder can be reached from the web, remove them.

Learn more: [what Vigie writes on a server](../updates.md).
