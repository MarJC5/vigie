import { useId } from "react";
import { useTranslation } from "react-i18next";

import { type Choice, type ChoiceGroup, itemName } from "../../lib/updates";
import { Icon } from "../Icon/Icon";
import styles from "./UpdateChoices.module.css";

export interface UpdateChoicesProps {
  groups: ChoiceGroup[];
  /** The keys of the items chosen. */
  chosen: Set<string>;
  onChange: (chosen: Set<string>) => void;
  /** The titles of the plugins and themes, by key. */
  names: Map<string, string>;
}

/** The updates of a site by kind, each offered one with a checkbox; a kind with none says so. */
export function UpdateChoices({ groups, chosen, onChange, names }: UpdateChoicesProps) {
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
              />
            ))
          )}
        </div>
      ))}
    </div>
  );
}

function ChoiceRow({ choice, name, checked, onChange }: { choice: Choice; name: string; checked: boolean; onChange: (on: boolean) => void }) {
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
    <label className={styles.row}>
      <span className={styles.mark}>
        <input type="checkbox" className={styles.box} checked={checked} onChange={(event) => onChange(event.target.checked)} />
      </span>
      <span className={styles.name}>{name}</span>
      <span className={styles.change}>{change}</span>
    </label>
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
