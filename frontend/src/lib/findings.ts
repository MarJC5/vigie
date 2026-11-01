// The findings of a security scan as the window writes them: Vigie stores data (a kind, a subject,
// facts), the texts come from the catalogs in the interface language.
import type { FindingView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";

type T = (key: string, vars?: Record<string, unknown>) => string;

export interface FindingText {
  title: string;
  detail: string;
  /** Where to read more: the vulnerability's record, the plugin's page on wordpress.org. */
  link?: { label: string; href: string };
  /** Words read on the server, shown as they are: the latest fatal error of debug.log. */
  quote?: string;
}

/** The groups of the Security tab, in their order; set aside findings go last, whatever their level. */
export type Group = "urgent" | "attention" | "not-checked" | "info" | "ignored";
export const groupOrder: Group[] = ["urgent", "attention", "not-checked", "info", "ignored"];

export function groupOf(f: FindingView): Group {
  return f.ignore ? "ignored" : (f.level as Group);
}

/** The findings by group, in the order of the groups and, within one, as the scan ranked them. */
export function groupsOf(findings: FindingView[]): Array<{ group: Group; items: FindingView[] }> {
  return groupOrder
    .map((group) => ({ group, items: findings.filter((f) => groupOf(f) === group) }))
    .filter((g) => g.items.length > 0);
}

/** A date as the Swiss write it, 30.10.2026, in both languages. */
export function day(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const two = (n: number) => String(n).padStart(2, "0");
  return `${two(d.getDate())}.${two(d.getMonth() + 1)}.${d.getFullYear()}`;
}

/** A time of day, 14:12. */
export function clock(iso: string): string {
  const d = new Date(iso);
  return `${String(d.getHours()).padStart(2, "0")}:${String(d.getMinutes()).padStart(2, "0")}`;
}

// A plugin's or theme's title, else its slug; WordPress and PHP by their names.
function subjectName(f: FindingView): string {
  if (f.name) return f.name;
  if (f.subject === "core") return "WordPress";
  if (f.subject === "php") return "PHP";
  return f.subject.slice(f.subject.indexOf(":") + 1);
}

function slugOf(f: FindingView): string {
  return f.subject.slice(f.subject.indexOf(":") + 1);
}

// A link's label: the CVE number when the address names one, else the site's name.
function linkLabel(href: string): string {
  const cve = href.match(/CVE-\d{4}-\d+/i);
  if (cve) return cve[0].toUpperCase();
  try {
    return new URL(href).hostname.replace(/^www\./, "");
  } catch {
    return href;
  }
}

// The closure reasons wordpress.org gives, by their code; another one shows as given.
const closeReasons = ["security-issue", "author-request", "guideline-violation", "licensing-trademark-violation", "merged-into-core", "unused"];

/** The title, detail and link of a finding, in the interface language. */
/** A finding in words. subsites names a network's subsites by their ID, for the administrators
 of a network (ADR-0029). */
export function findingText(f: FindingView, t: T, lang: string, subsites?: Record<string, string>): FindingText {
  const facts = f.facts ?? {};
  const name = subjectName(f);
  const k = (key: string, vars?: Record<string, unknown>) => t(`security.finding.${key}`, vars);
  const join = (...parts: Array<string | false | undefined>) => parts.filter(Boolean).join(" · ");
  const stale = facts.fetched ? k("stale", { date: day(facts.fetched) }) : undefined;
  const wporg = { label: "wordpress.org", href: `https://wordpress.org/plugins/${slugOf(f)}/` };

  switch (f.kind) {
    case "vulnerability": {
      const score = facts.score ? new Intl.NumberFormat(lang, { maximumFractionDigits: 1 }).format(Number(facts.score)) : "";
      const severity = facts.severity ? k(`severity.${facts.severity}`) : "";
      const sources = (facts.sources ?? "").split(" ").filter(Boolean);
      return {
        title: k("vulnerability.title", { subject: name, version: f.version, name: facts.name }),
        detail: join(
          score ? k("vulnerability.score", { score, severity }) : severity || k("vulnerability.unscored"),
          facts.kev === "true" && k("vulnerability.kev"),
          facts.unfixed === "true" ? k("vulnerability.unfixed") : facts.fixed && k("vulnerability.fixed", { version: facts.fixed }),
          stale,
        ),
        link: sources[0] ? { label: linkLabel(sources[0]), href: sources[0] } : undefined,
      };
    }
    case "php-support": {
      // About the PHP serving the pages when the scan knew it (ADR-0028).
      const web = facts.on === "web" ? "Web" : "";
      return {
        title: k(`${facts.status === "d" ? "phpSupport.unmaintained" : "phpSupport.securityOnly"}${web}`, { version: f.version }),
        detail: facts.end ? k("phpSupport.end", { date: day(facts.end) }) : "",
      };
    }
    case "debug-fatal": {
      const count = Number(facts.count ?? 0);
      return {
        title: k("debugFatal.title", { count }),
        detail: facts.last ? k(count > 1 ? "debugFatal.latest" : "debugFatal.one", { day: day(facts.last), time: clock(facts.last) }) : "",
        quote: facts.message,
      };
    }
    case "plugin-closed": {
      const reason = closeReasons.includes(facts.reason ?? "") ? k(`closeReason.${facts.reason}`) : (facts.reason ?? "");
      return {
        title: k("pluginClosed.title", { name }),
        detail: join(
          facts.date ? k("pluginClosed.on", { date: day(facts.date), reason }) : reason,
          facts["other-plugin"] === "true" && k("pluginClosed.otherPlugin"),
          stale,
        ),
        link: wporg,
      };
    }
    case "plugin-abandoned":
      return {
        title: k("pluginAbandoned.title", { name }),
        detail: k("pluginAbandoned.detail", { date: day(facts["last-release"] ?? "") }),
        link: wporg,
      };
    case "core-file": {
      // A file often deleted on purpose has its own finding; the others are counted together.
      if (f.identifier) return { title: k("coreFile.removed", { file: f.identifier }), detail: k("coreFile.removedDetail") };
      const changed = Number(facts.changed ?? 0);
      const missing = Number(facts.missing ?? 0);
      const added = Number(facts.added ?? 0);
      return {
        title: k("coreFile.title", { count: changed + missing + added }),
        detail: join(k("coreFile.counts", { changed, missing, added }), facts.files && k("coreFile.files", { files: facts.files }), k("coreFile.detail")),
      };
    }
    case "plugin-file": {
      const added = Number(facts.added ?? 0);
      const changed = Number(facts.changed ?? 0);
      return {
        title: k("pluginFile.title", { name, count: added + changed }),
        detail: join(
          k("pluginFile.counts", { added, changed }),
          facts.files && k("pluginFile.files", { files: facts.files }),
          facts.uncomparable === "true"
            ? k("pluginFile.uncomparable")
            : facts.mismatch === "true"
              ? k("pluginFile.mismatch")
              : k("pluginFile.detail"),
        ),
      };
    }
    case "plugin-unverified":
      return { title: k("pluginUnverified.title", { name }), detail: k("pluginUnverified.detail", { version: f.version }) };
    case "uploads-php":
      return { title: k("uploadsPhp.title", { file: f.identifier }), detail: k("uploadsPhp.detail") };
    case "uploads-php-more":
      return { title: k("uploadsPhpMore.title", { count: Number(facts.shown) }), detail: k("uploadsPhp.detail") };
    case "administrator": {
      const login = f.subject.slice("user:".length);
      const state = facts["two-factor"] ?? "unknown";
      // On a network: a super administrator says so first, then which subsites each administers.
      const sites = (facts.sites ?? "")
        .split(",")
        .filter(Boolean)
        .map((id) => (subsites?.[id] === "/" ? k("administrator.mainSite") : (subsites?.[id] ?? id)));
      return {
        title: k(facts.super === "true" ? `administrator.super_${state}` : `administrator.${state}`, { login }),
        detail: join(
          facts.email,
          facts.registered && k("administrator.since", { date: day(facts.registered) }),
          sites.length > 0 && k("administrator.sites", { sites: new Intl.ListFormat(lang, { type: "conjunction" }).format(sites) }),
        ),
      };
    }
    case "cron-overdue":
      return {
        title: k("cronOverdue.title", { count: Number(facts.count) }),
        detail: k("cronOverdue.detail", { hook: facts.hook, date: day(facts.since ?? "") }),
      };
    case "autoload-size": {
      const kb = (bytes: string | undefined) => new Intl.NumberFormat(lang).format(Math.round(Number(bytes) / 1000));
      return { title: k("autoloadSize.title"), detail: k("autoloadSize.detail", { size: kb(facts.bytes), limit: kb(facts.limit) }) };
    }
    case "database-version":
      return { title: k("databaseVersion.title", { version: facts.version }), detail: "" };
    case "certificate": {
      const until = day(facts.expires ?? "");
      if (facts.problem) return { title: k(`certificate.${facts.problem}`), detail: join(k("certificate.until", { date: until }), facts.issuer) };
      return { title: k("certificate.valid", { date: until }), detail: facts.issuer ?? "" };
    }
    case "domain": {
      if (facts.reason) {
        const tld = (facts.domain ?? "").slice((facts.domain ?? "").lastIndexOf(".") + 1);
        return { title: k("domain.title", { domain: facts.domain }), detail: k(`domain.${facts.reason}`, { tld }) };
      }
      return { title: k("domain.title", { domain: facts.domain }), detail: join(k("domain.expires", { date: day(facts.expires ?? "") }), stale) };
    }
    case "debug-log":
      return { title: k("debugLog.title"), detail: k("debugLog.detail", { url: facts.url }) };
    case "check":
      return {
        title: k(`check.${keyOf(f.identifier)}`),
        detail: facts.reason === "off" ? k("check.off") : (facts.error ?? ""),
      };
  }
  // A setting, or a kind this window does not know yet: its key names it.
  return { title: k(`${camel(f.kind)}.title`), detail: k(`${camel(f.kind)}.detail`) };
}

/** An identifier as a key of the catalogs, where a dot separates keys: wordpress.org → wordpressOrg. */
export function keyOf(id: string): string {
  return id.replace(/\.(\w)/g, (_, c: string) => c.toUpperCase());
}

// file-editing → fileEditing, the catalogs' style.
function camel(kind: string): string {
  return kind.replace(/-(\w)/g, (_, c: string) => c.toUpperCase());
}

/** What an ignored finding says about its ignore. */
export function ignoreText(f: FindingView, t: T): string {
  if (!f.ignore) return "";
  const date = day(f.ignore.created);
  return f.ignore.untilVersion
    ? t("security.ignoredUntil", { version: f.ignore.untilVersion, date })
    : t("security.ignoredForever", { date, reason: f.ignore.reason });
}
