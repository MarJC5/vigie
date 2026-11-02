import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";

import type { BackupsView, FindingView, OperationView, Problem, ScanView, SiteView, TagView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sizeText } from "../../lib/data";
import { clock, day } from "../../lib/findings";
import { since } from "../../lib/time";
import { type Choice, choicesOf, namesOf, offeredKeys } from "../../lib/updates";
import { Button } from "../Button/Button";
import { ChangeList } from "../ChangeList/ChangeList";
import { DetailHeader } from "../DetailHeader/DetailHeader";
import { Dialog } from "../Dialog/Dialog";
import { ExtensionList } from "../ExtensionList/ExtensionList";
import { InfoAside, InfoList, InfoRow } from "../InfoList/InfoList";
import { NotesPanel } from "../NotesPanel/NotesPanel";
import { Notice } from "../Notice/Notice";
import { type QuickLink, QuickLinks } from "../QuickLinks/QuickLinks";
import { SecurityPanel } from "../SecurityPanel/SecurityPanel";
import { StatusDot } from "../StatusDot/StatusDot";
import { TabBar } from "../TabBar/TabBar";
import { TagRow } from "../TagRow/TagRow";
import { UpdateChoices } from "../UpdateChoices/UpdateChoices";
import { BackupList } from "../BackupList/BackupList";
import { UpdateHistory } from "../UpdateHistory/UpdateHistory";
import styles from "./SiteOverview.module.css";

export interface SiteOverviewProps {
  site: SiteView;
  scanning: boolean;
  onScan: () => void;
  /** The clock of "3 minutes ago"; the current time by default. */
  now?: Date;
  /** Opens the site's menu under the point given. */
  onMore?: (point: { x: number; y: number }) => void;
  /** The site is in an update or a rollback not over yet: it cannot be scanned or updated. */
  updating?: boolean;
  /** Opens the update of the items chosen; without it, the updates are only listed. */
  onUpdate?: (keys: Set<string>) => void;
  /** Opens the window of the update going on. */
  onFollow?: () => void;
  /** The updates and rollbacks of the site, the newest first; without it, no History tab. */
  history?: OperationView[];
  onRollback?: (operationID: string) => void;
  /** The site's backups on its server (ADR-0033), shown above its updates in the History tab. */
  backups?: BackupsView;
  /** Asks to remove the backups past the limits. */
  onRemoveBackups?: () => void;
  /** Asks to remove the site from Vigie, offered when its folder holds no WordPress any more. */
  onRemove?: () => void;
  /** Scans again, comparing the files even when it was done less than a day ago. */
  onCheckFiles?: () => void;
  /** Opens the dialog that sets a finding aside; without it, findings are only listed. */
  onIgnore?: (finding: FindingView) => void;
  onRestore?: (finding: FindingView) => void;
  /** Opens the dialog that ignores an update offered (ADR-0026); without it, no "Ignore…". */
  onIgnoreUpdate?: (choice: Choice) => void;
  onRestoreUpdate?: (choice: Choice) => void;
  /** Opens an address in the browser, or an ssh:// address in the user's terminal. */
  onOpen?: (href: string) => void;
  /** Puts a text on the clipboard; without it, no "Copy SSH command". */
  onCopy?: (text: string) => Promise<unknown> | void;
  /** The system opens ssh:// addresses in a terminal: "SSH terminal" is offered. */
  terminal?: boolean;
  /** The site's Markdown notes; without them, no Notes tab. */
  notes?: string;
  /** Saves the notes; without it, they are only read. */
  onSaveNotes?: (notes: string) => Promise<Problem | null>;
  /** The site's tags; without them, no tag row. */
  tags?: string[];
  /** The tags in use on every site, offered while one is added. */
  usedTags?: TagView[];
  /** Saves the site's tags; without it, they are only read. */
  onSaveTags?: (tags: string[]) => Promise<Problem | null>;
  /** Opens a tag's page. */
  onOpenTag?: (tag: string) => void;
  /** Opens a subsite's page, from the Plugins tab of a network (ADR-0029). */
  onOpenSubsite?: (blogId: number) => void;
}

