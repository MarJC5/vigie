# Vigie

Vigie is a desktop app to monitor and update WordPress sites from your own computer, over SSH and
WP-CLI. It installs nothing on your sites.

> **Status: early development.** Vigie is not usable yet. This page describes what it is being
> built to do.

## Why

Managing many WordPress sites usually means a dashboard plugin installed on every site, or an
online service holding access to all of them. Both add an entry point to the sites they are meant
to protect. Vigie takes another route:

- **Agentless**: it connects with SSH and runs WP-CLI on the server. Nothing is added to your sites.
- **Local**: a native app on your machine. No account, no hosted service, no open network port.
- **Read-only by default**: every action that changes a server shows what it will do first, asks
  for confirmation, and is logged.
- **No stored secrets**: your SSH keys stay in your SSH agent (1Password, Bitwarden, ssh-agent).
  Vigie never stores a password.

## Planned features

- A site manager with nested folders, hosts (one SSH account each) and sites, with colors and
  notes; discovery of every WordPress install on a host; hosts from `~/.ssh/config`, and from
  FileZilla's site manager with the add-on [`vigie-filezilla`](https://github.com/MarJC5/vigie-filezilla).
- A scan of each site: WordPress, PHP and database versions, pending updates (core, plugins,
  themes, translations), known vulnerabilities, abandoned plugins, file integrity, administrators
  and two-factor status, TLS certificate and domain expiry.
- Multisite networks handled as one site, with their subsites.
- Safe updates, on one site or in bulk: backup first, update, check the site, and roll back on
  failure, including premium plugins.
- English and French interface.
- A `vigie` command line with JSON output, for scripts and scheduled scans.

## Privacy

Vigie keeps your data on your computer: no account, no hosted service, no telemetry. A security
scan asks a few outside services what the server cannot tell (known vulnerabilities, closed
plugins, domain expiry), from your computer and only after you agreed: each one is listed with
exactly what it receives, and each one can be turned off in the settings. Your sites' addresses
are never sent to a third party. See [docs/security.md](docs/security.md).

## Requirements

- macOS or Linux.
- For each site: SSH access with a key, PHP on the command line and WP-CLI on the server.

## License

[MIT](LICENSE).

Vigie is not affiliated with or endorsed by the WordPress Foundation. WordPress is a trademark of
the WordPress Foundation.
