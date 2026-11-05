import { useMemo } from "react";
import { useTranslation } from "react-i18next";

import type { FindingView, ScanView, SubsiteView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { clock, day, findingText, type Group, groupOf, groupsOf, ignoreText } from "../../lib/findings";
import { labelsOf } from "../../lib/subsites";
import { Button } from "../Button/Button";
import { Icon } from "../Icon/Icon";
import { LinkButton } from "../LinkButton/LinkButton";
import { type Status, StatusDot } from "../StatusDot/StatusDot";
import { ExternalLink } from "../ExternalLink/ExternalLink";
import styles from "./SecurityPanel.module.css";

export interface SecurityPanelProps {
  scan: ScanView;
  /** Compares the files again within the day; without it, no button. */
  onCheckFiles?: () => void;
  /** A scan of the site is running. */
  scanning?: boolean;
  /** Opens the dialog that sets a finding aside; without it, findings are only listed. */
  onIgnore?: (finding: FindingView) => void;
  onRestore?: (finding: FindingView) => void;
  /** Opens an address in the browser; links open in a new window without it. */
  onOpen?: (href: string) => void;
  /** A network's subsites, to name those its administrators administer (ADR-0029). */
  subsites?: SubsiteView[];
}

// The shape of each group: never color alone.
const shapes: Partial<Record<Group, Status>> = { urgent: "danger", attention: "warn", "not-checked": "partial" };

// A name, not words to translate.
const wpvulnerability = { href: "https://www.wpvulnerability.com/", name: "WPVulnerability" };

/** The Security tab of a site: what its latest scan found, the worst first, by group. */
export function SecurityPanel({ scan, onCheckFiles, scanning = false, onIgnore, onRestore, onOpen, subsites }: SecurityPanelProps) {
  const { t, i18n } = useTranslation();
  // Once for every row: a network can have thousands of subsites and of administrators.
  const labels = useMemo(() => labelsOf(subsites), [subsites]);
  const c = scan.counts;
  const summary = [
    c.urgent > 0 && t("security.summary.urgent", { count: c.urgent }),
    c.attention > 0 && t("security.summary.attention", { count: c.attention }),
    c.notChecked > 0 && t("security.summary.notChecked", { count: c.notChecked }),
    c.ignored > 0 && t("security.summary.ignored", { count: c.ignored }),
  ].filter(Boolean);
  const findings = scan.findings ?? [];

  return (
    <div className={styles.panel}>
      <div className={styles.bar}>
        <span>{summary.length > 0 ? summary.join(" · ") : t("security.summary.none")}</span>
        <span className={styles.files}>
          <span className={styles.filesText}>
            <Icon name="shield" size="sm" />
            {scan.filesChecked
              ? t("security.filesChecked", { date: day(scan.filesChecked), time: clock(scan.filesChecked) })
              : t("security.filesNever")}
          </span>
          {onCheckFiles && (
            <Button onClick={onCheckFiles} disabled={scanning} aria-busy={scanning || undefined}>
              {scanning ? t("security.checkingFiles") : t("security.checkFiles")}
            </Button>
          )}
        </span>
      </div>

      {groupsOf(findings).map(({ group, items }) => (
        // Headed groups, not landmarks: the window has several sites' worth of them in stories.
        <div key={group} className={styles.group}>
          <h2 className={styles.heading}>
            {t(`security.group.${group}`)}
            <span className={styles.count}>{items.length}</span>
          </h2>
          <ul className={styles.list}>
            {items.map((f) => (
              <Row key={f.key} finding={f} group={group} onIgnore={onIgnore} onRestore={onRestore} onOpen={onOpen} lang={i18n.language} labels={labels} />
            ))}
          </ul>
        </div>
      ))}

      <p className={styles.credit}>
        {t("security.creditBefore")}{" "}
        <ExternalLink href={wpvulnerability.href} onOpen={onOpen} size="inline">
          {wpvulnerability.name}
        </ExternalLink>
        {t("security.creditAfter")}
      </p>
    </div>
  );
}

/** Findings in the rows of the Security tab, without its groups: a subsite's administrators. */
export function FindingRows({ findings, label, subsites }: { findings: FindingView[]; label: string; subsites?: SubsiteView[] }) {
  const { i18n } = useTranslation();
  const labels = useMemo(() => labelsOf(subsites), [subsites]);
  return (
    <ul className={styles.list} aria-label={label}>
      {findings.map((f) => (
        <Row key={f.key} finding={f} group={groupOf(f)} lang={i18n.language} labels={labels} />
      ))}
    </ul>
  );
}

function Row({
  finding,
  group,
  onIgnore,
  onRestore,
  onOpen,
  lang,
  labels,
}: {
  finding: FindingView;
  group: Group;
  onIgnore?: (f: FindingView) => void;
  onRestore?: (f: FindingView) => void;
  onOpen?: (href: string) => void;
  lang: string;
  labels?: Record<string, string>;
}) {
  const { t } = useTranslation();
  const text = findingText(finding, t, lang, labels);
  const shape = shapes[group];
  // Information is only shown: it never counts, so there is nothing to set aside.
  const canIgnore = onIgnore && !finding.ignore && finding.level !== "info";
  return (
    <li className={styles.row} data-ignored={finding.ignore ? true : undefined}>
      <span className={styles.mark}>
        {shape ? (
          <StatusDot status={shape} label={t(`security.level.${group}`)} />
        ) : (
          <Icon name={group === "ignored" ? "eyeSlash" : "ok"} label={t(`security.level.${group}`)} />
        )}
      </span>
      <span className={styles.text}>
        <span className={styles.title} data-selectable>
          {text.title}
        </span>
        {text.detail && (
          <span className={styles.detail} data-selectable>
            {text.detail}
          </span>
        )}
        {text.quote && (
          <code className={styles.quote} data-selectable>
            {text.quote}
          </code>
        )}
        {finding.ignore && <span className={styles.detail}>{ignoreText(finding, t)}</span>}
      </span>
      <span className={styles.linkCell}>
        {text.link && (
          <ExternalLink href={text.link.href} onOpen={onOpen}>
            {text.link.label}
          </ExternalLink>
        )}
      </span>
      <span className={styles.action}>
        {canIgnore && (
          <LinkButton icon="eyeSlash" label={t("security.ignoreNamed", { title: text.title })} onClick={() => onIgnore(finding)}>
            {t("security.ignore")}
          </LinkButton>
        )}
        {onRestore && finding.ignore && (
          <LinkButton label={t("security.restoreNamed", { title: text.title })} onClick={() => onRestore(finding)}>
            {t("security.restore")}
          </LinkButton>
        )}
      </span>
    </li>
  );
}

/** A link out of the window: the browser opens it. */