/** A site with its latest scan: an overview, the updates waiting, and the history of its updates. */
export function SiteOverview({
  site,
  scanning,
  onScan,
  now,
  onMore,
  updating = false,
  onUpdate,
  onFollow,
  history,
  backups,
  onRemoveBackups,
  onRollback,
  onRemove,
  onCheckFiles,
  onIgnore,
  onRestore,
  onIgnoreUpdate,
  onRestoreUpdate,
  onOpen,
  onCopy,
  terminal = false,
  notes,
  onSaveNotes,
  tags,
  usedTags,
  onSaveTags,
  onOpenTag,
  onOpenSubsite,
}: SiteOverviewProps) {
  const { t, i18n } = useTranslation();
  const [tab, setTab] = useState("overview");
  const panel = useId();
  const scan = site.scan;
  const pending = scan ? scan.pending.core + scan.pending.plugins + scan.pending.themes + scan.pending.languages : 0;
  const updates = scan?.outcome === "completed" ? (scan.updates ?? []) : [];
  const groups = choicesOf(updates);
  const names = namesOf(updates);
  // The choice goes back to every item offered at each new scan.
  const scanKey = `${site.id}:${scan?.finished ?? ""}`;
  const [choice, setChoice] = useState<{ key: string; chosen: Set<string> }>();
  const chosen = choice?.key === scanKey ? choice.chosen : offeredKeys(groups);
  // An ignored update counts among those that can be chosen, unticked until it is.
  const offered = groups.flatMap((g) => g.choices).filter((c) => c.offered).length;
  const completed = scan?.outcome === "completed";
  const toLookAt = completed && scan ? scan.counts.urgent + scan.counts.attention : 0;

  // "Copied" for two seconds after the SSH command was copied.
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);
  // The quick links (ADR-0023): the site, its dashboard, a shell on its account.
  const links: QuickLink[] = [];
  if (onOpen && site.url) links.push({ label: t("site.openSite"), icon: "globe", onClick: () => onOpen(site.url) });
  // The admin address from the Go side: the user's, else the WordPress address followed by /wp-admin/.
  if (onOpen && site.adminUrl) links.push({ label: t("site.wpAdmin"), icon: "external", onClick: () => onOpen(site.adminUrl) });
  if (onOpen && terminal && site.sshUrl) links.push({ label: t("site.terminal"), icon: "terminal", onClick: () => onOpen(site.sshUrl) });
  if (onCopy && site.sshCommand) {
    links.push({
      label: copied ? t("site.copied") : t("site.copySsh"),
      icon: copied ? "check" : "copy",
      onClick: () => void Promise.resolve(onCopy(site.sshCommand)).then(() => setCopied(true)),
    });
  }

  return (
    <div className={styles.overview}>
      <div className={styles.header}>
        <DetailHeader
          title={site.name}
          meta={scan ? t("site.lastScan", { when: since(scan.finished, i18n.language, now) }) : undefined}
          links={
            <>
              <QuickLinks label={t("site.links")} links={links} />
              {tags && <TagRow tags={tags} used={usedTags ?? []} onSave={onSaveTags} onOpenTag={onOpenTag} />}
            </>
          }
        >
          {onMore && (
            <Button
              variant="icon"
              icon="dots"
              label={t("edit.more")}
              onClick={(event) => {
                const r = event.currentTarget.getBoundingClientRect();
                onMore({ x: r.left, y: r.bottom + 4 });
              }}
            />
          )}
        </DetailHeader>
        <TabBar
          label={t("tabs.label")}
          tabs={[
            { id: "overview", label: t("tabs.overview") },
            { id: "extensions", label: t("tabs.extensions") },
            { id: "updates", label: t("tabs.updates"), count: pending > 0 ? pending : undefined },
            { id: "security", label: t("tabs.security"), count: toLookAt > 0 ? toLookAt : undefined },
            ...(notes !== undefined ? [{ id: "notes", label: t("tabs.notes") }] : []),
            ...(history ? [{ id: "history", label: t("tabs.history") }] : []),
          ]}
          value={tab}
          onChange={setTab}
          panelId={panel}
        >
          {updating && onFollow ? (
            <Button variant="primary" icon="update" onClick={onFollow}>
              {t("update.follow")}
            </Button>
          ) : tab === "updates" && onUpdate && offered > 0 ? (
            <>
              <span className={styles.selected}>{t("update.selected", { count: chosen.size, total: offered })}</span>
              <Button variant="primary" icon="update" onClick={() => onUpdate(chosen)} disabled={chosen.size === 0 || updating || scanning} aria-busy={scanning || undefined}>
                {t("update.updateSelected", { count: chosen.size })}
              </Button>
            </>
          ) : (
            <Button variant="primary" icon="refresh" onClick={onScan} disabled={scanning || updating} aria-busy={scanning || undefined}>
              {scanning ? t("overview.scanning") : t("action.scanNow")}
            </Button>
          )}
        </TabBar>
      </div>
      <div role="tabpanel" id={panel} aria-labelledby={`${panel}-${tab}`} className={styles.panel}>
        {scan?.problem && tab !== "history" && (
          <ScanNotice code={scan.problem.code} detail={scan.problem.detail} onRemove={onRemove} />
        )}
        {scan?.attempt?.problem && tab !== "history" && (
          <AttemptNotice finished={scan.finished} problem={scan.attempt.problem} onRemove={onRemove} />
        )}
        {tab === "overview" && (site.changes ?? []).length > 0 && <SinceBefore site={site} />}
        {tab === "overview" && <Overview site={site} onExtensions={() => setTab("extensions")} />}
        {tab === "extensions" &&
          (completed && scan ? (
            <ExtensionList
              extensions={scan.extensions ?? []}
              onFinding={() => setTab("security")}
              onOpen={onOpen}
              subsites={site.subsites ?? []}
              onOpenSubsite={onOpenSubsite}
            />
          ) : (
            !scan && (
              <InfoList>
                <InfoRow label={t("tabs.extensions")}>{t("ext.notScanned")}</InfoRow>
              </InfoList>
            )
          ))}
        {tab === "updates" &&
          scan?.outcome === "completed" &&
          (updates.length === 0 ? (
            <InfoList>
              <InfoRow label={t("updates.label")}>{t("updates.none")}</InfoRow>
            </InfoList>
          ) : (
            <UpdateChoices
              groups={groups}
              chosen={chosen}
              onChange={(next) => setChoice({ key: scanKey, chosen: next })}
              names={names}
              onIgnore={onIgnoreUpdate}
              onRestore={onRestoreUpdate}
              onOpen={onOpen}
            />
          ))}
        {tab === "security" &&
          (completed && scan ? (
            <SecurityPanel
              scan={scan}
              scanning={scanning}
              onCheckFiles={updating ? undefined : onCheckFiles}
              onIgnore={onIgnore}
              onRestore={onRestore}
              onOpen={onOpen}
              subsites={site.subsites ?? []}
            />
          ) : (
            !scan && (
              <InfoList>
                <InfoRow label={t("tabs.security")}>{t("security.notScanned")}</InfoRow>
              </InfoList>
            )
          ))}
        {tab === "notes" && notes !== undefined && (
          <NotesPanel notes={notes} name={site.name} onSave={onSaveNotes} onOpen={onOpen} />
        )}
        {tab === "history" && history && (
          <div className={styles.history}>
            {backups && <BackupList backups={backups} operations={history} names={names} onRemove={onRemoveBackups} busy={updating} />}
            {backups && <h2 className={styles.historyTitle}>{t("backups.updatesTitle")}</h2>}
            <UpdateHistory operations={history} names={names} onRollback={(id) => onRollback?.(id)} busy={updating} />
          </div>
        )}
      </div>
    </div>
  );
}

