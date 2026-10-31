# Read a site's status, and set a finding aside

Each scan gives a site a status, shown in the site list and on its page. It is the worst of what
the scan found.

## The statuses

- **Urgent**: act now. A known vulnerability that is serious or exploited, WordPress files
  changed, a PHP file in the uploads folder, a certificate or a domain about to expire.
- **Needs attention**: a pending update, an administrator without two-factor, PHP no longer
  maintained, and other points to fix when you can.
- **Not fully checked**: a check could not run. The Security tab says which one, and why.
- **Up to date**: nothing to report.

## Read the findings

The **Security** tab of a site lists what the scan found, by level, with the facts and the links
to the sources. **Check files now** compares the files with wordpress.org again, which Vigie does
otherwise once a day.

## Set a finding aside

A finding you know about can be ignored: **Ignore…**, then either **until the next version** of
its plugin, theme, WordPress or PHP, or **permanently**, with a reason. It stays listed, dimmed,
and no longer counts in the status. **Stop ignoring** puts it back.

Learn more: [how findings are ranked](../security.md).
