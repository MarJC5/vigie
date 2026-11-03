# Security scans: what Vigie reads, asks and sends

Every scan of a site is also a security scan. It reads the site over SSH with WP-CLI, like the
rest of the scan, and only reads: nothing is written on the server. A few things cannot be read on
the server, so Vigie asks outside services for them, from your computer, and only after you
agreed. This page lists both, and how the results are ranked.

## What is read on the server

Read-only WP-CLI commands, with the plugins and themes not loaded, and three shell commands (the
site's real folder, a `find` in its uploads, and the end of `debug.log`):

- **Files compared with wordpress.org**: `wp core verify-checksums` and
  `wp plugin verify-checksums --all`. The server downloads the published checksums and reads every
  file, so this runs **at most once a day** per site; in between, the last comparison is carried
  over. It runs again at once when WordPress or a plugin changed version, when the last comparison
  failed, or when you ask for it (**Check files now**, or `vigie scan --check-files`).
- **PHP files in the uploads folder**, where WordPress never puts any: their names and sizes, the
  first 120 bytes of an `index.php` and the first line of another file, so that the files plugins
  drop on purpose and that run nothing are told apart: the empty "Silence is golden" `index.php`
  that stops folder listings, one that only answers 403 or 404, and a file whose first line stops
  PHP (Matomo keeps its configuration that way). Nothing more of a file is read, and nothing of it
  is kept.
- **Administrators** (and super administrators of a network), and which two-factor methods each
  turned on: Two Factor, Solid Security and WP 2FA in the user's meta, Wordfence in its own table
  when Wordfence is active (that read needs the `mysql` client on the server). **Never** a
  two-factor secret, a backup code or a password.
- **A network's subsites** (multisite), in one go: a read-only PHP script embedded in Vigie,
  `internal/wpcli/subsites.php`, sent to `wp eval-file -` on standard input, so that WordPress
  loads once for every subsite. It only calls WordPress's reading functions (a test checks the list)
  and prints, for each subsite not archived, spam or deleted, the plugins active on it, its theme,
  its admin email and its administrators with their two-factor methods; for the network, the
  plugins active network-wide and the super administrators. Each administrator, of the main site or
  of a subsite, is then one finding, with the subsites they administer, the super administrators
  first.
- **Settings**: the names of the constants of `wp-config.php`, then the values of `WP_DEBUG`,
  `WP_DEBUG_DISPLAY`, `WP_DEBUG_LOG` and `DISALLOW_FILE_EDIT` only. **Never** the database
  password, the keys or the salts.
- **Options**: whether search engines are asked not to index the site, the size of the options
  loaded on every page (names and sizes, never their values), and the admin email with the one
  waiting for its confirmation (`admin_email`, `new_admin_email`), where WordPress sends its
  notices. It stays on your computer, as the administrators' addresses do.
- **PHP serving the pages**, which a host can set apart from the command line's: WordPress's
  dashboard asks wordpress.org about its PHP version and keeps the answer a week, under a key made
  from that version (`wp_check_php_version()`). Vigie lists those keys and their expiry
  (`wp transient list --network --search=php_check_*`), never the answers, and finds on your
  computer which version each was made from. It is known only when someone opened the dashboard
  in the last week; otherwise, or when the notes are in an external object cache WP-CLI cannot
  list, the window says it is not known, and why. When known, the PHP checks below are about it.
- **`debug.log`**, where WordPress writes it (`WP_DEBUG_LOG`), or one left in `wp-content` from
  before: its size, and the fatal and parse errors of its last megabyte, at most 50 lines. Those of
  the last 7 days give one finding with their count, the date of the latest and its message, cut at
  300 characters; it can hold a path of the server, and stays on your computer.
- **WP-Cron**: the scheduled events and when each is due (not `wp cron test`, which would run them).
- **Database server version**, when the `mysql` client is on the server.
- **Vigie's own backups**, made before each update: the date and size of each update's, in
  `~/.vigie/backups/<site ID>` (`du`), for the site's History tab (see [updates.md](updates.md)).

## How long a scan takes, and how hard it works a server

A scan runs about 24 commands on the server, one after the other. Most take a tenth of a second. The
time goes to what the server asks wordpress.org: the update checks of WordPress, the plugins and the
themes, a few seconds each, and once a day the comparison of the files, which downloads the
checksums of every plugin. Nothing else is asked per plugin: the translation updates and the plugins
wordpress.org knows come from the update checks, as WordPress's own Updates screen shows them. On
Vigie's test bench, a site with 22 plugins is scanned in about 6 seconds, 25 seconds on the day its
files are compared.

- **Sites at a time**: a run scans up to 16 sites at a time, across hosts.
- **Commands at a time on a host**: 2 by default, set in the host's connection (**Commands at a
  time**). The sites of a host share them, so 20 sites on one shared hosting account take a minute
  or two: keep 2 there, as hosts expect. On a server of your own you can raise it, up to 10, the
  commands an SSH server lets run on one connection by default (`MaxSessions`). Two hosts in Vigie
  with the same server, user and port share one connection, and their commands add up.
