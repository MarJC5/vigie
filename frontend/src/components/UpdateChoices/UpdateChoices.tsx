import { useId } from "react";
import { useTranslation } from "react-i18next";

import { type Choice, type ChoiceGroup, ignoredText, itemName, setAsideText } from "../../lib/updates";
import { CheckboxControl } from "../Checkbox/Checkbox";
import { ExternalLink } from "../ExternalLink/ExternalLink";
import { Icon } from "../Icon/Icon";
import { LinkButton } from "../LinkButton/LinkButton";
import styles from "./UpdateChoices.module.css";

export interface UpdateChoicesProps {
  groups: ChoiceGroup[];
  /** The keys of the items chosen. */
  chosen: Set<string>;
  onChange: (chosen: Set<string>) => void;
  /** The titles of the plugins and themes, by key. */
  names: Map<string, string>;
  /** Opens the dialog that ignores an update (ADR-0026); without it, no "Ignore". */
  onIgnore?: (choice: Choice) => void;
  /** Counts an ignored update again. */
  onRestore?: (choice: Choice) => void;
  /** Opens a changelog in the browser; without it, the link opens a new tab. */
  onOpen?: (href: string) => void;
}

/** The updates of a site by kind, each offered one with a checkbox, its changelog and "Ignore"; a
 kind with none says so; the versions set aside (ADR-0042), then the ignored updates, last,
 unticked, with why. */
export function UpdateChoices({ groups, chosen, onChange, names, onIgnore, onRestore, onOpen }: UpdateChoicesProps) {
  const { t } = useTranslation();
  const id = useId();
  const toggle = (key: string, on: boolean) => {
    const next = new Set(chosen);
    if (on) next.add(key);
    else next.delete(key);
    onChange(next);
  };

  return (
    <div className={styles.groups}>
      {groups.map((group) => (
        <div role="group" key={group.kind} className={styles.group} aria-labelledby={`${id}-${group.kind}`}>
          <h2 id={`${id}-${group.kind}`} className={styles.heading}>
            {t(`update.group.${group.kind}`)}
            {(group.kind === "ignored" || group.kind === "setAside") && <span className={styles.count}>{group.choices.length}</span>}
          </h2>
          {group.choices.length === 0 ? (
            <div className={styles.row} data-muted>
              <span className={styles.mark}>
                <Icon name="ok" />
              </span>
              <span>{t(group.kind === "core" ? "update.upToDateOne" : "update.upToDate")}</span>
            </div>
          ) : (
            group.choices.map((choice) => (
              <ChoiceRow
                key={choice.key}
                choice={choice}
                name={choiceName(t, choice, names)}
                checked={chosen.has(choice.key)}
                onChange={(on) => toggle(choice.key, on)}
                onIgnore={onIgnore}
                onRestore={onRestore}
                onOpen={onOpen}
              />
            ))
          )}
        </div>
      ))}
    </div>
  );
}

interface ChoiceRowProps extends Pick<UpdateChoicesProps, "onIgnore" | "onRestore" | "onOpen"> {
  choice: Choice;
  name: string;
  checked: boolean;
  onChange: (on: boolean) => void;
}

function ChoiceRow({ choice, name, checked, onChange, onIgnore, onRestore, onOpen }: ChoiceRowProps) {
  const { t } = useTranslation();
  const change = choice.from && choice.to ? `${choice.from} → ${choice.to}` : "";
  if (!choice.offered) {
    return (
      <div className={styles.row} data-muted>
        <span className={styles.mark}>
          <Icon name="lock" />
        </span>
        <span className={styles.name}>{name}</span>
        <span className={styles.change}>{change}</span>
        <span className={styles.note}>{t(`update.notOffered.${choice.reason}`)}</span>
      </div>
    );
  }
  return (
    <div className={styles.row}>
      {/* The label lays out in the row's grid: a click on the name ticks the box. */}
      <label className={styles.pick}>
        <span className={styles.mark}>
          <CheckboxControl checked={checked} onChange={onChange} />
        </span>
        <span className={styles.nameCell}>
          <span className={styles.name}>{name}</span>
          {choice.ignore && <span className={styles.detail}>{ignoredText(choice.ignore, t)}</span>}
          {choice.setAside && <span className={styles.detail}>{setAsideText(choice.setAside, t)}</span>}
        </span>
        <span className={styles.change}>{change}</span>
      </label>
      <span className={styles.actions}>
        {choice.changelog && (
          <ExternalLink href={choice.changelog} onOpen={onOpen}>
            {t(choice.kind === "core" ? "update.releaseNotes" : "update.changelog")}
          </ExternalLink>
        )}
        {choice.ignore
          ? onRestore && (
              <LinkButton label={t("update.restoreNamed", { name })} onClick={() => onRestore(choice)}>
                {t("update.restore")}
              </LinkButton>
            )
          : onIgnore &&
            choice.ignoreKey && (
              <LinkButton icon="eyeSlash" label={t("update.ignoreNamed", { name })} onClick={() => onIgnore(choice)}>
                {t("update.ignore")}
              </LinkButton>
            )}
      </span>
    </div>
  );
}

type T = (key: string, vars?: Record<string, unknown>) => string;

/** A choice's name; the translations name what they translate. */
function choiceName(t: T, choice: Choice, names: Map<string, string>): string {
  if (choice.kind !== "language") return choice.kind === "core" ? t("update.group.core") : choice.name;
  const of = choice.of.map((slug) =>
    slug === "" ? t("update.group.core") : (names.get(`plugin:${slug}`) ?? names.get(`theme:${slug}`) ?? itemName(t, "plugin", slug, names)),
  );
  return t("update.languages", { languages: choice.languages.join(", "), items: joined(t, of) });
}

/** "A, B and C" in the interface language. */
function joined(t: T, list: string[]): string {
  if (list.length < 2) return list.join("");
  return list.slice(0, -1).join(", ") + t("update.and") + list.at(-1);
}
