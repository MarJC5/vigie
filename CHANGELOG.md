# Changelog

All notable changes to Vigie are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Project license (MIT) and description.
- Desktop window for macOS and Linux, with the site manager: folders, hosts and sites in a tree
  that folds and works from the keyboard, and the stored details of the selected item.
- Interface in English and French, following the system language.
- Dark and light themes, following the system appearance, or the one chosen in the settings.
- Add a host with a wizard: an SSH host name and user, or an alias from `~/.ssh/config`; the
  server's key is shown to be checked, then kept in Vigie's own `known_hosts` (your SSH files are
  not changed); a connection test of SSH, PHP and WP-CLI; then the WordPress installs of the account
  are found, and you choose the ones to add.
- Scans of a site, a host, a folder or everything, in the background and read-only: the versions
  of WordPress, PHP and WP-CLI, and the pending updates of WordPress, plugins, themes and
  translations. An update check that could not reach wordpress.org is reported as such, never
  shown as up to date. A theme kept in a plugin's folder is read with the plugins loaded, for that
  read only.
- The latest scan in the window: a status in the site list, the overview and updates of a site,
  the table of the sites of a host or folder, the scans running with a button to stop them.
- Search in the site list.
- The number of pending updates of each host, next to it in the site list.
- Menu bar in English and French, with shortcuts to add a host, search and scan.
- Command line: `vigie tree`, `vigie scan [path]` and `vigie discover <host>`, with `--json`
  and exit codes for scripts (0 all good, 1 needs attention or not fully checked, 2 urgent, 3 error).
- Edit the site manager: create folders, rename in place, move by drag and drop or from a dialog,
  give a color to a folder, host or site, and delete a site, a host with its sites, or an empty
  folder (servers are not touched). Edit a host's connection, and find the new WordPress installs
  of a host.
- Updates of WordPress, plugins, themes and translations, on a site with the items you choose, or
  on every site of a host or folder. A preview shows what changes and what is backed up first;
  the database is dumped and the items archived on the server, then each item is updated and the
  site checked (pages, critical error, new fatal errors in `debug.log`); the update stops at the
  first failure. The item that failed can be rolled back (with the database after a WordPress
  update). The last 3 backups of each site are kept, in `~/.vigie/backups` of the account: see
  [docs/updates.md](docs/updates.md).
- The history of the updates and rollbacks of each site, with their steps.
- Command line: `vigie update [path]`, which prints the preview and asks before writing anything
  (`--yes` for scripts, `--json`).
- Security scans, read-only, with every scan: known vulnerabilities of WordPress, plugins, themes
  and PHP; plugins closed or abandoned on wordpress.org; WordPress and plugin files compared with
  wordpress.org's (at most once a day, or on demand); PHP files in the uploads folder;
  administrators and their two-factor; risky settings; overdue WP-Cron events; heavy autoloaded
  options; the database version; the TLS certificate, the domain's expiry and a public
  `debug.log`. See [docs/security.md](docs/security.md).
- Lookups outside, made from your computer and only after you agreed: WPVulnerability,
  wordpress.org, the domain registries and the site itself, each one listed with what it receives
  and each one can be turned off. Nothing about your sites is sent to a third party.
- Findings ranked urgent, needs attention, information or not checked, and a site status from the
  worst one: urgent, needs attention, not fully checked or up to date. A finding can be ignored
  until the next version, or permanently with a reason.
- The Security tab of a site, the worst finding of each site in the tables, and the settings in
  tabs (language, lookups outside, about), opened from the rail or with ⌘,.
- Command line: `vigie scan` exits with 2 when a site is urgent, lists the findings with `--json`,
  and compares the files again with `--check-files`.
- Export the site manager, everything or a folder or host, with or without the history, to a JSON
  file readable by you only; its format is documented with a JSON Schema
  ([docs/data.md](docs/data.md)). Keys and passwords are never in it.
