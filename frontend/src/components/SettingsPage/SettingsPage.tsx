import { type ReactNode, useId } from "react";
import { useTranslation } from "react-i18next";

import type { LookupsView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { clock, day, keyOf } from "../../lib/findings";
import { Button } from "../Button/Button";
import { Icon } from "../Icon/Icon";
import { SelectField } from "../SelectField/SelectField";
import { Switch } from "../Switch/Switch";
import { TabBar } from "../TabBar/TabBar";
import styles from "./SettingsPage.module.css";

export type SettingsTab = "general" | "lookups" | "data" | "about";

/** Where an import reads from. */
export type ImportSource = "vigie" | "ssh" | "csv";

export interface SettingsPageProps {
  tab: SettingsTab;
  onTab: (tab: SettingsTab) => void;
  language: string;
  onLanguage: (lang: string) => void;
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
  /** What the last export or import did, under its row. */
  dataNote?: { row: "export" | "import"; text: string };
}

// An address, not words to translate.
const wpvulnerability = { href: "https://www.wpvulnerability.com/", name: "wpvulnerability.com" };

/** The settings, in tabs: each row says what it does on the left and holds its control on the right. */
export function SettingsPage({
  tab,
  onTab,
  language,
  onLanguage,
  lookups,
  onAgree,
  onWithdraw,
  onDestination,
  version,
  onOpen,
  onExport,
  onImport,
  dataNote,
}: SettingsPageProps) {
  const { t } = useTranslation();
  const panel = useId();
  const tabs: SettingsTab[] = ["general", "lookups", "data", "about"];
  return (
    <div className={styles.page}>
      <h1 className={styles.title}>{t("settings.title")}</h1>
      <TabBar label={t("settings.title")} tabs={tabs.map((id) => ({ id, label: t(`settings.tabs.${id}`) }))} value={tab} onChange={(id) => onTab(id as SettingsTab)} panelId={panel} />
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
              <Link href={wpvulnerability.href} onOpen={onOpen}>
                {wpvulnerability.name}
              </Link>
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
          <Link href={wpvulnerability.href} onOpen={onOpen}>
            {wpvulnerability.name}
          </Link>
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

function Row({ title, desc, note, children }: { title: string; desc: string; note?: string; children?: ReactNode }) {
  return (
    <div className={styles.row}>
      <span className={styles.text}>
        <span className={styles.rowTitle}>{title}</span>
        <span className={styles.desc}>{desc}</span>
        {note && <span className={styles.note}>{note}</span>}
      </span>
      <span className={styles.control}>{children}</span>
    </div>
  );
}

function Link({ href, onOpen, children }: { href: string; onOpen?: (href: string) => void; children: ReactNode }) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className={styles.link}
      onClick={(event) => {
        if (!onOpen) return;
        event.preventDefault();
        onOpen(href);
      }}
    >
      {children}
      <Icon name="external" size={13} />
    </a>
  );
}
