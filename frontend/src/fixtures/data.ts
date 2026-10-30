// Fictional exports and imports used by stories and tests: never real hosts or sites.
import type {
  ExportSummary,
  ImportItem,
  ImportPreview,
  SSHConfigView,
} from "../../bindings/github.com/MarJC5/vigie/internal/app/models";

export const everything: ExportSummary = { path: "", folders: 3, hosts: 6, sites: 11, ignores: 4, historyBytes: 3_150_000, problem: null };
export const oneHost: ExportSummary = { path: "Clients/Shared hosting", folders: 0, hosts: 1, sites: 3, ignores: 0, historyBytes: 412_000, problem: null };

const item = (i: Partial<ImportItem> & Pick<ImportItem, "kind" | "path" | "status">): ImportItem => ({
  reason: "",
  detail: "",
  account: "",
  sitePath: "",
  subsites: 0,
  scans: 0,
  operations: 0,
  line: 0,
  value: "",
  ...i,
});

export const vigiePreview: ImportPreview = {
  cancelled: false,
  source: "vigie",
  file: "vigie-2026-10-30.json",
  exported: "2026-10-30T13:12:00Z",
  app: "0.6.0",
  history: true,
  lines: 0,
  separator: "",
  addOn: "",
  addOnTitle: "",
  report: "",
  into: "",
  items: [
    item({ kind: "folder", path: "Clients/Valais", status: "added" }),
    item({ kind: "host", path: "Clients/Valais/Atelier", status: "added", account: "atelier@ssh.example-host.ch" }),
    item({ kind: "site", path: "Clients/Valais/Atelier/atelier-exemple.ch", status: "added", sitePath: "~/www/atelier", scans: 12, operations: 3 }),
    item({ kind: "site", path: "Clients/Valais/Atelier/reseau-ecoles.ch", status: "added", sitePath: "~/www/ecoles", subsites: 3 }),
    item({ kind: "host", path: "Clients/Valais/Atelier (2)", status: "added", reason: "renamed", account: "deploy@203.0.113.24:2222" }),
    item({ kind: "host", path: "Clients/Shared hosting", status: "present", reason: "same-connection" }),
    item({ kind: "site", path: "Clients/Shared hosting/boulangerie-exemple.ch", status: "present", reason: "same-path" }),
  ],
  keys: [
    { hostId: "h-atelier", host: "Clients/Valais/Atelier", server: "ssh.example-host.ch", type: "ssh-ed25519", fingerprint: "SHA256:3f9k1QvXwZpL0d7mYb2hT8sRcN4eJuA6oGiK5lVtWq0", refused: "" },
    { hostId: "h-vps", host: "Clients/Valais/Atelier (2)", server: "[203.0.113.24]:2222", type: "ssh-ed25519", fingerprint: "SHA256:Zx81LmQ0aP4rTn9wC2vB7yHk5dJ3uEs6gFoI1qRtYw8", refused: "other-trusted" },
  ],
  added: 5,
  problem: null,
};

export const csvPreview: ImportPreview = {
  ...vigiePreview,
  source: "csv",
  file: "sites.csv",
  exported: "",
  app: "",
  history: false,
  lines: 14,
  separator: ";",
  items: [
    item({ kind: "folder", path: "Valais", status: "added" }),
    item({ kind: "host", path: "Valais/ssh.example-host.ch", status: "added", account: "atelier@ssh.example-host.ch" }),
    item({ kind: "site", path: "Valais/ssh.example-host.ch/atelier-exemple.ch", status: "added", sitePath: "~/www/atelier" }),
    item({ kind: "line", path: "", status: "refused", reason: "no-path", line: 6, detail: "Valais;ssh.example-host.ch;22;atelier;;https://vieux-exemple.ch" }),
    item({ kind: "line", path: "", status: "refused", reason: "bad-port", line: 11, value: "22a", detail: "Valais;ssh.example-host.ch;22a;atelier;~/www/test;" }),
  ],
  keys: [],
  added: 3,
};

export const sshConfig: SSHConfigView = {
  hosts: [
    { alias: "shared", account: "u12ab@ssh.example-host.ch:22", present: "Clients/Shared hosting", error: "" },
    { alias: "atelier", account: "atelier@ssh.example-host.ch:22", present: "", error: "" },
    { alias: "vps-perso", account: "deploy@203.0.113.24:2222", present: "", error: "" },
    { alias: "old-box", account: "", present: "", error: "ssh -G old-box: exit status 255" },
  ],
  patterns: 2,
  problem: null,
};
