# Import and export

Your folders, hosts and sites can be saved to a file, moved to another computer, or brought in
from elsewhere. Nothing is sent anywhere.

## Export

**Settings, Data, Export…**, or **Export…** in the menu of a folder or host. The file is readable
by you only. It names your servers and SSH users and holds your notes: keep it somewhere safe.
**Never** a private key or a password.

## Import

In **Settings, Data**:

- **A Vigie file**: an export, from this computer or another one.
- **A CSV file**: one line per site, from a spreadsheet for example.
- **SSH configuration**: the hosts of `~/.ssh/config` you choose.

A preview shows what will be added. Only what is new is added, nothing existing is changed. The
server keys a file brings are kept only once you confirm them.

## From another program

An add-on brings in data from another program, for example FileZilla's site manager. Install it,
then open its page in **Add-ons**: what it wrote opens in the same preview.

Learn more: [the export format and the CSV columns](../data.md).
