// Fictional scan results for stories and tests: never real hosts or sites.
import type { HostView, ScanView, SiteRow, SiteView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { exploited } from "./findings";

const scan = (s: Partial<ScanView>): ScanView => ({
  finished: "2026-10-29T08:57:00Z",
  outcome: "completed",
  status: "ok",
  problem: null,
  wordpress: "7.1.3",
  phpCli: "8.4.27",
  wpCli: "2.12.0",
  home: "https://atelier-exemple.ch",
  checked: { core: true, plugins: true, themes: true, languages: true },
  pending: { core: 0, plugins: 0, themes: 0, languages: 0 },
  plugins: 12,
  themes: 2,
  updates: [],
  sourceUnknown: [],
  filesChecked: "2026-10-29T08:57:00Z",
  findings: [],
  counts: { urgent: 0, attention: 0, notChecked: 0, ignored: 0 },
  ...s,
});

export const outdatedSite: SiteView = {
  id: "outdated",
  name: "boulangerie-exemple.ch",
  path: "~/sites/boulangerie",
  url: "https://boulangerie-exemple.ch",
  multisite: false,
  scan: scan({
    status: "warn",
    home: "https://boulangerie-exemple.ch",
    pending: { core: 1, plugins: 2, themes: 0, languages: 2 },
    updates: [
      { kind: "core", slug: "", title: "", language: "", version: "7.1.3", updateVersion: "7.1.4", update: "available" },
      { kind: "plugin", slug: "akismet", title: "Akismet Anti-spam", language: "", version: "5.3", updateVersion: "5.7.2", update: "available" },
      { kind: "plugin", slug: "woocommerce", title: "WooCommerce", language: "", version: "9.8.1", updateVersion: "10.2.0", update: "available" },
      { kind: "plugin", slug: "old-gallery", title: "Old Gallery", language: "", version: "2.1", updateVersion: "3.0", update: "unavailable" },
      { kind: "language", slug: "", title: "", language: "fr_FR", version: "", updateVersion: "", update: "available" },
      { kind: "language", slug: "akismet", title: "", language: "fr_FR", version: "", updateVersion: "", update: "available" },
    ],
    sourceUnknown: ["Theme Builder Pro"],
  }),
};

export const upToDateSite: SiteView = {
  id: "atelier",
  name: "atelier-exemple.ch",
  path: "~/sites/atelier",
  url: "https://atelier-exemple.ch",
  multisite: false,
  scan: scan({}),
};

export const offlineSite: SiteView = {
  id: "offline",
  name: "cabinet-exemple.ch",
  path: "~/sites/cabinet",
  url: "https://cabinet-exemple.ch",
  multisite: false,
  scan: scan({ status: "warn", checked: { core: false, plugins: false, themes: false, languages: false } }),
};

export const failedSite: SiteView = {
  id: "failed",
  name: "vieux-site.ch",
  path: "~/sites/vieux",
  url: "",
  multisite: false,
  scan: scan({
    outcome: "failed",
    status: "unknown",
    problem: { code: "failure.timedOut", detail: "ssh: connect to host ssh.example-host.ch port 22: Operation timed out" },
    wordpress: "",
    phpCli: "",
    wpCli: "",
    home: "",
    plugins: 0,
    themes: 0,
  }),
};

export const newSite: SiteView = { id: "new", name: "nouveau.example.test", path: "~/sites/nouveau", url: "", multisite: false, scan: null };

export const hostRows: SiteRow[] = [
  { id: "atelier", name: "atelier-exemple.ch", path: "Clients/Shared hosting/atelier-exemple.ch", status: "ok", wordpress: "7.1.3", phpCli: "8.4.27", pending: 0, worst: null },
  { id: "outdated", name: "boulangerie-exemple.ch", path: "Clients/Shared hosting/boulangerie-exemple.ch", status: "danger", wordpress: "7.1.3", phpCli: "8.4.27", pending: 4, worst: exploited },
  { id: "new", name: "nouveau.example.test", path: "Clients/Shared hosting/nouveau.example.test", status: "", wordpress: "", phpCli: "", pending: 0, worst: null },
];

export const sharedHost: HostView = {
  id: "shared",
  name: "Shared hosting",
  alias: "",
  hostname: "ssh.example-host.ch",
  port: 22,
  user: "u12ab",
  wpCli: "wp-cli",
  sites: hostRows,
};