function Overview({ site, onExtensions }: { site: SiteView; onExtensions: () => void }) {
  const { t } = useTranslation();
  const scan = site.scan;
  const unchecked = t("overview.unchecked");
  const completed = scan?.outcome === "completed";
  return (
    <InfoList>
      {site.multisite && <InfoRow label={t("detail.kind")}>{t("kind.network")}</InfoRow>}
      {!scan && <InfoRow label={t("detail.lastScan")}>{t("overview.neverScanned")}</InfoRow>}
      {completed && scan && (
        <>
          <InfoRow label={t("overview.wordpress")}>
            <StatusDot status={scan.pending.core > 0 || !scan.checked.core ? "warn" : "ok"} />
            {[
              scan.wordpress,
              !scan.checked.core ? unchecked : newestCore(scan) && t("overview.coreUpdate", { version: newestCore(scan) }),
            ]
              .filter(Boolean)
              .join(" · ")}
          </InfoRow>
          <InfoRow label={t("overview.phpCli")}>{scan.phpCli}</InfoRow>
          <PHPWebRow scan={scan} />
          <InfoRow label={t("overview.wpCli")}>{scan.wpCli}</InfoRow>
          {scan.adminEmail && (
            <InfoRow
              label={t("overview.adminEmail")}
              note={scan.adminEmailPending ? t("overview.adminEmailPending", { email: scan.adminEmailPending }) : undefined}
            >
              <span data-selectable>{scan.adminEmail}</span>
            </InfoRow>
          )}
          {complements(scan) && <DebugLogRow scan={scan} />}
          <InfoRow label={t("overview.plugins")}>
            <StatusDot status={scan.pending.plugins > 0 || !scan.checked.plugins ? "warn" : "ok"} />
            <span className={styles.unknownLine}>
              {installed(t, scan.plugins, scan.pending.plugins, scan.checked.plugins)}
              <Button variant="text" label={t("overview.showPlugins")} onClick={onExtensions}>
                {t("overview.show")}
              </Button>
            </span>
          </InfoRow>
          <InfoRow label={t("overview.themes")}>
            <StatusDot status={scan.pending.themes > 0 || !scan.checked.themes ? "warn" : "ok"} />
            <span className={styles.unknownLine}>
              {installed(t, scan.themes, scan.pending.themes, scan.checked.themes)}
              <Button variant="text" label={t("overview.showThemes")} onClick={onExtensions}>
                {t("overview.show")}
              </Button>
            </span>
          </InfoRow>
          <InfoRow label={t("overview.translations")}>
            <StatusDot status={scan.pending.languages > 0 || !scan.checked.languages ? "warn" : "ok"} />
            {!scan.checked.languages
              ? unchecked
              : scan.pending.languages > 0
                ? t("overview.updates", { count: scan.pending.languages })
                : t("overview.upToDate")}
          </InfoRow>
          {(scan.sourceUnknown ?? []).length > 0 && (
            <InfoRow label={t("overview.sourceUnknown")}>
              <SourceUnknown names={scan.sourceUnknown ?? []} />
            </InfoRow>
          )}
        </>
      )}
      {(scan?.home || site.url) && (
        <InfoRow label={t("overview.address")}>
          <span data-selectable>{scan?.home || site.url}</span>
        </InfoRow>
      )}
      <InfoRow label={t("overview.path")}>
        <span data-selectable>{site.path}</span>
      </InfoRow>
    </InfoList>
  );
}