- **Connections**: one shared connection per host, kept a minute after its last command
  (`ControlPersist`). At most 8 open at once, whatever the servers: an SSH server turns away
  connections opened too many at once (`MaxStartups`, 10 by default), and several hosts can be on
  one server.

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
| github.com | the version of Vigie, the system and the architecture, to learn of a new version of Vigie (not for a Homebrew or Linux package install) | at most once a day, and when you ask in **Settings, About** | nothing |

**Never sent**: your sites' addresses to a third party, the versions of your plugins and themes
(they are compared on your computer), your SSH details, anything read from your databases.

Every request says what it is: `User-Agent: Vigie/<version> (+https://github.com/MarJC5/vigie)`.
When a service cannot be reached, the last answer kept is used and its date is shown; without one,
the check is "not checked".

Vulnerability data come from WPVulnerability, in the public domain (CC0), without warranty: no
known vulnerability is not the same as safe. Vigie links to each record's sources.

## WP-CLI installed on an account

When the connection test of a host finds PHP but no WP-CLI, Vigie offers to install it, in the
wizard and in **Test the connection**. Nothing happens before you confirm, and the dialog says
what will be written. The offer is not made when PHP is older than WP-CLI needs (7.2.24), nor
over a `~/bin/wp` that is already there.

- **What is written**: one file, `~/bin/wp`, in the home folder of the account; nothing in your
  sites. It is written under a temporary name and renamed, so a failure leaves nothing behind.
- **Where it comes from**: the WP-CLI release on github.com, downloaded **on your computer**, not
  by the server. This download is your own action, not one of the lookups above: it sends nothing
  about your sites, and says what it is (`User-Agent: Vigie/<version>`).
- **How it is checked**: Vigie installs one version of WP-CLI, pinned with its SHA-512 digest; a
  file that does not match is discarded before anything is sent. The digest is checked against
  WP-CLI's GPG signature each time the version is changed.
- **Then**: the file is sent over SSH and run once. The host's WP-CLI command becomes `~/bin/wp`
  (or `php ~/bin/wp` when the home does not allow running programs). If neither answers, the file
  is removed.
- **Kept in Activity**: each install, done or stopped at a step, under its host.

To remove it: `rm ~/bin/wp` on the account. WP-CLI updates itself with `wp cli update`; Vigie
does not update it.

## How findings are ranked

**Urgent**:

- a vulnerability of the installed version scored high or critical (CVSS 3), known to be exploited
  (CISA KEV), or fixed in no version;
- WordPress files changed, missing or added (one finding for all of them; `readme.html`,
  `license.txt` and `wp-config-sample.php` missing are only shown: they are often deleted on
  purpose);
- a PHP file in the uploads folder, other than one dropped on purpose that runs nothing;
- a TLS certificate expired, for another name, from an unknown authority, or expiring within 7 days;
- a domain expiring within 7 days;
- a `debug.log` anyone can download;
- a plugin closed on wordpress.org for a security issue (only shown when its files say the
  installed plugin is another one of the same name).

**Needs attention**: a pending update; any other vulnerability, including one without a score; a
plugin closed for another reason, or without a release for two years; files of a plugin that differ
from wordpress.org's (one finding per plugin; when nearly every file differs, it is probably
another plugin of the same name; when none of them matches wordpress.org's list, they could not be
compared, which is only shown); PHP with security fixes only, or no longer maintained (the PHP
serving the pages when it is known, the command line's otherwise); an
administrator without two-factor; file editing allowed from the dashboard; PHP errors shown on the
pages; fatal errors in `debug.log` in the last 7 days; the site hidden from search engines; a site that does not answer in HTTPS; WP-Cron events
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

## What changed since the previous scan

Each complete scan is compared with the complete scan of the site before it (one that failed or
was cancelled in between is skipped), from what both stored: nothing more is read on the server.
A change is:

- an administrator added (a super administrator said so) or removed,
- a plugin or theme installed or removed, or the active theme switched,
- a version of WordPress, a plugin or a theme changed, **by Vigie** when one of its updates of the
  site made it between the two scans, **outside Vigie** otherwise (automatic updates, by hand,
  another tool),
- files of WordPress or of a plugin newly different from wordpress.org's,
- a new finding that is urgent or needs attention, and is not ignored.

They are listed under each scan in the Activity journal, above a site's Overview ("Since the
previous scan", absent when nothing changed), and under each site in `vigie scan`. A new
administrator, and a plugin or theme installed outside Vigie, notify (see
[background.md](background.md)): Vigie never installs one. A change made and undone between two
scans is not seen.

## Command line

`vigie scan [path]` exits with 0 when every site is up to date, 1 when one needs attention or is
not fully checked, 2 when one is urgent, and 3 when a scan could not complete. `--json` lists the
findings of each site with their level, facts and whether they are ignored, and its `changes`
since the previous scan; `--check-files`
compares the files again even within the day.

In a terminal, `vigie scan`, `vigie update` and `vigie discover` show their progress on the
standard error, redrawn in place and erased before the results: a line per site being scanned with
the bar of its steps (per site being updated, with its step), the last sites done, and a total with
the time spent. Never taller than the terminal, never with `--json`, nor when the output goes to a
file or another program, so scripts read the same output as before.
