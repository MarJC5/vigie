# Assistants: Vigie in an MCP client

An assistant (Claude or any other client of the Model Context Protocol) can read what Vigie knows
of your sites, scan them, and, if you allow it, update, copy and ignore, each action waiting for
your yes. The client starts `vigie mcp` on your computer and speaks to it over its standard input
and output: nothing listens on the network, and the server lives only as long as the client that
started it.

The assistants are **off** until you choose otherwise, in **Settings, Assistants**.

## What an assistant may do

The level is set in **Settings, Assistants**, and read at every call: a change applies at once,
even to a client already started.

| Level | Tools |
| --- | --- |
| **Off** (the default) | None. A call answers that the assistants are off, and where to turn them on. |
| **Read** | `sites`, `site`, `journal`, `history`, `copies`, and `scan`, which only reads on the servers. |
| **Actions** | Also `update`, `copy`, `ignore` and `stop_ignoring`, each confirmed by you. |

**Never allowed**, at any level: deleting, moving or renaming, editing a connection, the keys and
the trust of the servers, export, import, the settings, installing WP-CLI, rolling back by hand.
They stay in the window and the command line.

## What an assistant never gets

Vigie answers with structured data: versions, slugs, statuses, levels, dates, counts. A text that
comes from a site or a source (a plugin's name, a finding's title) is cut to 200 characters, on one
line, in a field of its own, and every answer says that such texts are data, not instructions.

Never sent: the content of `debug.log` (only what Vigie concluded from it), the notes of your
folders, hosts and sites, the e-mail addresses of the administrators, the SSH keys and the servers'
keys, the paths inside your copies.

## Connect a client

The client starts the `vigie` command, which must be linked first (**Settings, General, Command
line**). **Settings, Assistants** then gives the lines to add to the client's MCP servers, with the
path of the command on your computer, and a button that copies them:

```json
{
  "mcpServers": {
    "vigie": {
      "command": "/usr/local/bin/vigie",
      "args": ["mcp"]
    }
  }
}
```

Where a client keeps its MCP servers:

| Client | Where |
| --- | --- |
| Claude Desktop | In its settings, **Developer**, **Edit Config**: `~/Library/Application Support/Claude/claude_desktop_config.json` on macOS. Quit it and open it again afterwards: it reads the file when it starts. |
| Claude Code | The command below, or `.mcp.json` at the root of a project, in the same form. |
| Cursor | `~/.cursor/mcp.json`, or `.cursor/mcp.json` in a project, in the same form. |
| VS Code | `.vscode/mcp.json` in a project: the entry goes under `"servers"` instead of `"mcpServers"`, with `"type": "stdio"` beside `command` and `args`. |

When the file already lists servers, add only the `"vigie"` entry inside its `"mcpServers"`, not a
second whole object. With the level at **Off**, a client connects but sees no tool.

A client configured on the command line takes the same command, for example Claude Code:

```sh
claude mcp add vigie -- /usr/local/bin/vigie mcp
```

## The confirmation

An action never leaves on the assistant's word alone. Its first call does nothing on the servers:
it answers with what the action will do (the items of an update and their versions, the sites a
copy reads, what an ignore hides).

* **A client that can ask you** (it declares elicitation): Vigie asks you through it, with that
  preview. **Accept** runs the action; **Decline** or closing the question changes nothing.
  The assistant cannot answer in your place.
* **Any other client** gets the preview and a confirmation. Only a second call with it runs the
  action: once, within five minutes, for the same tool and the same arguments.

In both cases Vigie reads the preview again before running: if it changed in between (a new
version came out), you are asked again. One action at a time runs from a client.

**The limit**: a client set to approve every tool on its own can chain the preview and the
confirmed call; the confirmation then proves a preview was made, not that you read it. Keep your
client asking before each tool, or keep the level at **Read**.

## In the journal

A scan, an update, a rollback or a copy started by an assistant reads as one in the Activity
journal and in the site's history, "by an assistant", with the name the client gives. Reads are not
noted. As with the command line, Vigie sends no notification at the end: the assistant tells you,
and the journal keeps it.
