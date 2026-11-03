# Security policy

Vigie connects to your servers and updates your sites: a flaw in it matters. Thank you for
reporting one privately.

## Reporting a vulnerability

Please do **not** open a public issue. Instead:

- use GitHub's private reporting: the **Security** tab of this repository, **Report a
  vulnerability**;
- or write to <dev@misits.ch>.

Say what you found, how to reproduce it, and what it lets someone do. Leave out real host names,
keys and passwords; a made-up example is enough. You will get an answer as soon as possible, and
credit in the release notes if you wish, once a fix is published.

## Supported versions

Only the latest release gets security fixes. Vigie updates itself from **Settings**, **About**; a
Homebrew install with `brew upgrade --cask vigie`.

## In scope

- the app and its `vigie` command: SSH, what is run on servers, updates and rollbacks, backups;
- what Vigie sends from your computer ([docs/security.md](docs/security.md));
- Vigie's own updates: a release is trusted only if it is signed with the key built into the app
  (`internal/buildinfo/update.go`), its files listed with their SHA-256 in `SHA256SUMS`;
- the WP-CLI that Vigie installs on an account, checked against the digest built into it.

A vulnerability in WordPress, a plugin or a theme is for their authors, or for
[WPVulnerability](https://www.wpvulnerability.com/), whose data Vigie shows.