- Import an export, from this computer or another one: a preview shows what is added, what is
  already there and left as it is, and what is refused with why; nothing existing is changed, and
  the server keys the file brings are kept only once you check them.
- Import hosts from `~/.ssh/config`, the aliases you choose (none is chosen at first, since the file
  names every server), and sites from a CSV list, with the lines refused and their numbers.
- The Data tab of the settings, and Export… in the menu of a folder or host.
- Command line: `vigie export [path] [--history] [-o file]` and `vigie import <file>` or
  `vigie import --ssh-config [alias…]`, which preview, then ask, with `--yes`, `--trust-keys` and
  `--json` for scripts.
- Add-ons: programs named `vigie-<name>`, installed by you, that Vigie finds on your `PATH` and in
  the usual install folders. The Add-ons page lists the installed ones and the Library of known
  ones, each with a page of its own; a source runs from its page, at most two minutes, and what it
  wrote goes through the import preview. `vigie <name>` runs one from the command line. Vigie
  never downloads or installs one ([docs/addons.md](docs/addons.md)).
- The Go module `github.com/MarJC5/vigie/addon`, shared with add-ons: the export format, the
  description of an add-on, and the checks an add-on runs in its own tests.
- In the background, each part off until turned on in the settings: Vigie keeps running in the
  menu bar once its window is closed, opens at login, scans every site every 1, 3, 6, 12 or 24
  hours (one scan after a sleep, never an update), and notifies when a scan turns a site urgent or
  a host unreachable ([docs/background.md](docs/background.md)).
- Vigie's own icon: an eye whose pupil is a shield.
- The Activity page: what happened, by day, for the last 30 days and older on demand: each scan
  with who started it (you, the automatic scans or the command line), what it changed (a site
  turned urgent or back to normal, a host unreachable) and what Vigie notified; updates and
  rollbacks; filtered by folder, host or site, a name opening its page.
- The Help page, from the rail or Help > Vigie Help: getting started (the first host, what to do
  when a host refuses the connection, the shortcuts), then short topics (updating, a site's status,
  what Vigie reads and sends, import and export, the background), offline, each ending with a link
  to its full doc. The docs are also in French ([docs/fr/](docs/fr/)).
- Quick links under a site's name: open the site, its dashboard (under the WordPress address, or
  the admin address you give for a login a plugin renamed), an SSH terminal (macOS, and Linux when
  the system opens ssh:// addresses), and the SSH command to paste, which opens in the site's folder.
- Notes in Markdown on every folder, host and site: written in place, shown without raw HTML, and
  read by the search of the site list.
- Test the connection of a host already added, from its menu or while editing it.
- The tables of sites sort by site, status or updates, and filter from the counts above them.
- A site's plugins and themes, from its latest complete scan: version, status, update offered,
  automatic updates, the worst finding, and a link to wordpress.org; filtered by active, inactive or
  with an update.
- The plugins and themes across the sites of a host or folder, in a tab next to its sites: how many
  sites run each, in which versions, with an update; a line opens to its sites, and a search by name.
  A site never scanned, or whose latest scan failed, is left out and named.
- The site list stays as you left it, from one page to another and from one launch to the next:
  the folders and hosts folded, and the list hidden or shown.
- Tags on sites, written under a site's quick links, the tags already used offered as you type;
  compared without case or accents. A tag's page, from the Tags group of the site list or a tag
  of a site, lists its sites wherever they are, with the counts, the table, the Plugins tab, and
  scan and update for those sites; its menu renames the tag everywhere (onto another tag, both
  become one) or removes it from every site. The search of the site list reads the tags.
- `--tag <tag>` on `vigie tree`, `scan` and `update`, for the sites with a tag. A `tags` column in
  a CSV site list; tags read from a file follow the rules of a tag, and those left out are named
  in the preview.
- An update can be ignored from a site's Updates tab, until a newer version is offered or
  permanently with a reason: it counts nowhere then, Update all and `vigie update` leave it out and
  say so, and the site's own update installs it only when ticked. The Plugins tab says so too.
- Each update offered links to its changelog when wordpress.org has one: a plugin's Development
  tab, the release notes of WordPress.
- What changed on a site since its previous complete scan: an administrator added or removed, a
  plugin or theme installed or removed, the active theme switched, a version changed by Vigie or
  outside it, files of WordPress or a plugin newly different, a new finding. Under each scan in the
  Activity journal, above a site's Overview, under each site in `vigie scan` and as `changes` in
  `--json`. A new administrator, and a plugin or theme installed outside Vigie, notify.
- A scan reads the PHP serving the pages, from the check WordPress's dashboard keeps a week (known
  when someone opened the dashboard that week, and said so otherwise); the PHP checks are then
  about it. It also reads the admin email and the one waiting for its confirmation, and
  `debug.log`: its size, whether WordPress still writes it, and its fatal errors of the last week,
  as one finding with the latest's message. On a site's Overview and in `vigie scan --json`.
