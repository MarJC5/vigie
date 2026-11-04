import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import type { CopyRunView, CopyView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sizeText } from "../../lib/data";
import { Button } from "../Button/Button";
import { ProgressBar } from "../Progress/Progress";
import { StatusDot } from "../StatusDot/StatusDot";
import styles from "./CopyList.module.css";

export interface CopyListProps {
  copies: CopyView[];
  /** The folder of the site's copies. */
  folder: string;
  /** The copy of the site running now, if one is. */
  run?: CopyRunView;
  onShow?: (copy: CopyView) => void;
  onResume?: (copy: CopyView) => void;
  onRemove?: (copy: CopyView) => void;
  onStop?: () => void;
}

/** The running copy is its folder's; a new copy says its folder once made: the newest one. */
function running(copies: CopyView[], run?: CopyRunView): CopyView | undefined {
  if (!run) return undefined;
  return run.folder ? copies.find((c) => c.folder === run.folder) : copies[0];
}

/** A site's copies on this computer (ADR-0041, mockups 28d to 28f): each with its archives, the
 one running archive by archive, an incomplete one with Resume. */
export function CopyList({ copies, folder, run, onShow, onResume, onRemove, onStop }: CopyListProps) {
  const { t, i18n } = useTranslation();
  const when = new Intl.DateTimeFormat(i18n.language, { dateStyle: "short", timeStyle: "short" });
  const size = (bytes: number) => sizeText(bytes, i18n.language);
  const [open, setOpen] = useState<string>();
  const current = running(copies, run);
  const contentName = (c: string) => t(`copies.content.${c}`);
  return (
    <div className={styles.copies}>
      <div className={styles.header}>
        <h2 className={styles.title}>{t("copies.title")}</h2>
        <span className={styles.folder}>{folder}</span>
      </div>
      <div className={styles.list}>
        {copies.length === 0 && !run && <p className={styles.none}>{t("copies.none")}</p>}
        {run && !current && (
          <div className={styles.row}>
            <span />
            <span>{t("copies.running")}</span>
            <span />
            <span />
            <span className={styles.state}>
              <StatusDot status="partial" label={t("copies.running")} withLabel />
            </span>
          </div>
        )}
        <ul className={styles.rows}>
          {copies.map((c) => {
            const isRunning = c === current;
            const expanded = isRunning || open === c.folder;
            const missing = c.planned - c.archives;
            return (
              <li key={c.folder} className={styles.copy} data-open={expanded || undefined}>
                <div className={styles.row}>
                  {isRunning || c.complete ? (
                    <span />
                  ) : (
                    <Button
                      variant="icon"
                      icon={expanded ? "caretDown" : "caretRight"}
                      label={t("copies.archivesOf", { done: c.archives, count: c.planned })}
                      aria-expanded={expanded}
                      onClick={() => setOpen(expanded ? undefined : c.folder)}
                    />
                  )}
                  <span className={styles.when}>{when.format(new Date(c.started))}</span>
                  <span className={styles.what}>
                    <span className={styles.content}>{contentName(c.content)}</span>
                    <span className={styles.archives}>
                      {c.complete ? t("copies.archives", { count: c.archives }) : t("copies.archivesOf", { done: isRunning && run ? run.done : c.archives, count: c.planned })}
                    </span>
                  </span>
                  <span className={styles.size}>{size(c.bytes)}</span>
                  <span className={styles.state}>
                    <StatusDot
                      status={isRunning ? "partial" : c.complete ? "ok" : "warn"}
                      label={isRunning ? t("copies.running") : c.complete ? t("copies.complete") : t("copies.incomplete")}
                      withLabel
                    />
                    {!isRunning && c.complete && onShow && (
                      <Button variant="outline" onClick={() => onShow(c)}>
                        {t("copies.show")}
                      </Button>
                    )}
                    {!isRunning && !c.complete && !expanded && onResume && (
                      <Button variant="primary" onClick={() => onResume(c)} disabled={!!run}>
                        {t("copies.resume")}
                      </Button>
                    )}
                    {!isRunning && !expanded && onRemove && (
                      <Button variant="icon" icon="trash" label={t("copies.remove")} title={t("copies.remove")} onClick={() => onRemove(c)} />
                    )}
                  </span>
                </div>
                {expanded && (
                  <Parts copy={c} run={isRunning ? run : undefined}>
                    <div className={styles.foot}>
                      <span className={styles.footText}>
                        {isRunning && run
                          ? t("copies.runningFoot", {
                              read: size(c.bytes + run.read),
                              total: size((c.parts ?? []).reduce((n, p) => n + p.bytes, 0)),
                              done: run.done,
                              count: run.total,
                            })
                          : t("copies.incompleteFoot", { count: missing })}
                      </span>
                      <span className={styles.footActions}>
                        {isRunning && onStop && <Button onClick={onStop}>{t("copies.stop")}</Button>}
                        {!isRunning && onRemove && (
                          <Button variant="icon" icon="trash" label={t("copies.remove")} title={t("copies.remove")} onClick={() => onRemove(c)} />
                        )}
                        {!isRunning && onResume && (
                          <Button variant="primary" onClick={() => onResume(c)} disabled={!!run}>
                            {t("copies.resume")}
                          </Button>
                        )}
                      </span>
                    </div>
                  </Parts>
                )}
              </li>
            );
          })}
        </ul>
        <p className={styles.note}>{t("copies.note")}</p>
      </div>
    </div>
  );
}

/** The archives of a copy: written, the one being read with its bar, waiting or missing. */
function Parts({ copy, run, children }: { copy: CopyView; run?: CopyRunView; children: React.ReactNode }) {
  const { t, i18n } = useTranslation();
  const size = (bytes: number) => sizeText(bytes, i18n.language);
  const label = useId();
  return (
    <div className={styles.parts}>
      <ul className={styles.partRows}>
        {(copy.parts ?? []).map((p) => {
          const reading = run !== undefined && run.archive === p.name;
          const status = p.written ? "ok" : reading ? "partial" : run ? "unknown" : "warn";
          const note = p.written
            ? t("copies.part.copied")
            : reading
              ? run.expected > 0
                ? t("copies.part.reading", { read: size(run.read), total: size(run.expected) })
                : t("copies.part.readingDump", { read: size(run.read) })
              : run
                ? t("copies.part.waiting")
                : t("copies.part.missing");
          return (
            <li key={p.name} className={styles.part}>
              <StatusDot status={status} label={note} />
              <span className={styles.partName}>{p.name}</span>
              <span className={styles.partNote}>
                {reading && run.expected > 0 && (
                  <span className={styles.bar}>
                    <ProgressBar labelledBy={`${label}-${p.name}`} value={Math.min(run.read, run.expected)} max={run.expected} />
                  </span>
                )}
                <span id={`${label}-${p.name}`}>{note}</span>
              </span>
              <span className={styles.size}>{p.written ? size(p.bytes) : ""}</span>
            </li>
          );
        })}
      </ul>
      {children}
    </div>
  );
}
