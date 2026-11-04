import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { CopyPreviewView, Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sizeText } from "../../lib/data";
import { Button } from "../Button/Button";
import { Checkbox } from "../Checkbox/Checkbox";
import { ChoiceCard } from "../ChoiceCard/ChoiceCard";
import { Dialog } from "../Dialog/Dialog";
import { Notice } from "../Notice/Notice";
import styles from "./CopyDialog.module.css";

export type CopyContent = "database" | "content" | "site";

/** The parts of a site that have their own archives, as the Go side names their folders. */
const parts = new Set(["plugins", "themes", "mu-plugins", "uploads"]);

export interface CopyDialogProps {
  name: string;
  /** The sizes read on the server; undefined while they are read. */
  preview?: CopyPreviewView;
  /** Why the sizes could not be read. */
  problem?: Problem | null;
  /** Starts the copy; its problem, when it was refused (the disk too small). */
  onCopy: (content: CopyContent, leftOut: string[]) => Promise<Problem | null>;
  onClose: () => void;
}

/** About how many archives a copy makes: each part split by size, the database dumped by tables;
 the files are listed only when the copy starts, so it is a count of the sizes, not of the files. */
export function archivesAbout(p: CopyPreviewView, content: CopyContent, leftOut: string[]): number {
  const limit = p.archiveSize * 1_000_000;
  const of = (bytes: number) => (p.split ? Math.max(1, Math.ceil(bytes / limit)) : 1);
  let n = of(p.database);
  if (content === "database") return n;
  let others = 0;
  for (const f of p.folders ?? []) {
    if (leftOut.includes(f.name)) continue;
    if (parts.has(f.name)) n += of(f.bytes);
    else others += f.bytes;
  }
  n += of(others);
  if (content === "site") n += of(p.site - p.content);
  return n;
}

/** The copy of a site to this computer (ADR-0041, mockups 28a to 28c): what to copy, the folders of
 wp-content left out, and how much it reads against the room there is. */
export function CopyDialog({ name, preview, problem, onCopy, onClose }: CopyDialogProps) {
  const { t, i18n } = useTranslation();
  const [content, setContent] = useState<CopyContent>("content");
  const [leftOut, setLeftOut] = useState<string[]>([]);
  const [refused, setRefused] = useState<Problem | null>(null);
  const [starting, setStarting] = useState(false);
  const size = (bytes: number) => sizeText(bytes, i18n.language);

  const total = preview
    ? content === "database"
      ? preview.database
      : (content === "site" ? preview.site : preview.content) -
        (preview.folders ?? []).filter((f) => leftOut.includes(f.name)).reduce((n, f) => n + f.bytes, 0)
    : 0;
  const noSpace = preview !== undefined && preview.free >= 0 && total > preview.free;
  const free = preview && preview.free >= 0 ? size(preview.free) : "?";
  const count = preview ? archivesAbout(preview, content, leftOut) : 0;
  const summary =
    preview &&
    (noSpace
      ? t("copies.dialog.noSpace", { total: size(total), free })
      : t(preview.split ? "copies.dialog.split" : "copies.dialog.unsplit", {
          parts: t("copies.dialog.parts", { count }),
          size: size(preview.archiveSize * 1_000_000),
          total: size(total),
          free,
        }));

  const copy = async () => {
    setStarting(true);
    const p = await onCopy(content, content === "database" ? [] : leftOut);
    setStarting(false);
    if (p) setRefused(p);
    else onClose();
  };

  const contents: Array<{ id: CopyContent; bytes: number }> = preview
    ? [
        { id: "database", bytes: preview.database },
        { id: "content", bytes: preview.content },
        { id: "site", bytes: preview.site },
      ]
    : [];
  return (
    <Dialog
      title={t("copies.dialog.title", { name })}
      lead={preview ? t("copies.dialog.lead") : undefined}
      wide
      onClose={onClose}
      progress={!preview && !problem ? { label: t("copies.dialog.reading") } : undefined}
      actions={
        <>
          <Button variant="outline" onClick={onClose}>
            {t("edit.cancel")}
          </Button>
          <Button variant="primary" onClick={() => void copy()} disabled={!preview || noSpace || starting} aria-busy={starting || undefined}>
            {t("copies.dialog.confirm")}
          </Button>
        </>
      }
    >
      {problem && <Notice text={t(problem.code, { message: problem.detail })} />}
      {preview && (
        <div className={styles.body}>
          <div className={styles.contents}>
            {contents.map((c) => (
              <ChoiceCard
                key={c.id}
                title={t(`copies.content.${c.id}`)}
                text={`${t(`copies.content.${c.id}Desc`)} ${size(c.bytes)}`}
                selected={content === c.id}
                onSelect={() => setContent(c.id)}
              />
            ))}
          </div>
          {content !== "database" && (preview.folders ?? []).length > 0 && (
            <fieldset className={styles.folders}>
              <legend className={styles.legend}>{t("copies.dialog.folders")}</legend>
              {(preview.folders ?? []).map((f) =>
                f.name === "" ? (
                  <div key="" className={styles.loose}>
                    <span>{t("copies.dialog.loose")}</span>
                    <span className={styles.size}>{size(f.bytes)}</span>
                  </div>
                ) : (
                  <div key={f.name} className={styles.folder}>
                    <Checkbox
                      label={f.name}
                      checked={!leftOut.includes(f.name)}
                      onChange={(on) => setLeftOut((l) => (on ? l.filter((n) => n !== f.name) : [...l, f.name]))}
                    />
                    <span className={leftOut.includes(f.name) ? styles.sizeLeft : styles.size}>{size(f.bytes)}</span>
                  </div>
                ),
              )}
            </fieldset>
          )}
          {refused ? (
            <Notice text={t(refused.code, { message: refused.detail })} />
          ) : (
            <div className={noSpace ? styles.summaryNoSpace : styles.summary} role={noSpace ? "alert" : undefined}>
              <span>{summary}</span>
              <span className={styles.destination}>{preview.destination}</span>
            </div>
          )}
        </div>
      )}
    </Dialog>
  );
}
