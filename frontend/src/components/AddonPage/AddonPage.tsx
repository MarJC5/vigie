import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { AddonView, Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { canRun, programOf, textIn } from "../../lib/addons";
import { AddonImage } from "../AddonImage/AddonImage";
import { AddonTrustDialog } from "../AddonTrustDialog/AddonTrustDialog";
import { Button } from "../Button/Button";
import { Checkbox } from "../Checkbox/Checkbox";
import { Icon } from "../Icon/Icon";
import { SelectField } from "../SelectField/SelectField";
import { TextField } from "../TextField/TextField";
import { ExternalLink } from "../ExternalLink/ExternalLink";
import { Notice } from "../Notice/Notice";
import styles from "./AddonPage.module.css";

export interface AddonPageProps {
  addon: AddonView;
  /** The values of its settings. */
  values: Record<string, string>;
  onValue: (id: string, value: string) => void;
  /** Asks for the file of a file setting. */
  onChooseFile: (id: string) => void;
  onRun: () => void;
  /** The add-on runs: its button waits, and the page can be left. */
  running?: boolean;
  /** Why the last run failed, with what the add-on said. */
  failure?: { problem: Problem; report: string } | null;
  onBack: () => void;
  onLink: (href: string) => void;
}

/** One add-on's page: drawn by Vigie from what the add-on says about itself, with the settings of a
 run and what it reads, writes and connects to. */
export function AddonPage({ addon, values, onValue, onChooseFile, onRun, running = false, failure, onBack, onLink }: AddonPageProps) {
  const { t, i18n } = useTranslation();
  const lang = i18n.language;
  const [trust, setTrust] = useState(false);
  const program = programOf(addon);
  const title = textIn(addon.title, lang);
  return (
    <div className={styles.page}>
      <a
        href="#addons"
        className={styles.back}
        onClick={(event) => {
          event.preventDefault();
          onBack();
        }}
      >
        <span className={styles.backIcon}>
          <Icon name="caretRight" size="xs" />
        </span>
        {t("addons.title")}
      </a>
      <div className={styles.top}>
        <AddonImage image={addon.image} size="hero" />
        <div className={styles.intro}>
          <h1 className={styles.title}>{title}</h1>
          <span className={styles.meta}>
            {t("addons.meta", { author: addon.author, kind: t(`addons.kind.${addon.kind}`), release: addon.release })}
          </span>
          <p className={styles.description}>{textIn(addon.description, lang)}</p>
        </div>
      </div>
      <div className={styles.columns}>
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>
            {t("addons.runTitle")}
          </h2>
          {addon.problem ? (
            <Notice tone="neutral" title={t("addons.unusable")} text={t(addon.problem.code, { message: addon.problem.detail })} />
          ) : !addon.installed ? (
            <p className={styles.lead}>{t("addons.notInstalled")}</p>
          ) : addon.kind !== "source" ? (
            <p className={styles.lead}>{t("addons.destinationNote", { name: addon.name })}</p>
          ) : (
            <>
              <p className={styles.lead}>{t("addons.runLead", { program })}</p>
              <div className={styles.settings}>
                {(addon.settings ?? []).map((s) => {
                  const label = textIn(s.label, lang);
                  const help = textIn(s.help, lang);
                  const value = values[s.id] ?? "";
                  switch (s.kind) {
                    case "checkbox":
                      return <Checkbox key={s.id} label={label} checked={value === "true"} onChange={(on) => onValue(s.id, on ? "true" : "false")} />;
                    case "choice":
                      return (
                        <SelectField
                          key={s.id}
                          label={label}
                          value={value}
                          options={(s.choices ?? []).map((c) => ({ value: c.value, label: textIn(c.label, lang) }))}
                          onChange={(v) => onValue(s.id, v)}
                        />
                      );
                    case "file":
                      return (
                        <div key={s.id} className={styles.fileSetting}>
                          <div className={styles.file}>
                            <TextField label={label} value={value} onChange={(v) => onValue(s.id, v)} />
                            <Button onClick={() => onChooseFile(s.id)}>{t("addons.chooseFile")}</Button>
                          </div>
                          {help && <span className={styles.help}>{help}</span>}
                        </div>
                      );
                    default:
                      return <TextField key={s.id} label={label} value={value} onChange={(v) => onValue(s.id, v)} hint={help || undefined} />;
                  }
                })}
              </div>
              {failure && (
                <Notice
                  tone="neutral"
                  title={t("addons.failedTitle", { program })}
                  text={failure.report ? t("addons.failedText") : t(failure.problem.code, { message: failure.problem.detail })}
                  detail={failure.report}
                />
              )}
              <div className={styles.runRow}>
                <Button variant="primary" disabled={running || !canRun(addon, values)} aria-busy={running} onClick={onRun}>
                  {running ? t("addons.running") : t("addons.run")}
                </Button>
                <Button variant="icon" icon="lock" label={t("addons.trust")} title={t("addons.trust")} onClick={() => setTrust(true)} className={styles.lock} />
                <span className={styles.lead}>{t("addons.nothingBefore")}</span>
              </div>
            </>
          )}
        </section>
        <section className={styles.section}>
          <h2 className={styles.sectionTitle}>
            {t("addons.details")}
          </h2>
          <dl className={styles.details}>
            {(addon.details ?? []).map((d) => (
              <div key={textIn(d.label, "en")} className={styles.detail}>
                <dt>{textIn(d.label, lang)}</dt>
                <dd>{textIn(d.value, lang)}</dd>
              </div>
            ))}
            {addon.path && (
              <div className={styles.detail}>
                <dt>{t("addons.installedAt")}</dt>
                <dd className={styles.path} data-selectable>
                  {addon.path}
                </dd>
              </div>
            )}
          </dl>
          {addon.install && !addon.installed && (
            <ExternalLink href={addon.install} onOpen={onLink}>
              {t("addons.howTo")}
            </ExternalLink>
          )}
        </section>
      </div>
      {trust && <AddonTrustDialog onClose={() => setTrust(false)} />}
    </div>
  );
}

