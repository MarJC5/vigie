import { type FormEvent, useState } from "react";
import { useTranslation } from "react-i18next";

import type { Node, Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { fold } from "../../lib/text";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { FieldNote } from "../FieldNote/FieldNote";
import { ItemList } from "../ItemList/ItemList";
import { TextField } from "../TextField/TextField";
import styles from "./TagManyDialog.module.css";

export interface TagManyDialogProps {
  /** The sites a selection holds, as the tree has them now. */
  sites: Node[];
  /** Adds a tag to every site (add), or takes it off those that have it; a problem keeps the
   dialog open and says why. */
  onTag: (tag: string, add: boolean) => Promise<Problem | null>;
  onClose: () => void;
}

/** The tags of several sites at once (ADR-0034): one added to all of them, or one removed from
 those that have it. */
export function TagManyDialog({ sites, onTag, onClose }: TagManyDialogProps) {
  const { t } = useTranslation();
  const [typed, setTyped] = useState("");
  const [problem, setProblem] = useState<Problem | null>(null);
  // Each tag once, whatever its case, with how many of the sites have it.
  const tags = new Map<string, { name: string; count: number }>();
  for (const site of sites) {
    for (const tag of site.tags ?? []) {
      const known = tags.get(fold(tag));
      if (known) known.count++;
      else tags.set(fold(tag), { name: tag, count: 1 });
    }
  }
  const listed = [...tags.values()].sort((a, b) => fold(a.name).localeCompare(fold(b.name)));
  const total = sites.length;

  const tag = async (name: string, add: boolean) => {
    const refused = await onTag(name, add);
    setProblem(refused);
    if (!refused && add) setTyped("");
  };
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (typed.trim()) void tag(typed.trim(), true);
  };

  return (
    <Dialog
      title={t("selection.tags.title", { count: total })}
      lead={t("selection.tags.lead")}
      onClose={onClose}
      actions={
        <Button variant="outline" onClick={onClose}>
          {t("selection.tags.done")}
        </Button>
      }
    >
      <form className={styles.add} onSubmit={submit}>
        <TextField
          label={t("selection.tags.field")}
          value={typed}
          onChange={(value) => {
            setTyped(value);
            setProblem(null);
          }}
          autoFocus
        />
        <Button type="submit" disabled={!typed.trim()}>
          {t("selection.tags.add")}
        </Button>
      </form>
      {/* Under the row, so that the button stays level with the field. */}
      {problem && (
        <FieldNote error announce="alert">
          {t(problem.code, { message: problem.detail })}
        </FieldNote>
      )}
      {listed.length === 0 ? (
        <p className={styles.none}>{t("selection.tags.none")}</p>
      ) : (
        <ItemList
          items={listed.map((g) => ({
            key: fold(g.name),
            icon: "tag",
            name: g.name,
            note: g.count === total ? t("selection.tags.onAll", { count: total }) : t("selection.tags.onSome", { n: g.count, total }),
            action: (
              <Button variant="text" label={t("selection.tags.removeLabel", { tag: g.name })} onClick={() => void tag(g.name, false)}>
                {t("selection.tags.remove")}
              </Button>
            ),
          }))}
        />
      )}
    </Dialog>
  );
}
