// Fictional updates for stories and tests: never real hosts or sites.
import type {
  OperationView,
  SitePreviewView,
  SiteResultView,
  StepView,
} from "../../bindings/github.com/MarJC5/vigie/internal/app/models";

export const preview: SitePreviewView = {
  siteId: "outdated",
  name: "boulangerie-exemple.ch",
  path: "Clients/Shared hosting/boulangerie-exemple.ch",
  core: { kind: "core", slug: "", language: "", from: "7.1.3", to: "7.1.4", ignored: false, setAside: false },
  plugins: [
    { kind: "plugin", slug: "akismet", language: "", from: "5.3", to: "5.7.2", ignored: false, setAside: false },
    { kind: "plugin", slug: "woocommerce", language: "", from: "9.8.1", to: "10.2.0", ignored: false, setAside: false },
  ],
  themes: [],
  languages: [
    { kind: "language", slug: "", language: "fr_FR", from: "", to: "", ignored: false, setAside: false },
    { kind: "language", slug: "akismet", language: "fr_FR", from: "", to: "", ignored: false, setAside: false },
  ],
  dumpTool: true,
  problem: null,
  backups: "~/.vigie/backups/7c1e4a52-0d55-4f0b-9a8e-3b8c1d2f6a10/",
};

/** A step at a minute and second of 29 October 2026, 14:12, lasting the seconds given. */
const step = (position: number, at: string, secs: number, s: Partial<StepView>): StepView => {
  const started = `2026-10-29T14:12:${at}Z`;
  const end = new Date(new Date(started).getTime() + secs * 1000).toISOString().replace(".000", "");
  return {
    position,
    step: "update",
    itemKind: "",
    slug: "",
    versionBefore: "",
    versionAfter: "",
    started,
    finished: secs >= 0 ? end : "",
    outcome: secs >= 0 ? "done" : "running",
    message: "",
    ...s,
  };
};

const backupDir = ".vigie/backups/7c1e4a52-0d55-4f0b-9a8e-3b8c1d2f6a10/20261029-141203";

const backups = [
  step(0, "03", 6, { step: "backup-database", message: `${backupDir}/database.sql.gz` }),
  step(1, "09", 1, { step: "backup-files", itemKind: "plugin", slug: "akismet", message: "/home/site/wp-content/plugins/akismet" }),
  step(2, "10", 2, { step: "backup-files", itemKind: "plugin", slug: "woocommerce", message: "/home/site/wp-content/plugins/woocommerce" }),
];

const operation = (o: Partial<OperationView>): OperationView => ({
  id: "op-running",
  siteId: "outdated",
  kind: "update",
  started: "2026-10-29T14:12:03Z",
  finished: "",
  outcome: "running",
  summary: "",
  backupDir,
  steps: [],
  ofOperation: "",
  automatic: false,
  rollback: null,
  ...o,
});

/** An update of two plugins and the translations, at its second plugin. */
export const running = operation({
  steps: [
    ...backups,
    step(3, "12", 4, { itemKind: "plugin", slug: "akismet", versionBefore: "5.3", versionAfter: "5.7.2" }),
    step(4, "16", 6, { step: "check", itemKind: "plugin", slug: "akismet" }),
    step(5, "22", -1, { itemKind: "plugin", slug: "woocommerce", versionBefore: "9.8.1", versionAfter: "10.2.0" }),
  ],
});

/** The same update, stopped by the check after WooCommerce. */
export const failed = operation({
  id: "op-failed",
  finished: "2026-10-29T14:12:51Z",
  outcome: "failed",
  summary: "1 of 2",
  steps: [
    ...running.steps!.slice(0, 5),
    step(5, "22", 7, { itemKind: "plugin", slug: "woocommerce", versionBefore: "9.8.1", versionAfter: "10.2.0" }),
    step(6, "29", 9, {
      step: "check",
      itemKind: "plugin",
      slug: "woocommerce",
      outcome: "failed",
      message:
        "new fatal error in debug.log: PHP Fatal error:  Uncaught Error: Call to undefined function wc_get_page_id() in wp-content/themes/boulangerie/functions.php:42",
    }),
  ],
  rollback: { kind: "plugin", slug: "woocommerce", language: "", from: "9.8.1", to: "10.2.0", ignored: false, setAside: false },
});

/** The rollback of WooCommerce after the failure. */
export const rolledBack = operation({
  id: "op-rollback",
  kind: "rollback",
  started: "2026-10-29T14:13:20Z",
  finished: "2026-10-29T14:13:31Z",
  outcome: "completed",
  summary: "plugin woocommerce back to 9.8.1",
  steps: [
    {
      ...step(0, "20", 4, { step: "restore-files", itemKind: "plugin", slug: "woocommerce", versionBefore: "10.2.0", versionAfter: "9.8.1" }),
      started: "2026-10-29T14:13:20Z",
      finished: "2026-10-29T14:13:24Z",
    },
    {
      ...step(1, "24", 7, { step: "check", itemKind: "plugin", slug: "woocommerce" }),
      started: "2026-10-29T14:13:24Z",
      finished: "2026-10-29T14:13:31Z",
    },
  ],
});

