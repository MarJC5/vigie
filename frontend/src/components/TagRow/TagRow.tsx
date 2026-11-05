import { type KeyboardEvent, useEffect, useId, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import type { Problem, TagView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { tagChoices } from "../../lib/tags";
import { FieldNote } from "../FieldNote/FieldNote";
import { Icon } from "../Icon/Icon";
import { InlineField } from "../InlineField/InlineField";
import styles from "./TagRow.module.css";

export interface TagRowProps {
  /** The site's tags, in their order. */
  tags: string[];
  /** The tags in use on every site, offered while one is typed. */
  used: TagView[];
  /** Saves the site's tags; a problem keeps the field open and says why. Without it, they are only read. */
  onSave?: (tags: string[]) => Promise<Problem | null>;
  /** Opens a tag's page. */
  onOpenTag?: (tag: string) => void;
}

/** A site's tags under its quick links (ADR-0025): each removed by its own button, a field that
 * offers the tags already used to add one. */
export function TagRow({ tags, used, onSave, onOpenTag }: TagRowProps) {
  const { t } = useTranslation();
  const id = useId();
  // null: the field is closed.
  const [typed, setTyped] = useState<string | null>(null);
  const [active, setActive] = useState(0);
  const [problem, setProblem] = useState<Problem | null>(null);
  const addButton = useRef<HTMLButtonElement>(null);
  const reopen = useRef(false);

  // Back to "Add a tag" once the field closes from the keyboard.
  useEffect(() => {
    if (typed === null && reopen.current) {
      reopen.current = false;
      addButton.current?.focus();
    }
  }, [typed]);

  if (tags.length === 0 && !onSave) return null;
  const choices = typed === null ? [] : tagChoices(used, tags, typed);

  const save = async (next: string[]) => {
    const refused = await onSave!(next);
    setProblem(refused);
    return refused;
  };
  const add = async (name: string) => {
    if (!name.trim()) return;
    if (!(await save([...tags, name]))) {
      setTyped("");
      setActive(0);
    }
  };
  const close = (fromKeyboard: boolean) => {
    reopen.current = fromKeyboard;
    setTyped(null);
    setProblem(null);
    setActive(0);
  };
  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    switch (event.key) {
      case "ArrowDown":
      case "ArrowUp":
        event.preventDefault();
        if (choices.length > 0) setActive((a) => (a + (event.key === "ArrowDown" ? 1 : choices.length - 1)) % choices.length);
        break;
      case "Enter":
        event.preventDefault();
        void add(choices[active]?.name ?? typed ?? "");
        break;
      case "Escape":
        event.preventDefault();
        event.stopPropagation();
        close(true);
        break;
    }
  };

  return (
    <div className={styles.row}>
      <ul className={styles.tags} aria-label={t("tags.label")}>
        {tags.map((tag) => (
          <li key={tag} className={styles.tag}>
            <Icon name="tag" size="xs" />
            {onOpenTag ? (
              <button type="button" className={styles.name} onClick={() => onOpenTag(tag)}>
                {tag}
              </button>
            ) : (
              <span className={styles.name}>{tag}</span>
            )}
            {onSave && (
              <button
                type="button"
                className={styles.remove}
                aria-label={t("tags.remove", { tag })}
                onClick={() => void save(tags.filter((x) => x !== tag))}
              >
                <Icon name="close" size="xs" />
              </button>
            )}
          </li>
        ))}
        {onSave &&
          (typed === null ? (
            <li>
              <button ref={addButton} type="button" className={styles.add} onClick={() => setTyped("")}>
                <Icon name="plus" size="xs" />
                {t("tags.add")}
              </button>
            </li>
          ) : (
            <li className={styles.adding}>
              <span className={styles.field}>
                <InlineField
                  icon="tag"
                  role="combobox"
                  label={t("tags.field")}
                  aria-expanded={choices.length > 0}
                  aria-controls={`${id}-choices`}
                  aria-autocomplete="list"
                  aria-activedescendant={choices.length > 0 ? `${id}-${active}` : undefined}
                  invalid={!!problem}
                  value={typed}
                  maxLength={80}
                  autoFocus
                  onChange={(value) => {
                    setTyped(value);
                    setActive(0);
                    setProblem(null);
                  }}
                  onKeyDown={onKeyDown}
                  onBlur={() => close(false)}
                />
              </span>
              <FieldNote error={!!problem} announce="polite">
                {problem ? t(problem.code) : t("tags.hint")}
              </FieldNote>
              {choices.length > 0 && (
                <ul id={`${id}-choices`} role="listbox" aria-label={t("tags.choices")} className={styles.choices}>
                  {choices.map((c, i) => (
                    <li
                      key={c.name}
                      id={`${id}-${i}`}
                      role="option"
                      aria-selected={i === active}
                      className={styles.choice}
                      // Before the field's blur, which would close the list.
                      onMouseDown={(event) => event.preventDefault()}
                      onMouseEnter={() => setActive(i)}
                      onClick={() => void add(c.name)}
                    >
                      <Icon name={c.isNew ? "plus" : "tag"} size="sm" />
                      <span className={styles.choiceName}>{c.isNew ? t("tags.create", { tag: c.name }) : c.name}</span>
                      <span className={styles.choiceNote}>{c.isNew ? t("tags.new") : t("tags.sites", { count: c.sites })}</span>
                    </li>
                  ))}
                </ul>
              )}
            </li>
          ))}
      </ul>
    </div>
  );
}
