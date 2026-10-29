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
- Dark and light themes, following the system appearance.
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
- Menu bar in English and French, with shortcuts to add a host, search and scan.
- Command line: `vigie tree`, `vigie scan [path]` and `vigie discover <host>`, with `--json`
  and exit codes for scripts (0 all good, 1 needs attention, 3 error).
