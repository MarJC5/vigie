import type { TFunction } from "i18next";
import { useTranslation } from "react-i18next";

import type { ActivityEntry, Change, Journal, Node, NoticeView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { clock } from "../../lib/findings";
import { Button } from "../Button/Button";
import { ChangeList } from "../ChangeList/ChangeList";
import { Icon, type IconName } from "../Icon/Icon";
import { SelectField } from "../SelectField/SelectField";
import { StatusDot } from "../StatusDot/StatusDot";
import styles from "./ActivityPage.module.css";

export interface ActivityPageProps {
  journal: Journal;
  /** How many days the journal covers. */
  days: number;
  /** The folder, host or site shown; "" for every site. */
  filter: string;
  /** The site manager, for the filter. */
  nodes: Node[];
  onFilter: (nodeId: string) => void;
  /** Shows older days. */
  onOlder: () => void;
  /** Opens the page of a site or host. */
  onOpen: (nodeId: string) => void;
  /** Today, for the names of the days; now by default. */
  now?: Date;
}

/** What happened, by day, newest first: the scans with what they changed, updates and rollbacks. */
export function ActivityPage({ journal, days, filter, nodes, onFilter, onOlder, onOpen, now = new Date() }: ActivityPageProps) {
  const { t, i18n } = useTranslation();
  const entries = journal.entries ?? [];
  const groups: Array<{ day: string; entries: ActivityEntry[] }> = [];
  for (const e of entries) {
    const day = new Date(e.at).toDateString();
    const last = groups.at(-1);
    if (last?.day === day) last.entries.push(e);
    else groups.push({ day, entries: [e] });
  }
  const dayName = (day: string) => {
    const yesterday = new Date(now);
    yesterday.setDate(now.getDate() - 1);
    if (day === now.toDateString()) return t("activity.today");
    if (day === yesterday.toDateString()) return t("activity.yesterday");
    return new Intl.DateTimeFormat(i18n.language, { weekday: "long", day: "numeric", month: "long" }).format(new Date(day));
  };
  const options = [
    { value: "", label: t("activity.everything") },
    ...nodes.map((n) => ({ value: n.id, label: n.path })),
  ];
  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}>{t("activity.title")}</h1>
        <span className={styles.filter}>
          <span aria-hidden="true">{t("activity.filter")}</span>
          <SelectField label={t("activity.filter")} hideLabel value={filter} options={options} onChange={onFilter} />
        </span>
      </div>
      {groups.length === 0 && <p className={styles.empty}>{t("activity.empty", { days })}</p>}
      {groups.map((g) => (
        <section key={g.day} className={styles.day}>
          <h2 className={styles.dayTitle}>{dayName(g.day)}</h2>
          <ul className={styles.list}>
            {g.entries.map((e, i) => (
              <Entry key={`${e.at}-${i}`} entry={e} onOpen={onOpen} />
            ))}
          </ul>
        </section>
      ))}
      <span>
        <Button onClick={onOlder}>{t("activity.older")}</Button>
      </span>
    </div>
  );
}

// The icon of an entry: who started a run, or what an operation did.
const icons: Record<string, IconName> = {
  user: "refresh",
  schedule: "clock",
  cli: "terminal",
  scan: "refresh",
  update: "update",
  rollback: "rollback",
};

function Entry({ entry: e, onOpen }: { entry: ActivityEntry; onOpen: (nodeId: string) => void }) {
  const { t } = useTranslation();
  const run = e.kind === "run";
  const title = run ? (e.path ? t(`activity.run_${e.by}`, { path: e.path }) : t(`activity.runAll_${e.by}`)) : "";
  const meta = [run ? t("activity.sites", { count: e.sites }) : "", outcome(e) ? t(`activity.${outcome(e)}`) : ""].filter(Boolean);
  const changes = e.changes ?? [];
  const sites = e.siteChanges ?? [];
  const notices = e.notices ?? [];
  return (
    <li className={styles.entry}>
      <span className={styles.time}>{clock(e.at)}</span>
      <span className={styles.icon}>
        <Icon name={icons[run ? e.by : e.kind] ?? "refresh"} size={16} />
      </span>
      <span className={styles.body}>
        <span className={styles.line}>
          {run ? (
            <span className={styles.what}>{title}</span>
          ) : (
            <span className={styles.what}>
              <Named text={t(`activity.${e.kind}`, { path: mark })} name={e.path} onOpen={() => onOpen(e.siteId)} />
            </span>
          )}
          {meta.length > 0 && <span className={styles.meta}>{meta.join(" · ")}</span>}
        </span>
        {e.summary && <span className={styles.meta}>{e.summary}</span>}
        {changes.map((c) => (
          <ChangeLine key={`${c.kind}-${c.nodeId}`} change={c} onOpen={onOpen} />
        ))}
        {/* What changed on each site since its previous complete scan (ADR-0027). */}
        {sites.map((s) => (
          <span key={s.siteId} className={styles.site}>
            <button type="button" className={styles.name} onClick={() => onOpen(s.siteId)}>
              {s.name}
            </button>
            <ChangeList changes={s.changes ?? []} label={t("activity.changesOf", { name: s.name })} />
          </span>
        ))}
        {noticeLines(notices, t).map((line) => (
          <span key={line} className={styles.notice}>
            <Icon name="bell" size={12} />
            {line}
          </span>
        ))}
      </span>
    </li>
  );
}

/**
 * The notifications a run sent, a line each: a site turned urgent or unreachable by its names; the
 * changes that notify (ADR-0027) by what they were, those of the same sites on one line; the end of
 * a long run as its notification said it.
 */
function noticeLines(notices: NoticeView[], t: TFunction): string[] {
  const lines: string[] = [];
  const changes = new Map<string, string[]>();
  for (const n of notices) {
    const names = (n.names ?? []).join(", ");
    // The end of a long run (ADR-0035): what its notification said.
    if (n.kind === "ended") {
      lines.push(t("activity.notifiedText", { text: n.text }));
    } else if (n.kind === "admin" || n.kind === "installed") {
      changes.set(names, [...(changes.get(names) ?? []), t(`activity.noticeKind.${n.kind}`)]);
    } else {
      lines.push(t("activity.notified", { names }));
    }
  }
  for (const [names, what] of changes) {
    lines.push(t("activity.notifiedChange", { what: what.join(t("activity.and")), names }));
  }
  return lines;
}

// Stands for a name in a sentence, which Named then shows as a link.
const mark = "\u0000";

/** A sentence whose name (in place of mark) opens the page of its site or host. */
function Named({ text, name, onOpen }: { text: string; name: string; onOpen: () => void }) {
  const [before, after] = text.split(mark);
  return (
    <span>
      {before}
      <button type="button" className={styles.name} onClick={onOpen}>
        {name}
      </button>
      {after}
    </span>
  );
}

// A change, its name opening the page of its site or host.
function ChangeLine({ change: c, onOpen }: { change: Change; onOpen: (nodeId: string) => void }) {
  const { t } = useTranslation();
  return (
    <span className={styles.change}>
      <StatusDot status={c.kind === "recovered" ? "ok" : "danger"} />
      <Named text={t(`activity.${c.kind}`, { name: mark })} name={c.name} onOpen={() => onOpen(c.nodeId)} />
    </span>
  );
}

// The outcome worth saying: one that is not a plain end.
function outcome(e: ActivityEntry): string {
  if (e.outcome === "completed") return "";
  return ["running", "cancelled", "failed", "interrupted"].includes(e.outcome) ? e.outcome : "";
}
