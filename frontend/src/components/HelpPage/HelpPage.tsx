import { useCallback, useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "../Button/Button";
import { Markdown } from "../Markdown/Markdown";
import { TabBar } from "../TabBar/TabBar";
import { linkTarget, type Topic, topicNames, topicText } from "./docs";
import styles from "./HelpPage.module.css";

export type HelpTab = "start" | Topic;

export interface HelpPageProps {
  tab: HelpTab;
  onTab: (tab: HelpTab) => void;
  /** Opens an address in the browser. */
  onOpen: (href: string) => void;
  /** Opens the "add a host" wizard; the button is left out without it. */
  onAddHost?: () => void;
}

// The shortcuts of the menu bar (internal/app/menu.go), with the label of their item.
const shortcuts = [
  ["⌘N", "menu.newHost"],
  ["⇧⌘N", "menu.newFolder"],
  ["⌘F", "menu.search"],
  ["⌘R", "menu.scan"],
  ["⇧⌘R", "menu.scanAll"],
  ["⌘,", "menu.settings"],
] as const;

// What a host that refuses answers: the first sentence of the message Vigie shows, the cause, and
// what to do.
const refusals = [
  ["failure.timedOut", "help.start.ipFilter"],
  ["failure.authRefused", "help.start.key"],
  ["failure.refused", "help.start.port"],
] as const;

const steps = ["step1", "step2", "step3", "step4"] as const;

/** The help: getting started (the first host, a host that refuses, the shortcuts), then short topics. */
export function HelpPage({ tab, onTab, onOpen, onAddHost }: HelpPageProps) {
  const { t, i18n } = useTranslation();
  const lang = i18n.language;
  const body = useRef<HTMLDivElement>(null);
  const panel = "help-panel";

  // A new tab starts at its top.
  useEffect(() => {
    // The window's content area scrolls the page: back to its top.
    body.current?.closest("main")?.scrollTo({ top: 0 });
  }, [tab]);

  // Stable while the language is: the topic is not drawn again on each render.
  const link = useCallback(
    (href: string) => {
      const target = linkTarget(href, lang);
      if (!target) return;
      if ("topic" in target) onTab(target.topic);
      else onOpen(target.url);
    },
    [lang, onTab, onOpen],
  );
  const leaves = useCallback((href: string) => "url" in (linkTarget(href, lang) ?? {}), [lang]);
  const quote = (text: string) => t("help.quote", { text: text.split(/(?<=[.?!])\s/)[0] });

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}>{t("help.title")}</h1>
        <TabBar
          label={t("help.title")}
          tabs={(["start", ...topicNames] as HelpTab[]).map((id) => ({ id, label: t(`help.tabs.${id}`) }))}
          value={tab}
          onChange={(id) => onTab(id as HelpTab)}
          panelId={panel}
        />
      </div>
      <div ref={body} id={panel} role="tabpanel" aria-labelledby={`${panel}-${tab}`} className={styles.body}>
        {tab === "start" ? (
          <div className={styles.columns}>
            <div className={styles.main}>
              <section className={styles.section} aria-labelledby="help-first-host">
                <h2 id="help-first-host" className={styles.heading}>
                  {t("help.start.firstHost")}
                </h2>
                <ol className={styles.steps}>
                  {steps.map((step, i) => (
                    <li key={step} className={styles.step}>
                      <span className={styles.number} aria-hidden="true">
                        {i + 1}
                      </span>
                      <span>
                        <strong className={styles.strong}>{t(`help.start.${step}.title`)}</strong>{" "}
                        <span className={styles.muted}>{t(`help.start.${step}.text`)}</span>
                      </span>
                    </li>
                  ))}
                </ol>
                {onAddHost && (
                  <span className={styles.action}>
                    <Button variant="primary" onClick={onAddHost}>
                      {t("action.addHost")}
                    </Button>
                  </span>
                )}
              </section>
              <section className={styles.section} aria-labelledby="help-refused">
                <h2 id="help-refused" className={styles.heading}>
                  {t("help.start.refused")}
                </h2>
                <p className={styles.lead}>{t("help.start.refusedLead")}</p>
                <ul className={styles.refusals}>
                  {refusals.map(([message, help]) => (
                    <li key={message} className={styles.refusal}>
                      <span className={styles.says} data-selectable>
                        {quote(t(message))}
                      </span>
                      <span>
                        <strong className={styles.strong}>{t(`${help}.cause`)}</strong> {t(`${help}.todo`)}
                      </span>
                    </li>
                  ))}
                </ul>
              </section>
            </div>
            <aside className={styles.aside} aria-labelledby="help-shortcuts">
              <h2 id="help-shortcuts" className={styles.asideTitle}>
                {t("help.start.shortcuts")}
              </h2>
              <dl className={styles.shortcuts}>
                {shortcuts.map(([keys, label]) => (
                  <div key={keys} className={styles.shortcut}>
                    <dt>
                      <kbd className={styles.kbd}>{keys}</kbd>
                    </dt>
                    <dd>{t(label).replace(/…$/, "")}</dd>
                  </div>
                ))}
              </dl>
            </aside>
          </div>
        ) : (
          <div className={styles.topic}>
            <Markdown text={topicText(tab, lang)} onLink={link} external={leaves} />
          </div>
        )}
      </div>
    </div>
  );
}
