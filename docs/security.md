# Security scans: what Vigie reads, asks and sends

Every scan of a site is also a security scan. It reads the site over SSH with WP-CLI, like the
rest of the scan, and only reads: nothing is written on the server. A few things cannot be read on
the server, so Vigie asks outside services for them, from your computer, and only after you
agreed. This page lists both, and how the results are ranked.

## What is read on the server

Read-only WP-CLI commands, with the plugins and themes not loaded, and two shell commands (the
site's real folder, and a `find` in its uploads):

- **Files compared with wordpress.org**: `wp core verify-checksums` and
  `wp plugin verify-checksums --all`. The server downloads the published checksums and reads every
  file, so this runs **at most once a day** per site; in between, the last comparison is carried
  over. It runs again at once when WordPress or a plugin changed version, when the last comparison
  failed, or when you ask for it (**Check files now**, or `vigie scan --check-files`).
- **PHP files in the uploads folder**, where WordPress never puts any: their names, and for an
  `index.php` the first 120 bytes, so that the empty "Silence is golden" files plugins drop to
  stop folder listings are told apart.
- **Administrators** (and super administrators of a network), and which two-factor methods each
  turned on: Two Factor, Solid Security and WP 2FA in the user's meta, Wordfence in its own table
  when Wordfence is active (that read needs the `mysql` client on the server). **Never** a
  two-factor secret, a backup code or a password.
- **Settings**: the names of the constants of `wp-config.php`, then the values of `WP_DEBUG`,
  `WP_DEBUG_DISPLAY` and `DISALLOW_FILE_EDIT` only. **Never** the database password, the keys or
  the salts.
- **Options**: whether search engines are asked not to index the site, and the size of the options
  loaded on every page (names and sizes, never their values).
- **WP-Cron**: the scheduled events and when each is due (not `wp cron test`, which would run them).
- **Database server version**, when the `mysql` client is on the server.

## What is asked from your computer

Nothing below is sent before you agreed, on the screen shown before the first scan that needs it.
Each service can be turned off in **Settings, Lookups outside**; its checks are then "not
checked", never "up to date".

| Service | What it receives | When | Answer kept |
| --- | --- | --- | --- |
| [WPVulnerability](https://www.wpvulnerability.com/) | the slugs of the plugins and themes, the WordPress version, the PHP branch | at each scan | a day per slug or version |
| api.wordpress.org | the slugs of the plugins | at each scan | a day per slug |
| data.iana.org | nothing about your sites (the list of domain registries) | when the kept list expires | until it expires |
| the registry of the domain's TLD (RDAP) | the registered domain name; nothing for .ch, .li and .de, whose registries publish no expiry date | at each scan | a week per domain |
| the site itself | its host name (a TLS connection on port 443, for the certificate) and one request for the first byte of `/wp-content/debug.log` | at each scan | nothing |

**Never sent**: your sites' addresses to a third party, the versions of your plugins and themes
(they are compared on your computer), your SSH details, anything read from your databases.

Every request says what it is: `User-Agent: Vigie/<version> (+https://github.com/MarJC5/vigie)`.
When a service cannot be reached, the last answer kept is used and its date is shown; without one,
the check is "not checked".

Vulnerability data come from WPVulnerability, in the public domain (CC0), without warranty: no
known vulnerability is not the same as safe. Vigie links to each record's sources.

## How findings are ranked

**Urgent**:

- a vulnerability of the installed version scored high or critical (CVSS 3), known to be exploited
  (CISA KEV), or fixed in no version;
- a WordPress file changed, missing or added (except `readme.html`, `license.txt` and
  `wp-config-sample.php` missing: they are often deleted on purpose, and are only shown);
- a PHP file in the uploads folder (other than an empty "Silence is golden" `index.php`);
- a TLS certificate expired, for another name, from an unknown authority, or expiring within 7 days;
- a domain expiring within 7 days;
- a `debug.log` anyone can download;
- a plugin closed on wordpress.org for a security issue.

**Needs attention**: a pending update; any other vulnerability, including one without a score; a
plugin closed for another reason, or without a release for two years; files of a plugin that differ
from wordpress.org's (one finding per plugin; when nearly every file differs, it is probably
another plugin of the same name); PHP with security fixes only, or no longer maintained; an
administrator without two-factor; file editing allowed from the dashboard; PHP errors shown on the
pages; the site hidden from search engines; a site that does not answer in HTTPS; WP-Cron events
overdue by more than an hour; options loaded on every page above 800,000 bytes (WordPress's own
limit); a certificate or a domain expiring within 30 days.

**Information**: what was read and is fine or good to know: the administrators with two-factor, the
database version, the certificate and the domain expiry, the plugins wordpress.org has no checksums
for.

**Not checked**: a check that could not run, with its reason.

A site's status is its worst finding not ignored: **urgent**, else **needs attention**, else **not
fully checked** when a check did not run, else **up to date**. A scan cancelled or failed after a
complete one does not replace it: the complete one stays shown, and the window says why nothing
newer is.

## Ignoring a finding

From the Security tab of a site, **Ignore** sets a finding aside, either **until the next version**
of its plugin, theme, WordPress or PHP (it ends by itself as soon as a scan reads another version),
or **permanently**, with a reason. An ignored finding stays listed, dimmed, with its reason and date,
and no longer counts in the status. **Stop ignoring** puts it back.

## Command line

`vigie scan [path]` exits with 0 when every site is up to date, 1 when one needs attention or is
not fully checked, 2 when one is urgent, and 3 when a scan could not complete. `--json` lists the
findings of each site with their level, facts and whether they are ignored; `--check-files`
compares the files again even within the day.
