# Changelog

All notable changes to Vigie are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Assistants**: an assistant (Claude or any other MCP client) reads what Vigie knows of your
  sites, scans them and, if you allow it, updates, copies and ignores, through `vigie mcp`, which
  the client starts on this computer: nothing listens on the network. Off by default; Settings,
  Assistants sets the level (off, read, actions) and gives the lines for the client. Every action
  waits for your yes, asked by Vigie through a client that can ask you, or confirmed by a second
  call otherwise. The content of `debug.log`, your notes, keys and the administrators' e-mail
  addresses are never sent. What an assistant started shows in the journal and the history, "by an
  assistant", with the client's name.

### Fixed

- A PHP file in the uploads whose message runs code before PHP stops (`die("{${...}}")`, or a
  403 header written that way) is no longer taken for a file dropped on purpose: the scan reports
  it. An `index.php` that is empty, holds no PHP (as Matomo writes) or answers 403 through the
  server's protocol no longer raises an urgent finding.

## [0.2.0] - 2026-11-05

### Added

- **Copies** of a site on this computer, from its new Copies tab, its menu or a selection: the
  database, the database and `wp-content`, or the whole site, read only on the server. A large
  part is split into archives (500 MB by default), a copy cut off resumes with only what is
  missing, and a copy removed goes to the Trash. Settings, Data sets the folder of the copies and
  the size of the archives. Also `vigie copy` on the command line.
- **Rollback on its own**: a plugin or theme whose check fails after its update goes back to its
  version before, and the update goes on with the next items. The version is set aside on the site:
  Update all and `vigie update` leave it out until a newer one is offered. The update window, the
  summary of an Update all, the end notification, the History tab and the journal say it.
  WordPress itself still waits for your choice. On by default, in Settings, General.

### Changed

- Buttons and menus no longer end with an ellipsis ("Export", not "Export…"); what is running
  keeps it ("Scanning…").
- One look for every field, the same in every engine: checkboxes and radios are drawn by Vigie (a
  white check on the accent), Ignore and Export choose with cards, the reason of an ignore and the
  notes of a site share one text area, and the fields in a row (rename in the tree, name a site
  found, add a tag) share one. Every button that reads as a link, every filter and every banner
  has one look, and icons come in six sizes.

### Fixed

- The Homebrew cask states the macOS it needs (Monterey or later) in the form Homebrew asks for.

## [0.1.1] - 2026-11-03

### Changed

- Automatic scans run at fixed times of the day, every interval from 06:05 (every 6 hours: 00:05,
  06:05, 12:05 and 18:05), instead of an interval after the latest complete scan. A scan of every
  site made shortly before a time stands for it, and turning automatic scans on no longer catches
  up a time that came before.

### Fixed

- The summary of "Update all" on a host or folder is its own: it no longer shows the sites of an
  earlier update of another host while the new one runs, nor is it replaced by another run that
  ends after it.
- Settings, About says "installed with Homebrew" only for the app Homebrew installed, not for
  another copy of Vigie on a Mac that also has the cask.

## [0.1.0] - 2026-11-03

The first release.

### Added

- **Site manager**: folders, hosts (one SSH account each) and sites in a tree that works from the
  keyboard, with colors, notes in Markdown, tags across folders, quick links (the site, its
  dashboard, an SSH terminal) and a search. Several items selected act together: scan, update,
  move, color, tag or delete them in one go.
- **Adding hosts**: a wizard that shows the server's key to check, tests SSH, PHP and WP-CLI, then
  finds every WordPress install of the account; hosts from `~/.ssh/config`, sites from a CSV list,
  and add-ons that bring hosts from other tools, such as
  [`vigie-filezilla`](https://github.com/MarJC5/vigie-filezilla) ([docs/addons.md](docs/addons.md)).
- **WP-CLI installed on an account that has none**, from the connection test and after you
  confirm: downloaded on your computer, checked against the digest built into Vigie, sent to
  `~/bin/wp`.
- **Scans**, read-only: the versions of WordPress, PHP (command line and web), WP-CLI and the
  database; pending updates of WordPress, plugins, themes and translations; the admin email and
  `debug.log`. A site's plugins and themes, and those of every site of a host or folder.
- **Security**: known vulnerabilities, plugins closed or abandoned on wordpress.org, files that
  differ from wordpress.org's, PHP files in the uploads, administrators and their two-factor, risky
  settings, overdue cron events, heavy autoloaded options, the TLS certificate, the domain's expiry
  and a public `debug.log`. Findings ranked, and ignored until the next version or for good with a
  reason ([docs/security.md](docs/security.md)).
- **What changed since the previous scan**: an administrator added or removed, a plugin or theme
  installed or removed outside Vigie, a version changed, files newly different, a new finding.
- **Multisite networks** as one site, unfolding to their subsites, each with its plugins, theme and
  administrators.
- **Updates** of WordPress, plugins, themes and translations, on one site or many: a preview, a
  database dump and an archive of each item on the server first, the site checked after each item,
  a stop at the first failure and a rollback. Backups kept 30 days and the last 3 updates by
  default; an update ignored, a link to each changelog, the history of each site
  ([docs/updates.md](docs/updates.md)).
- **Activity**: what happened, by day, with what each scan changed and what Vigie notified.
- **In the background**, each part turned on in the settings: Vigie stays in the menu bar, opens
  at login, scans every 1 to 24 hours, and notifies when a site turns urgent, a host turns
  unreachable, an administrator appears or a long run you started ends
  ([docs/background.md](docs/background.md)).
- **Lookups outside**, made from your computer only after you agreed, each listed with what it
  receives and each with its switch; your sites' addresses are never sent to a third party.
- **Data**: export and import (a preview first, keys never in the file), a documented JSON format,
  the history kept as long as the pages need it, and a compaction of the database
  ([docs/data.md](docs/data.md)).
- **Command line**: `vigie tree`, `scan`, `discover`, `update`, `export`, `import` and the add-ons,
  with `--json`, `--tag` and exit codes for scripts, and their progress in a terminal. `vigie` alone
  opens the window; Homebrew links the command, the settings give the one that links it otherwise.
- **Vigie updates itself** when you ask, from releases signed with a key built into the app
  ([docs/releasing.md](docs/releasing.md)).
- **Help** in the window, offline, with the docs in English and French.
- English and French, dark and light, following the system; a menu bar with shortcuts.
- The macOS app (universal, macOS 12 or later), the AppImage, deb, rpm and Arch packages for
  x86_64 and aarch64, and a Homebrew cask.

### Security

- The window loads only what Vigie embeds: a strict Content Security Policy refuses any script,
  style, font, image or connection from elsewhere.

[Unreleased]: https://github.com/MarJC5/vigie/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/MarJC5/vigie/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/MarJC5/vigie/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/MarJC5/vigie/releases/tag/v0.1.0
