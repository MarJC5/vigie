import { type RefObject, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "../Button/Button";
import { Markdown, outline } from "../Markdown/Markdown";
import { TabBar } from "../TabBar/TabBar";
import { type Doc, docNames, docText, linkTarget } from "./docs";
import styles from "./HelpPage.module.css";

export type HelpTab = "start" | Doc;

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

/** The help: getting started (the first host, a host that refuses, the shortcuts), then the docs. */
export function HelpPage({ tab, onTab, onOpen, onAddHost }: HelpPageProps) {
  const { t, i18n } = useTranslation();
  const lang = i18n.language;
  const body = useRef<HTMLDivElement>(null);
  const panel = "help-panel";

  // A new tab starts at its top.
  useEffect(() => {
    body.current?.scrollTo({ top: 0 });
  }, [tab]);

  const link = (href: string) => {
    const target = linkTarget(href, lang);
    if (!target) return;
    if ("doc" in target) onTab(target.doc);
    else onOpen(target.url);
  };
  const quote = (text: string) => t("help.quote", { text: text.split(/(?<=[.?!])\s/)[0] });

  return (
    <div className={styles.page}>
      <div className={styles.header}>
        <h1 className={styles.title}>{t("help.title")}</h1>
        <div className={styles.tabs}>
          <TabBar
            label={t("help.title")}
            tabs={(["start", ...docNames] as HelpTab[]).map((id) => ({ id, label: t(`help.tabs.${id}`) }))}
            value={tab}
            onChange={(id) => onTab(id as HelpTab)}
            panelId={panel}
          />
        </div>
      </div>
      <div ref={body} id={panel} role="tabpanel" tabIndex={0} aria-labelledby={`${panel}-${tab}`} className={styles.body}>
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
          <DocTab key={`${tab}-${lang}`} text={docText(tab, lang)} onLink={link} scroller={body} />
        )}
      </div>
    </div>
  );
}

/** A doc, with its sections beside it; the one read is marked as the page scrolls. */
function DocTab({ text, onLink, scroller }: { text: string; onLink: (href: string) => void; scroller: RefObject<HTMLDivElement | null> }) {
  const { t } = useTranslation();
  const sections = useMemo(() => outline(text), [text]);
  const [current, setCurrent] = useState(sections[0]?.id ?? "");

  useEffect(() => {
    const area = scroller.current;
    if (!area) return;
    // The section read is the last one whose heading went past the top third of the area.
    const follow = () => {
      const limit = area.getBoundingClientRect().top + area.clientHeight / 3;
      let read = sections[0]?.id ?? "";
      for (const s of sections) {
        const heading = document.getElementById(s.id);
        if (heading && heading.getBoundingClientRect().top <= limit) read = s.id;
      }
      setCurrent(read);
    };
    area.addEventListener("scroll", follow, { passive: true });
    return () => area.removeEventListener("scroll", follow);
  }, [scroller, sections]);

  return (
    <div className={styles.docColumns}>
      <Markdown text={text} onLink={onLink} />
      {sections.length > 0 && (
        <nav aria-label={t("help.onPage")} className={styles.toc}>
          <span className={styles.asideTitle} aria-hidden="true">
            {t("help.onPage")}
          </span>
          {sections.map((s) => (
            <a
              key={s.id}
              href={`#${s.id}`}
              aria-current={s.id === current ? "location" : undefined}
              className={styles.tocLink}
              onClick={(event) => {
                event.preventDefault();
                document.getElementById(s.id)?.scrollIntoView({ block: "start" });
                setCurrent(s.id);
              }}
            >
              {s.text}
            </a>
          ))}
        </nav>
      )}
    </div>
  );
}
