import { type ReactNode, useId } from "react";
import { useTranslation } from "react-i18next";

import type { BackgroundSettings, DatabaseView, LoginItemState, LookupsView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sizeText } from "../../lib/data";
import { clock, day, keyOf } from "../../lib/findings";
import { Button } from "../Button/Button";
import { SelectField } from "../SelectField/SelectField";
import { Switch } from "../Switch/Switch";
import { TabBar } from "../TabBar/TabBar";
import { ExternalLink } from "../ExternalLink/ExternalLink";
import styles from "./SettingsPage.module.css";

export type SettingsTab = "general" | "lookups" | "data" | "about";

/** The theme of the window: the system's, or one chosen. */
export type Theme = "system" | "light" | "dark";

/** Where an import reads from. */
export type ImportSource = "vigie" | "ssh" | "csv";

export interface SettingsPageProps {
  tab: SettingsTab;
  onTab: (tab: SettingsTab) => void;
  language: string;
  onLanguage: (lang: string) => void;
  theme: Theme;
  onTheme: (theme: Theme) => void;
  lookups: LookupsView;
  onAgree: () => void;
  onWithdraw: () => void;
  onDestination: (id: string, on: boolean) => void;
  /** Vigie's version, for the About tab. */
  version: string;
  /** Opens an address in the browser; links open in a new window without it. */
  onOpen?: (href: string) => void;
  /** The Data tab: export the site manager, import from a source. */
  onExport: () => void;
  onImport: (source: ImportSource) => void;
  /** What the last export, import or compaction did, under its row. */
  dataNote?: { row: "export" | "import" | "compact"; text: string };
  /** The database's size and the space free in it; not shown before it is read. */
  database?: DatabaseView;
  /** The database is being compacted. */
  compacting?: boolean;
  /** Scans or updates are running: the database is compacted after them. */
  busy?: boolean;
  onCompact: () => void;
  /** The days the backups of an update stay on the server, 0 for no limit (ADR-0033). */
  backupDays?: number;
  onBackupDays?: (days: number) => void;
  /** Vigie in the background: kept running, automatic scans, notifications. */
  background: BackgroundSettings;
  onBackground: (b: BackgroundSettings) => void;
  /** Start at login: the system's state; not offered where the system has none. */
  loginItem: LoginItemState;
  onLoginItem: (on: boolean) => void;
}

// The intervals of automatic scans, in hours; 0 is never (app.AutoScanHours).
const autoScanHours = [0, 1, 3, 6, 12, 24];

// An address, not words to translate.
const wpvulnerability = { href: "https://www.wpvulnerability.com/", name: "wpvulnerability.com" };

