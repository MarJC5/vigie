import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import type { FindingView, OperationView, ScanView, SiteView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { clock, day } from "../../lib/findings";
import { since } from "../../lib/time";
import { choicesOf, namesOf, offeredKeys } from "../../lib/updates";
import { Button } from "../Button/Button";
import { DetailHeader } from "../DetailHeader/DetailHeader";
import { InfoList, InfoRow } from "../InfoList/InfoList";
import { Notice } from "../Notice/Notice";
import { SecurityPanel } from "../SecurityPanel/SecurityPanel";
import { StatusDot } from "../StatusDot/StatusDot";
import { TabBar } from "../TabBar/TabBar";
import { UpdateChoices } from "../UpdateChoices/UpdateChoices";
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
  /** Asks to remove the site from Vigie, offered when its folder holds no WordPress any more. */
  onRemove?: () => void;
  /** Scans again, comparing the files even when it was done less than a day ago. */
  onCheckFiles?: () => void;
  /** Opens the dialog that sets a finding aside; without it, findings are only listed. */
  onIgnore?: (finding: FindingView) => void;
  onRestore?: (finding: FindingView) => void;
  /** Opens an address in the browser. */
  onOpen?: (href: string) => void;
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
  onRollback,
  onRemove,
  onCheckFiles,
  onIgnore,
  onRestore,
  onOpen,
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
  const offered = offeredKeys(groups).size;
  const completed = scan?.outcome === "completed";
  const toLookAt = completed && scan ? scan.counts.urgent + scan.counts.attention : 0;

  return (
    <div className={styles.overview}>
      <DetailHeader
        title={site.name}
        meta={scan ? t("site.lastScan", { when: since(scan.finished, i18n.language, now) }) : undefined}
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
          { id: "updates", label: t("tabs.updates"), count: pending > 0 ? pending : undefined },
          { id: "security", label: t("tabs.security"), count: toLookAt > 0 ? toLookAt : undefined },
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
      <div role="tabpanel" id={panel} aria-labelledby={`${panel}-${tab}`} className={styles.panel}>
        {scan?.problem && tab !== "history" && (
          <ScanNotice code={scan.problem.code} detail={scan.problem.detail} onRemove={onRemove} />
        )}
        {scan?.attempt?.problem && tab !== "history" && (
          <AttemptNotice finished={scan.finished} problem={scan.attempt.problem} onRemove={onRemove} />
        )}
        {tab === "overview" && <Overview site={site} />}
        {tab === "updates" &&
          scan?.outcome === "completed" &&
          (updates.length === 0 ? (
            <InfoList>
              <InfoRow label={t("updates.label")}>{t("updates.none")}</InfoRow>
            </InfoList>
          ) : (
            <UpdateChoices groups={groups} chosen={chosen} onChange={(next) => setChoice({ key: scanKey, chosen: next })} names={names} />
          ))}
        {tab === "security" &&
          (completed && scan ? (
            <SecurityPanel scan={scan} scanning={scanning} onCheckFiles={updating ? undefined : onCheckFiles} onIgnore={onIgnore} onRestore={onRestore} onOpen={onOpen} />
          ) : (
            !scan && (
              <InfoList>
                <InfoRow label={t("tabs.security")}>{t("security.notScanned")}</InfoRow>
              </InfoList>
            )
          ))}
        {tab === "history" && history && (
          <UpdateHistory operations={history} names={names} onRollback={(id) => onRollback?.(id)} busy={updating} />
        )}
      </div>
    </div>
  );
}

function Overview({ site }: { site: SiteView }) {
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
          <InfoRow label={t("overview.wpCli")}>{scan.wpCli}</InfoRow>
          <InfoRow label={t("overview.plugins")}>
            <StatusDot status={scan.pending.plugins > 0 || !scan.checked.plugins ? "warn" : "ok"} />
            {installed(t, scan.plugins, scan.pending.plugins, scan.checked.plugins)}
          </InfoRow>
          <InfoRow label={t("overview.themes")}>
            <StatusDot status={scan.pending.themes > 0 || !scan.checked.themes ? "warn" : "ok"} />
            {installed(t, scan.themes, scan.pending.themes, scan.checked.themes)}
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

/** The plugins whose updates Vigie cannot see: their count on one line, the list on demand. */
function SourceUnknown({ names }: { names: string[] }) {
  const { t } = useTranslation();
  const [open, setOpen] = useState(false);
  const list = useId();
  return (
    <span className={styles.unknown}>
      <span className={styles.unknownLine}>
        {t("overview.sourceUnknownCount", { count: names.length })}
        <Button variant="text" aria-expanded={open} aria-controls={list} onClick={() => setOpen(!open)}>
          {open ? t("overview.hide") : t("overview.show")}
        </Button>
      </span>
      {open && (
        <span id={list} className={styles.unknownList}>
          <span>{names.join(", ")}</span>
          <span>{t("overview.sourceUnknownHint")}</span>
        </span>
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
