// A fictional multisite network (ADR-0029), for stories and tests: never a real site.
import type { ExtensionView, FindingView, Node, SiteView, SubsiteView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { outdatedSite } from "./scans";
import { sampleTree } from "./tree";

const subsite = (s: Partial<SubsiteView> & Pick<SubsiteView, "blogId" | "label">): SubsiteView => ({
  url: `https://reseau-ecoles.ch${s.label === "/" ? "/" : `${s.label}/`}`,
  adminUrl: `https://reseau-ecoles.ch${s.label === "/" ? "" : s.label}/wp-admin/`,
  state: "public", read: true, theme: "astra", themeName: "Astra", themeVersion: "4.8.1", adminEmail: "",
  plugins: [], admins: [], ...s,
});

export const subsites: SubsiteView[] = [
  subsite({ blogId: 1, label: "/", theme: "twentytwentyfive", themeName: "Twenty Twenty-Five", themeVersion: "1.2", adminEmail: "admin@reseau-ecoles.ch", admins: ["admin", "jdupont"] }),
  subsite({ blogId: 2, label: "/alpha", adminEmail: "direction@ecole-alpha.ch", plugins: ["contact-form-7", "the-events-calendar"], admins: ["admin", "marie"] }),
  subsite({ blogId: 3, label: "/beta", adminEmail: "beta@reseau-ecoles.ch", plugins: ["contact-form-7"], admins: ["marie"] }),
  subsite({ blogId: 4, label: "/gamma", state: "archived", read: false, theme: "", themeName: "", themeVersion: "" }),
];

const extension = (e: Partial<ExtensionView> & Pick<ExtensionView, "kind" | "slug" | "title" | "version">): ExtensionView => ({
  status: "inactive", update: "none", updateVersion: "", autoUpdate: false, onWordPressOrg: true, worst: null, ignore: null, subsites: [], ...e,
});

const extensions: ExtensionView[] = [
  extension({ kind: "plugin", slug: "woocommerce", title: "WooCommerce", version: "9.8.1", status: "active-network" }),
  extension({ kind: "plugin", slug: "contact-form-7", title: "Contact Form 7", version: "6.0.4", subsites: [2, 3] }),
  extension({ kind: "plugin", slug: "the-events-calendar", title: "The Events Calendar", version: "6.8.2", subsites: [2] }),
  extension({ kind: "plugin", slug: "hello", title: "Hello Dolly", version: "1.7.2" }),
  extension({ kind: "theme", slug: "astra", title: "Astra", version: "4.8.1", subsites: [2, 3] }),
  extension({ kind: "theme", slug: "twentytwentyfive", title: "Twenty Twenty-Five", version: "1.2", status: "active", subsites: [1] }),
];

const admin = (login: string, level: string, facts: Record<string, string>): FindingView => ({
  key: `administrator|user:${login}|`, kind: "administrator", level, subject: `user:${login}`, name: "", identifier: "", version: "", facts, ignore: null,
});

const findings: FindingView[] = [
  admin("jdupont", "attention", { email: "jdupont@reseau-ecoles.ch", registered: "2021-02-04", "two-factor": "off", super: "true", sites: "1" }),
  admin("marie", "attention", { email: "marie@ecole-alpha.ch", registered: "2024-09-12", "two-factor": "off", sites: "2,3" }),
  admin("admin", "info", { email: "admin@reseau-ecoles.ch", registered: "2021-02-04", "two-factor": "on", super: "true", sites: "1,2" }),
];

/** The network's page, with its latest scan. */
export const networkSite: SiteView = {
  ...outdatedSite,
  id: "ecoles",
  name: "reseau-ecoles.ch",
  url: "https://reseau-ecoles.ch",
  multisite: true,
  adminUrl: "https://reseau-ecoles.ch/wp-admin/",
  subsites,
  scan: {
    ...outdatedSite.scan!,
    status: "warn",
    home: "https://reseau-ecoles.ch",
    extensions,
    findings,
    counts: { urgent: 0, attention: 2, notChecked: 0, ignored: 0 },
  },
};

/** The site list with the network's subsites to unfold. */
export const networkTree: Node[] = sampleTree.map((n) =>
  n.id === "ecoles"
    ? { ...n, subsites: 4, status: "warn", subsiteList: subsites.map((s) => ({ blogId: s.blogId, label: s.label, state: s.state })) }
    : n,
);
