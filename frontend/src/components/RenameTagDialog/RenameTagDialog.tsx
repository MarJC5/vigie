import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { Node, Problem, TagView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { hasTag } from "../../lib/tags";
import { fold } from "../../lib/text";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { Notice } from "../Notice/Notice";
import { TextField } from "../TextField/TextField";

export interface RenameTagDialogProps {
  tag: string;
  /** The tags in use: a name another one has merges them. */
  tags: TagView[];
  /** The site manager, to count the sites of a merge. */
  nodes: Node[];
  /** Renames the tag; a problem keeps the dialog open. */
  onRename: (name: string) => Promise<Problem | null>;
  onClose: () => void;
}

/** Renames a tag on every site that has it (ADR-0025); to the name of another tag, both become one. */
export function RenameTagDialog({ tag, tags, nodes, onRename, onClose }: RenameTagDialogProps) {
  const { t } = useTranslation();
  const [name, setName] = useState(tag);
  const [problem, setProblem] = useState<Problem | null>(null);
  const sites = nodes.filter((n) => n.kind === "site" && hasTag(n.tags, tag));
  const other = tags.find((u) => fold(u.name) === fold(name.trim()) && fold(u.name) !== fold(tag));
  const merged = other ? nodes.filter((n) => n.kind === "site" && (hasTag(n.tags, tag) || hasTag(n.tags, other.name))).length : 0;
  const unchanged = name.trim() === "" || name.trim() === tag;
  const rename = async () => {
    const refused = await onRename(name);
    if (refused) setProblem(refused);
  };
  return (
    <Dialog
      title={t("tags.rename.title", { tag })}
      lead={t("tags.rename.lead", { count: sites.length })}
      onClose={onClose}
      actions={
        <>
          <Button onClick={onClose}>{t("action.cancel")}</Button>
          <Button variant="primary" disabled={unchanged} onClick={() => void rename()}>
            {t("tags.rename.confirm")}
          </Button>
        </>
      }
    >
      <TextField
        label={t("tags.rename.label")}
        value={name}
        onChange={(value) => {
          setName(value);
          setProblem(null);
        }}
        autoFocus
        error={problem ? t(problem.code) : undefined}
      />
      {other && (
        <Notice
          tone="neutral"
          icon="tag"
          role="note"
          title={t("tags.rename.mergeTitle", { tag: other.name })}
          text={t("tags.rename.merge", { tag: other.name, count: merged })}
        />
      )}
    </Dialog>
  );
}