/** An update of the whole site that went well, a week earlier. */
export const completed = operation({
  id: "op-done",
  backupDir: ".vigie/backups/7c1e4a52-0d55-4f0b-9a8e-3b8c1d2f6a10/20261022-090310",
  started: "2026-10-22T09:03:10Z",
  finished: "2026-10-22T09:04:41Z",
  outcome: "completed",
  summary: "2 of 2",
  steps: [
    { ...backups[0]!, started: "2026-10-22T09:03:10Z", finished: "2026-10-22T09:03:16Z" },
    { ...backups[1]!, started: "2026-10-22T09:03:16Z", finished: "2026-10-22T09:03:17Z" },
    {
      ...step(2, "00", 0, { itemKind: "plugin", slug: "akismet", versionBefore: "5.2", versionAfter: "5.3" }),
      started: "2026-10-22T09:03:17Z",
      finished: "2026-10-22T09:03:21Z",
    },
    {
      ...step(3, "00", 0, { step: "check", itemKind: "plugin", slug: "akismet" }),
      started: "2026-10-22T09:03:21Z",
      finished: "2026-10-22T09:03:27Z",
    },
    {
      ...step(4, "00", 0, { itemKind: "language", slug: "core" }),
      started: "2026-10-22T09:03:27Z",
      finished: "2026-10-22T09:04:41Z",
    },
  ],
});

/** The newest first, as History returns them; the rollback offered on the newest only. */
export const historyRolledBack: OperationView[] = [rolledBack, { ...failed, rollback: null }, completed];
export const historyToRollBack: OperationView[] = [failed, completed];

/** A host updated: one site updated, one failed, the others not started. */
export const bulk: SiteResultView[] = [
  { siteId: "a", path: "Clients/Shared hosting/atelier-exemple.ch", outcome: "updated", detail: "", operationId: "op-a", rolledBack: [] },
  {
    siteId: "outdated",
    path: "Clients/Shared hosting/boulangerie-exemple.ch",
    outcome: "failed",
    detail: "http://boulangerie-exemple.ch/ answers 500 (critical-error)",
    operationId: "op-failed",
    rolledBack: [],
  },
  { siteId: "c", path: "Clients/Shared hosting/cabinet-exemple.ch", outcome: "not-started", detail: "", operationId: "", rolledBack: [] },
  { siteId: "d", path: "Clients/Shared hosting/reseau-ecoles.ch", outcome: "not-started", detail: "", operationId: "", rolledBack: [] },
  { siteId: "e", path: "Clients/Shared hosting/vieux-site.ch", outcome: "refused", detail: "problem.noDumpTool", operationId: "", rolledBack: [] },
];

/** The update of ADR-0042: WooCommerce breaks the site and goes back on its own, then the run
 goes on with the translations. */
export const autoRunning = operation({
  id: "op-auto",
  steps: [
    ...failed.steps!,
    step(7, "52", -1, { itemKind: "language", slug: "core" }),
  ],
});

/** The rollback of WooCommerce Vigie ran on its own, linked to its update. */
export const autoRolledBack = operation({
  id: "op-auto-back",
  kind: "rollback",
  started: "2026-10-29T14:12:38Z",
  finished: "2026-10-29T14:12:51Z",
  outcome: "completed",
  summary: "plugin woocommerce back to 9.8.1",
  ofOperation: "op-auto",
  automatic: true,
  steps: [
    step(0, "38", 4, { step: "restore-files", itemKind: "plugin", slug: "woocommerce", versionBefore: "10.2.0", versionAfter: "9.8.1" }),
    step(1, "42", 9, { step: "check", itemKind: "plugin", slug: "woocommerce" }),
  ],
});

/** The same update once over: Akismet updated, WooCommerce back, the translations updated. */
export const autoDone = operation({
  id: "op-auto",
  finished: "2026-10-29T14:13:20Z",
  outcome: "completed",
  summary: "1 of 2, 1 rolled back",
  steps: [
    ...failed.steps!,
    step(7, "52", 8, { itemKind: "language", slug: "core" }),
    step(8, "56", 2, { itemKind: "language", slug: "plugin" }),
    step(9, "58", 2, { itemKind: "language", slug: "theme" }),
  ],
});

/** The rollback on its own that failed: the site still breaks once WooCommerce is back. */
export const autoRollbackFailed = operation({
  ...autoRolledBack,
  outcome: "failed",
  steps: [
    autoRolledBack.steps![0]!,
    step(1, "42", 9, {
      step: "check",
      itemKind: "plugin",
      slug: "woocommerce",
      outcome: "failed",
      message: "http://boulangerie-exemple.ch/ shows the critical error page of WordPress (status 500)",
    }),
  ],
});

/** Its update, stopped there, the rollback still offered to try again. */
export const autoStopped = operation({
  ...failed,
  id: "op-auto",
  summary: "1 of 2",
});

/** The history after it: the rollback on its own above the update it came from. */
export const historyAuto: OperationView[] = [autoRolledBack, autoDone, completed];

/** A host updated, an item gone back on its own on one site: every site up. */
export const bulkAuto: SiteResultView[] = [
  { siteId: "a", path: "Clients/Shared hosting/atelier-exemple.ch", outcome: "updated", detail: "", operationId: "op-a", rolledBack: [] },
  {
    siteId: "outdated",
    path: "Clients/Shared hosting/boulangerie-exemple.ch",
    outcome: "updated",
    detail: "",
    operationId: "op-auto",
    rolledBack: [
      {
        kind: "plugin",
        slug: "woocommerce",
        version: "10.2.0",
        backTo: "9.8.1",
        reason: "http://boulangerie-exemple.ch/ shows the critical error page of WordPress (status 500)",
      },
    ],
  },
  { siteId: "c", path: "Clients/Shared hosting/cave-exemple.ch", outcome: "updated", detail: "", operationId: "op-c", rolledBack: [] },
  { siteId: "d", path: "Clients/Shared hosting/reseau-ecoles.ch", outcome: "updated", detail: "", operationId: "op-d", rolledBack: [] },
];
