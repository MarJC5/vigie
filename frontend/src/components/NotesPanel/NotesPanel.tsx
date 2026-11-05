import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { Button } from "../Button/Button";
import { FieldNote } from "../FieldNote/FieldNote";
import { Markdown } from "../Markdown/Markdown";
import { TextArea } from "../TextArea/TextArea";
import styles from "./NotesPanel.module.css";

/** The most characters notes hold, as the Go side checks (model.MaxNotes). */
export const maxNotes = 20000;

export interface NotesPanelProps {
  /** The notes, in Markdown; "" when there are none. */
  notes: string;
  /** Whose notes, for the field's name. */
  name: string;
  /** A heading over them ("Notes" on a host's or folder's page); none in a site's Notes tab. */
  title?: string;
  /** Saves the notes; a problem keeps them in edition. Without it, they are only read. */
  onSave?: (notes: string) => Promise<Problem | null>;
  /** Opens a web address of the notes in the browser. */
  onOpen?: (href: string) => void;
}

/** Markdown notes, read as a document and edited in place (ADR-0023, point 2). */
export function NotesPanel({ notes, name, title, onSave, onOpen }: NotesPanelProps) {
  const { t, i18n } = useTranslation();
  const [draft, setDraft] = useState<string>();
  const [problem, setProblem] = useState<Problem | null>(null);
  const [saving, setSaving] = useState(false);
  const editing = draft !== undefined;
  const length = editing ? [...draft].length : 0;
  const number = new Intl.NumberFormat(i18n.language);

  const save = async () => {
    if (!onSave || draft === undefined) return;
    setSaving(true);
    const refused = await onSave(draft);
    setSaving(false);
    setProblem(refused);
    if (!refused) setDraft(undefined);
  };
  // Links of notes open in the browser; another address is not followed.
  const link = (href: string) => {
    if (/^(https?|mailto):/i.test(href)) onOpen?.(href);
  };
  const Heading = title ? "h2" : "span";

  return (
    <section className={styles.panel} aria-label={title ? undefined : t("notes.title")}>
      <div className={styles.header}>
        {title ? <Heading className={styles.title}>{title}</Heading> : <span />}
        {onSave && !editing && (
          <Button icon="note" onClick={() => setDraft(notes)}>
            {notes ? t("notes.edit") : t("notes.add")}
          </Button>
        )}
      </div>
      {editing ? (
        <div className={styles.edit}>
          <TextArea
            label={t("notes.label", { name })}
            hideLabel
            variant="notes"
            invalid={!!problem || length > maxNotes}
            value={draft}
            autoFocus
            spellCheck
            onChange={(value) => {
              setDraft(value);
              setProblem(null);
            }}
          />
          <div className={styles.footer}>
            <span className={styles.status}>
              <FieldNote error={!!problem} announce="polite">
                {problem ? t(problem.code) : t("notes.hint")}
              </FieldNote>
            </span>
            <span className={styles.count} data-over={length > maxNotes || undefined}>
              {t("notes.count", { count: number.format(length), max: number.format(maxNotes) })}
            </span>
            <Button
              onClick={() => {
                setDraft(undefined);
                setProblem(null);
              }}
            >
              {t("action.cancel")}
            </Button>
            <Button variant="primary" onClick={() => void save()} disabled={saving || length > maxNotes} aria-busy={saving || undefined}>
              {t("notes.save")}
            </Button>
          </div>
        </div>
      ) : notes ? (
        <Markdown text={notes} onLink={link} level={title ? 3 : 2} />
      ) : (
        <p className={styles.empty}>{t("notes.empty")}</p>
      )}
    </section>
  );
}