/** Whether the scan read its complements (ADR-0028): a scan made before them, or whose settings
 could not be read, says nothing of them. */
function complements(scan: ScanView): boolean {
  return scan.adminEmail !== "" || scan.debugLog !== null || scan.phpWeb.version !== "" || scan.phpWeb.unknown !== "";
}

/** The PHP serving the pages, as the dashboard noted it, or why it is not known. */
function PHPWebRow({ scan }: { scan: ScanView }) {
  const { t } = useTranslation();
  const web = scan.phpWeb;
  const label = t("overview.phpWeb");
  if (web.version) {
    return (
      <InfoRow label={label} note={t("overview.phpWebChecks")}>
        {web.version}
        {web.seen && <InfoAside>{t("overview.phpWebSeen", { day: day(web.seen) })}</InfoAside>}
      </InfoRow>
    );
  }
  const why: Record<string, [string, string]> = {
    "not-noted": ["overview.phpWebNotKnown", "overview.phpWebNotNoted"],
    "object-cache": ["overview.phpWebNotKnown", "overview.phpWebObjectCache"],
    unmatched: ["overview.phpWebDiffers", "overview.phpWebUnmatched"],
  };
  const [value, note] = why[web.unknown] ?? [];
  if (!value || !note) return null;
  return (
    <InfoRow label={label} note={t(note)}>
      <span className={styles.missing}>{t(value)}</span>
    </InfoRow>
  );
}