/** The settings, in tabs: each row says what it does on the left and holds its control on the right. */
export function SettingsPage({
  tab,
  onTab,
  language,
  onLanguage,
  theme,
  onTheme,
  lookups,
  onAgree,
  onWithdraw,
  onDestination,
  version,
  onOpen,
  onExport,
  onImport,
  dataNote,
  database,
  compacting = false,
  busy = false,
  onCompact,
  backupDays = 30,
  onBackupDays,
  background,
  onBackground,
  loginItem,
  onLoginItem,
}: SettingsPageProps) {
  const { t, i18n } = useTranslation();
  const panel = useId();
  const size = (bytes: number) => sizeText(bytes, i18n.language);
  const tabs: SettingsTab[] = ["general", "lookups", "data", "about"];
  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}>{t("settings.title")}</h1>
        <TabBar label={t("settings.title")} tabs={tabs.map((id) => ({ id, label: t(`settings.tabs.${id}`) }))} value={tab} onChange={(id) => onTab(id as SettingsTab)} panelId={panel} />
      </div>
      <div role="tabpanel" id={panel} aria-labelledby={`${panel}-${tab}`} className={styles.panel}>
        {tab === "general" && (
          <Section>
            <Row title={t("settings.language.title")} desc={t("settings.language.desc")}>
              <SelectField
                label={t("settings.language.title")}
                hideLabel
                value={language}
                options={["en", "fr"].map((l) => ({ value: l, label: t(`settings.language.${l}`) }))}
                onChange={onLanguage}
              />
            </Row>
            <Row title={t("settings.theme.title")} desc={t("settings.theme.desc")}>
              <SelectField
                label={t("settings.theme.title")}
                hideLabel
                value={theme}
                options={(["system", "light", "dark"] as const).map((v) => ({ value: v, label: t(`settings.theme.${v}`) }))}
                onChange={(v) => onTheme(v as Theme)}
              />
            </Row>
          </Section>
        )}
        {tab === "general" && (
          <Section title={t("settings.background.section")}>
            <Row title={t("settings.background.onTitle")} desc={t("settings.background.onDesc")}>
              <Switch label={t("settings.background.onTitle")} on={background.on} onChange={(on) => onBackground({ ...background, on })} />
            </Row>
            {loginItem.supported && (
              <Row
                title={t("settings.background.loginTitle")}
                desc={t("settings.background.loginDesc")}
                note={background.on ? undefined : t("settings.background.loginNeedsBackground")}
              >
                <Switch
                  label={t("settings.background.loginTitle")}
                  on={background.on && loginItem.on}
                  disabled={!background.on}
                  onChange={onLoginItem}
                />
              </Row>
            )}
            <Row title={t("settings.background.autoTitle")} desc={t("settings.background.autoDesc")}>
              <SelectField
                label={t("settings.background.autoTitle")}
                hideLabel
                value={String(background.autoScan)}
                options={autoScanHours.map((h) => ({
                  value: String(h),
                  label: h === 0 ? t("settings.background.never") : h === 24 ? t("settings.background.daily") : t("settings.background.every", { count: h }),
                }))}
                onChange={(v) => onBackground({ ...background, autoScan: Number(v) })}
              />
            </Row>
            <Row title={t("settings.background.notifyTitle")} desc={t("settings.background.notifyDesc")}>
              <Switch label={t("settings.background.notifyTitle")} on={background.notify} onChange={(notify) => onBackground({ ...background, notify })} />
            </Row>
          </Section>
        )}
        {tab === "lookups" && <Lookups lookups={lookups} onAgree={onAgree} onWithdraw={onWithdraw} onDestination={onDestination} onOpen={onOpen} />}
        {tab === "data" && (
          <>
            <Section title={t("data.backup")}>
              <Row title={t("data.exportTitle")} desc={t("data.exportDesc")} note={dataNote?.row === "export" ? dataNote.text : undefined}>
                <Button variant="primary" onClick={onExport}>
                  {t("data.exportAction")}
                </Button>
              </Row>
            </Section>
            <Section title={t("data.servers")}>
              <Row title={t("data.keepTitle")} desc={t("data.keepDesc")}>
                <SelectField
                  label={t("data.keepTitle")}
                  hideLabel
                  value={String(backupDays)}
                  options={["7", "30", "90", "0"].map((d) => ({ value: d, label: t(`data.keep.${d}`) }))}
                  onChange={(d) => onBackupDays?.(Number(d))}
                />
              </Row>
            </Section>
            <Section title={t("data.base")}>
              <Row title={t("data.keptTitle")} desc={t("data.keptDesc")} />
              <Row
                title={t("data.compactTitle")}
                desc={t("data.compactDesc")}
                meta={
                  database && !database.problem
                    ? database.free > 0
                      ? t("data.size", { size: size(database.bytes), free: size(database.free) })
                      : t("data.sizeOnly", { size: size(database.bytes) })
                    : undefined
                }
                note={dataNote?.row === "compact" ? dataNote.text : undefined}
              >
                <Button onClick={onCompact} disabled={compacting || busy}>
                  {t(compacting ? "data.compacting" : "data.compactAction")}
                </Button>
              </Row>
            </Section>
            <Section title={t("data.import")}>
              {(["vigie", "ssh", "csv"] as const).map((source, i) => (
                <Row
                  key={source}
                  title={t(`data.${source}Title`)}
                  desc={t(`data.${source}Desc`)}
                  note={i === 0 && dataNote?.row === "import" ? dataNote.text : undefined}
                >
                  <Button onClick={() => onImport(source)}>{t(source === "ssh" ? "data.sshAction" : "data.importAction")}</Button>
                </Row>
              ))}
            </Section>
          </>
        )}
        {tab === "about" && (
          <Section>
            <Row title={t("settings.version")} desc={t("settings.versionDesc", { version })} />
            <Row title={t("settings.license")} desc={t("settings.licenseDesc")} />
            <Row title={t("settings.sourceTitle")} desc={t("settings.sourceDesc")}>
              <ExternalLink href={wpvulnerability.href} onOpen={onOpen}>
                {wpvulnerability.name}
              </ExternalLink>
            </Row>
          </Section>
        )}
      </div>
    </div>
  );
}

