import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import type { OperationView, ScanView, SiteView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { since } from "../../lib/time";
import { choicesOf, namesOf, offeredKeys } from "../../lib/updates";
import { Button } from "../Button/Button";
import { DetailHeader } from "../DetailHeader/DetailHeader";
import { InfoList, InfoRow } from "../InfoList/InfoList";
import { Notice } from "../Notice/Notice";
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
            <Button variant="primary" icon="update" onClick={() => onUpdate(chosen)} disabled={chosen.size === 0 || updating}>
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
        {scan?.problem && tab !== "history" && <ScanNotice code={scan.problem.code} detail={scan.problem.detail} />}
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
              <span title={t("overview.sourceUnknownHint")}>{(scan.sourceUnknown ?? []).join(", ")}</span>
            </InfoRow>
          )}
        </>
      )}
      {(scan?.home || site.url) && <InfoRow label={t("overview.address")}>{scan?.home || site.url}</InfoRow>}
      <InfoRow label={t("overview.path")}>{site.path}</InfoRow>
    </InfoList>
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

/** Why the last scan did not complete; the raw message only when the text does not already say it. */
function ScanNotice({ code, detail }: { code: string; detail: string }) {
  const { t } = useTranslation();
  const text = t(code, { message: detail });
  return <Notice title={t("overview.scanProblem")} text={text} detail={detail && !text.includes(detail) ? detail : undefined} />;
}
