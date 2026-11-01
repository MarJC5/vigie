// Fictional findings for stories and tests, as the Go side serves them: never real hosts or sites.
import type { FindingView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";

const finding = (f: Partial<FindingView> & Pick<FindingView, "kind" | "level">): FindingView => ({
  key: `${f.kind}|${f.subject ?? "site"}|${f.identifier ?? ""}`,
  subject: "site",
  name: "",
  identifier: "",
  version: "",
  facts: {},
  ignore: null,
  ...f,
});

export const exploited = finding({
  kind: "vulnerability",
  level: "urgent",
  subject: "plugin:woocommerce",
  name: "WooCommerce",
  identifier: "0d1f5a2c-woo",
  version: "9.8.1",
  facts: {
    name: "SQL injection without authentication",
    severity: "critical",
    score: "9.8",
    kev: "true",
    fixed: "9.8.3",
    sources: "https://www.cve.org/CVERecord?id=CVE-2026-41234 https://example.test/advisory",
  },
});

export const publicLog = finding({
  kind: "debug-log",
  level: "urgent",
  facts: { url: "https://boulangerie-exemple.ch/wp-content/debug.log" },
});

export const xss = finding({
  kind: "vulnerability",
  level: "attention",
  subject: "plugin:contact-form-7",
  name: "Contact Form 7",
  identifier: "7c2e-cf7",
  version: "6.0.4",
  facts: { name: "Stored cross-site scripting", severity: "medium", score: "5.4", fixed: "6.0.6", sources: "https://www.cve.org/CVERecord?id=CVE-2026-38811" },
});

export const closed = finding({
  kind: "plugin-closed",
  level: "attention",
  subject: "plugin:old-gallery",
  name: "Old Gallery",
  version: "2.1",
  facts: { date: "2026-01-30", reason: "author-request" },
});

export const noTwoFactor = finding({
  kind: "administrator",
  level: "attention",
  subject: "user:marie",
  facts: { email: "marie@boulangerie-exemple.ch", registered: "2024-03-12", "two-factor": "off" },
});

export const sameName = finding({
  kind: "plugin-file",
  level: "attention",
  subject: "plugin:gutenblocks",
  name: "Gutenblocks",
  version: "1.0.0",
  facts: { added: "231", changed: "1", files: "app/blocks/home-hero/block.json, gutenblocks.php", mismatch: "true" },
});

export const fileEditing = finding({ kind: "file-editing", level: "attention" });

export const notChecked = finding({ kind: "check", level: "not-checked", identifier: "wordpress.org", facts: { reason: "off" } });

export const domainSilent = finding({
  kind: "domain",
  level: "info",
  facts: { domain: "boulangerie-exemple.ch", reason: "not-published" },
});

export const certificate = finding({
  kind: "certificate",
  level: "info",
  version: "2026-12-25",
  facts: { expires: "2026-12-25T10:00:00Z", issuer: "Let's Encrypt" },
});

export const hidden = finding({
  kind: "hidden-from-search",
  level: "attention",
  ignore: { untilVersion: "", reason: "staging copy, hidden on purpose", created: "2026-10-12T09:30:00Z" },
});

// The scan's complements (ADR-0028): fatal errors of the last week, PHP on the web near its end.
export const debugFatal = finding({
  kind: "debug-fatal",
  level: "attention",
  facts: {
    count: "3",
    last: "2026-10-31T02:14:07Z",
    message:
      "PHP Fatal error:  Uncaught Error: Call to undefined function wc_get_order() in /home/clients/u12ab/sites/boulangerie/wp-content/themes/astra-child/functions.php:42",
  },
});

export const phpWeb = finding({
  kind: "php-support",
  level: "attention",
  subject: "php",
  version: "8.2.29",
  facts: { status: "s", end: "2026-12-31", on: "web" },
});

/** A site's findings, the worst first, as a scan ranks them. */
export const findings: FindingView[] = [exploited, publicLog, xss, closed, sameName, noTwoFactor, fileEditing, notChecked, domainSilent, certificate, hidden];
