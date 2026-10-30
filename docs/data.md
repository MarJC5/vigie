# Data: what Vigie keeps, and its export format

Vigie keeps everything on your computer, in one SQLite file: `~/Library/Application
Support/Vigie/vigie.db` on macOS, `~/.config/vigie/vigie.db` on Linux (or the folder given with
`--data-dir`). Next to it, `known_hosts` holds the keys of the servers you trusted. Before each
update of its database schema, Vigie writes a copy, `vigie.db.before-<version>`.

## The export format

An export is a JSON file, UTF-8, that holds your site manager. Vigie writes it, Vigie on this
computer or another one reads it back, and add-ons can write or read it too. Its exact shape is the
JSON Schema [`export-v1.schema.json`](export-v1.schema.json).

```json
{
  "format": "vigie",
  "version": 1,
  "exported": "2026-10-30T14:12:00Z",
  "app": "0.6.0",
  "scope": "",
  "folders": [ ... ],
  "hosts": [ ... ],
  "sites": [ ... ],
  "ignores": [ ... ],
  "history": { "scans": [ ... ], "operations": [ ... ] }
}
```

* `scope` is the path of the folder or host exported (`Clients/Infomaniak`), or `""` for
  everything. A folder or host at the top of the export has `""` as its parent.
* **folders**: `id`, `parent`, `name`, `color` (`none`, `red`, `green`, `blue`, `yellow`, `cyan`,
  `magenta`, `orange`), `notes` (Markdown), `position`.
* **hosts**: `id`, `folder`, `name`, and either `alias` (an entry of `~/.ssh/config`) or
  `hostname`, `port` and `user`; `keyPath` (the path of a key on your computer, never the key),
  `wpCli`, `maxConcurrent`, `color`, `notes`, `position`, and `trustedKeys`: the public keys of the
  server you trusted, each with the `name` Vigie's `known_hosts` gives it (`host` or
  `[host]:port`), its `type` and its `key`.
* **sites**: `id`, `host`, `name`, `path` (the WordPress root on the server), `url`, `wpCli`,
  `color`, `notes`, `tags`, `position`, `multisite` and, for a network, its `subsites`.
* **ignores**: the findings you set aside: `site`, the finding's `key`, `untilVersion` (`""` when
  permanent), `reason`, `created`.
* **history**, only when you ask for it: the scans of the sites exported, with what they read
  (`items`) and found (`findings`), and the updates and rollbacks with their steps.

**Never in an export**: private keys, passwords, the settings of this computer (language, your
consent to lookups from it), the answers Vigie keeps from outside services.

**Sensitive**: an export names your servers, your SSH users and holds your notes. Vigie writes it
readable by you only (`0600`), through a temporary file renamed at the end, so an interrupted
export never leaves half a file. Keep it somewhere safe.

### Versions

`version` changes when the format does. Vigie reads every older version, and refuses a newer one
with a message, rather than dropping what it does not know: a field it does not know is refused for
the same reason.

When Vigie reads an export, it also checks that it holds together: every `id` an ID, each one once;
every `parent`, `folder`, `host` and `site` in the file; names and paths given; no folder inside
itself; colors, key types and levels among the values the schema lists.

## Importing

An import adds what is new, into the folder you choose (the top by default), and changes nothing
already there. Before it adds anything, it shows what it will do with each folder, host and site of
the file:

* **already there**, left as it is: the same ID; for a folder, one of the same name in the same
  place, which receives what the file puts in it; for a host, one with the same alias, or the same
  hostname, port and user, which receives the file's new sites; for a site, one with the same path
  on the same host;
* **added**, with the file's ID, or a new one when another node has it, and `(2)`, `(3)`... after
  its name when a neighbour has that name;
* **refused**, with the reason, and with it what it holds.

The ignores and the history of the file come with the sites added only. An update or rollback that
was still running where the file was written comes back as interrupted.

**Trusted host keys** come back only once you confirm them: the preview shows each key's
fingerprint, for the host it belongs to. A key you leave out, or one Vigie cannot take (not for that
server, or Vigie already trusts another key for it), is not kept, and the host asks at its first
connection, as a new host does.

Everything is added in one go: when something changed in between and clashes with the preview,
nothing is added.

## Importing a site list (CSV)

A list of sites, from a spreadsheet for example, is imported the same way: Vigie reads it into an
export with new IDs, shows the preview, and adds what you confirm.

* **A header line** names the columns, in any order: `path` (the WordPress root on the server), and
  either `alias` (an entry of `~/.ssh/config`) or `host` and `user` with an optional `port`; then,
  if you want, `folder`, `url`, `name`, `color` and `notes`. Another column refuses the file.
* **One line per site.** Values are separated by commas, or by semicolons when the header has more
  of them, as spreadsheets write in French; quotes keep a separator or a line break in a value
  (RFC 4180); a byte order mark at the start is skipped.
* **Folders** are paths (`Clients/Valais`), made when missing. The sites of one account (the same
  alias, or the same host, port and user) share one host, named after its alias or its host.
* A site without a `name` is named after its address, or else after the last part of its path.
  `color` is one of `none`, `red`, `green`, `blue`, `yellow`, `cyan`, `magenta`, `orange`.

```csv
folder;host;port;user;path;url
Clients/Valais;ssh.example-host.ch;22;u12ab;~/sites/atelier;https://atelier-exemple.ch
Clients/Valais;ssh.example-host.ch;22;u12ab;~/sites/cave;https://cave-exemple.ch
```

A line Vigie cannot take is listed with its number and the reason: no path, no connection or both,
a host without a user, a port, color or name it cannot read, an account already given another
folder on an earlier line, the same site twice, or not as many values as the header.