- A multisite network in detail: it unfolds in the site list to its subsites, each with its page
  (its address, state, theme, plugins, admin email and administrators); its Plugins tab says where
  each plugin and theme is active (across the network, on some subsites, nowhere). A scan reads
  every subsite in one go, with a read-only script sent to WP-CLI, and every administrator of a
  subsite is a finding as those of the main site are, the super administrators first.
- Settings, Data, Database: the history kept, and "Compact the database", which shows the size of
  the database and the space free in it, and gives that space back to the disk.

### Changed

- The backups Vigie makes on a server before an update are kept 30 days by default (7, 30 or 90
  days, or no limit, in Settings, Data), and at most those of a site's last 3 updates: after each
  update, the limits apply to every site of the host. Each scan reads them; a site's History tab
  lists them and removes those past the limits; deleting a site or host from Vigie offers to remove
  its backups too, and gives the command to run when the server cannot be reached.
- The history of scans is kept for as long as the pages need it, so the database stops growing:
  each site's latest scans always, every scan in detail for 90 days, then for a year only the scans
  that changed something, without their details; runs and notifications a year. Vigie lets the
  rest go when it starts and after each run of scans. See [docs/data.md](docs/data.md).

- The site list, a site's page and the Activity journal stay fast with a long history: the latest
  scans and the journal's days are found by index, not by going through every scan kept.
- A scan no longer asks wordpress.org once per plugin from the server: the translation updates
  and the plugins wordpress.org knows come from the update check WordPress makes anyway, as its
  Updates screen shows them. A site with 22 plugins is scanned in about 6 s instead of 33 (the
  daily comparison of the files aside).
- A run scans up to 16 sites at a time instead of 8, each host keeping its own limit of commands
  at a time, and opens at most 8 SSH connections at once, whatever the servers: an SSH server
  turns away connections opened too many at once, and several hosts can share one server.
- The window stays quick while many sites are scanned: the eye in the menu bar reads the sites'
  state once scans are stored, not as each site ends, and not at all when it is not shown.
- The Activity journal reads what each scan noted of itself when it was made (its status and its
  changes), instead of computing it again from every detail: it opens in a tenth of a second on
  hundreds of sites. A finding ignored later no longer rewrites the journal's past lines; the
  scans stored before are noted once, in the background, at the first start.
- `WP_DEBUG_LOG` is read as WordPress reads it: "true" and "1", in any case, name
  `wp-content/debug.log`, and `WP_DEBUG` counts whenever it is true for PHP.
- A scan cancelled or failed after a complete one no longer hides it: the complete one stays shown,
  and the window says why nothing newer is.
- Whether wordpress.org knows a plugin is asked from your computer when you agreed to it: a server
  without outbound access no longer shows every plugin as closed.

### Security

- The window loads only what Vigie embeds: a strict Content Security Policy refuses any script,
  style, font, image or connection from elsewhere, and the tests open every section to check it.