function Lookups({
  lookups,
  onAgree,
  onWithdraw,
  onDestination,
  onOpen,
}: Pick<SettingsPageProps, "lookups" | "onAgree" | "onWithdraw" | "onDestination" | "onOpen">) {
  const { t } = useTranslation();
  const on = new Map((lookups.destinations ?? []).map((d) => [d.id, d.on]));
  const rows = (lookups.destinations ?? []).flatMap((d) => (d.id === "registry" ? ["iana", d.id] : [d.id]));
  const agreed = lookups.agreed;
  return (
    <>
      <p className={styles.lead}>{t("settings.lookupsLead")}</p>
      <Section title={t("settings.services")}>
        {rows.map((id) => {
          const key = `lookups.destination.${keyOf(id)}`;
          const name = t(`${key}.name`);
          const active = id === "iana" ? on.get("registry") : on.get(id);
          return (
            <Row
              key={id}
              title={name}
              desc={t("settings.receives", { sent: t(`${key}.sent`), kept: t(`${key}.kept`) })}
              note={!active && id !== "iana" ? t(`lookups.off.${keyOf(id)}`) : undefined}
            >
              {id !== "iana" && <Switch label={name} on={!!active} onChange={(next) => onDestination(id, next)} />}
            </Row>
          );
        })}
      </Section>
      <Section title={t("lookups.neverTitle")}>
        <Row title={t("settings.neverRow")} desc={t("lookups.neverText")} />
      </Section>
      <Section title={t("settings.consentSection")}>
        <Row
          title={t("settings.consentTitle")}
          desc={agreed ? t("settings.agreedDesc", { date: day(agreed), time: clock(agreed) }) : t("settings.notAgreedDesc")}
        >
          {agreed ? <Button onClick={onWithdraw}>{t("settings.withdraw")}</Button> : <Button variant="primary" onClick={onAgree}>{t("settings.agree")}</Button>}
        </Row>
        <Row title={t("settings.sourceTitle")} desc={t("settings.sourceDesc")}>
          <ExternalLink href={wpvulnerability.href} onOpen={onOpen}>
            {wpvulnerability.name}
          </ExternalLink>
        </Row>
      </Section>
    </>
  );
}

function Section({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <div className={styles.section}>
      {title && <h2 className={styles.sectionTitle}>{title}</h2>}
      {children}
    </div>
  );
}

function Row({ title, desc, meta, note, children }: { title: string; desc: string; meta?: string; note?: string; children?: ReactNode }) {
  return (
    <div className={styles.row}>
      <span className={styles.text}>
        <span className={styles.rowTitle}>{title}</span>
        <span className={styles.desc}>{desc}</span>
        {meta && <span className={styles.meta}>{meta}</span>}
        {note && <span className={styles.note}>{note}</span>}
      </span>
      <span className={styles.control}>{children}</span>
    </div>
  );
}
