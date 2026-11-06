# Assistants

An assistant, Claude or any other MCP client, can read what Vigie knows of your sites, and act on
them if you allow it. The client starts `vigie mcp` on this computer: nothing listens on the
network.

## Choose what it may do

In **Settings, Assistants**:

* **Off**, the default: Vigie answers no assistant.
* **Read**: the sites, the journal, the history, the copies, and a scan, which only reads on the
  servers.
* **Actions**: also update, copy to this computer, and ignore.

Deleting, moving, the connections and keys of the servers, export, import and the settings are
never allowed. The content of `debug.log`, your notes and the administrators' e-mail addresses are
never sent.

## Connect a client

Link the `vigie` command first, in **Settings, General, Command line**. **Settings, Assistants**
then gives the lines to add to your client's MCP servers: copy them with the button. Each client
keeps them in a file of its own: [where, client by client](../assistants.md#connect-a-client).

## Your yes before each action

An action first answers what it will do, and nothing more. A client that can ask you shows Vigie's
question: **Accept** runs it, **Decline** changes nothing. Another client gets a confirmation that
works once, within five minutes. Keep your client asking before each tool: a client set to approve
everything on its own could approve in your place.

What an assistant started shows in the journal, "by an assistant".

Learn more: [the levels, the confirmation and its limit](../assistants.md).