/** debug.log: its size and whether WordPress writes it, or that there is none. */
function DebugLogRow({ scan }: { scan: ScanView }) {
  const { t, i18n } = useTranslation();
  const log = scan.debugLog;
  const label = t("overview.debugLog");
  if (!log) {
    return (
      <InfoRow label={label}>
        <span className={styles.missing}>{t("overview.debugLogNone")}</span>
        <InfoAside>{t("overview.debugLogNoneNote")}</InfoAside>
      </InfoRow>
    );
  }
  return (
    <InfoRow label={label}>
      {log.size > 0 ? sizeText(log.size, i18n.language) : t("overview.debugLogEmpty")}
      <InfoAside>{log.written ? t("overview.debugLogWritten", { path: log.path }) : t("overview.debugLogLeft")}</InfoAside>
    </InfoRow>
  );
}

/** What the latest scan found changed since the complete scan before (ADR-0027). */
function SinceBefore({ site }: { site: SiteView }) {
  const { t } = useTranslation();
  return (
    <section className={styles.since}>
      <h2 className={styles.sinceTitle}>
        {t("changes.since")}
        <span className={styles.sinceWhen}>{t("changes.sinceWhen", { day: day(site.changesSince), time: clock(site.changesSince) })}</span>
      </h2>
      <div className={styles.sinceBox}>
        <ChangeList changes={site.changes ?? []} label={t("changes.since")} />
      </div>
    </section>
  );
}

/** The plugins whose updates Vigie cannot see: their count on one line, the list in a dialog, which
 * stays readable however many there are. */
function SourceUnknown({ names }: { names: string[] }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  return (
    <span className={styles.unknownLine}>
      {t("overview.sourceUnknownCount", { count: names.length })}
      <Button variant="text" aria-haspopup="dialog" onClick={() => setOpen(true)}>
        {t("overview.show")}
      </Button>
      {open && (
        <Dialog
          title={t("overview.sourceUnknown")}
          lead={t("overview.sourceUnknownHint")}
          onClose={() => setOpen(false)}
          actions={
            <Button variant="primary" onClick={() => setOpen(false)}>
              {t("overview.close")}
            </Button>
          }
        >
          <ul className={styles.unknownList}>
            {names.map((name) => (
              <li key={name}>{name}</li>
            ))}
          </ul>
        </Dialog>
      )}
    </span>
  );
}

type T = (key: string, vars?: Record<string, unknown>) => string;

function installed(t: T, count: number, updates: number, checked: boolean): string {
  if (!checked) return `${count} · ${t("overview.unchecked")}`;
  if (updates > 0) return t("overview.installedUpdates", { installed: count, count: updates });
  return t("overview.installedUpToDate", { installed: count });
}

// The highest WordPress release offered, if any.
function newestCore(scan: ScanView): string {
  return (scan.updates ?? [])
    .filter((u) => u.kind === "core" && u.update === "available")
    .map((u) => u.updateVersion)
    .sort((a, b) => a.localeCompare(b, undefined, { numeric: true }))
    .at(-1) ?? "";
}

/** The latest scan did not complete, the results shown are from the one before: why, and since when.
 A site with no WordPress left at its path can be removed from Vigie at once. */
function AttemptNotice({ finished, problem, onRemove }: { finished: string; problem: { code: string; detail: string }; onRemove?: () => void }) {
  const { t } = useTranslation();
  const reason = t(problem.code, { message: problem.detail });
  const gone = problem.code === "outcome.notWordPress";
  return (
    <Notice
      title={t("overview.attemptTitle")}
      // Some reasons end with a full stop, others not: one only.
      text={t("overview.attemptText", { reason: reason.replace(/\.$/, ""), date: day(finished), time: clock(finished) })}
      detail={problem.detail && !reason.includes(problem.detail) ? problem.detail : undefined}
      tone="warn"
      action={gone && onRemove ? { label: t("overview.remove"), onClick: onRemove } : undefined}
    />
  );
}

/** Why the last scan did not complete; the raw message only when the text does not already say it.
 A site with no WordPress left at its path can be removed from Vigie at once. */
function ScanNotice({ code, detail, onRemove }: { code: string; detail: string; onRemove?: () => void }) {
  const { t } = useTranslation();
  const text = t(code, { message: detail });
  const gone = code === "outcome.notWordPress";
  return (
    <Notice
      title={t("overview.scanProblem")}
      text={text}
      detail={detail && !text.includes(detail) ? detail : undefined}
      tone={gone ? "warn" : "danger"}
      action={gone && onRemove ? { label: t("overview.remove"), onClick: onRemove } : undefined}
    />
  );
}
