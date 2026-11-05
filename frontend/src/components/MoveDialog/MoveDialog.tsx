import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { Node } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { carriedKey } from "../../lib/selection";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { Icon } from "../Icon/Icon";
import { RadioControl } from "../Radio/Radio";
import styles from "./MoveDialog.module.css";

export interface MoveDialogProps {
  /** The folders and hosts to move. */
  items: Node[];
  /** Sites selected with them, which stay under their hosts (ADR-0034). */
  staying?: Node[];
  /** The whole tree, depth first. */
  nodes: Node[];
  /** The chosen folder, "" for the root. */
  onMove: (folderId: string) => void;
  onClose: () => void;
}

/** "Move to...": the root and the folders folders and hosts can go into; the place where they all
 are is shown but not offered. */
export function MoveDialog({ items, staying = [], nodes, onMove, onClose }: MoveDialogProps) {
  const { t } = useTranslation();
  const [choice, setChoice] = useState<string>();
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const moved = new Set(items.map((n) => n.id));
  const inside = (id: string) => {
    for (let f: string | undefined = id; f; f = byId.get(f)?.parentId) if (moved.has(f)) return true;
    return false;
  };
  const [node] = items;
  const several = items.length > 1;
  const title = several
    ? t("selection.move.title", { what: t(carriedKey(items.map((n) => n.kind)), { count: items.length }) })
    : t("edit.move.title", { name: node?.name });
  const lead = several
    ? [
        t(items.every((n) => n.kind === "host") ? "selection.move.leadHosts" : "selection.move.leadItems"),
        ...(staying.length > 0 ? [t("selection.move.sitesStay", { count: staying.length, names: staying.map((n) => n.name).join(", ") })] : []),
      ].join(" ")
    : t(node?.kind === "folder" ? "edit.move.leadFolder" : "edit.move.leadHost");
  const depth = (n: Node): number => (n.parentId ? depth(byId.get(n.parentId)!) + 1 : 0);
  const destinations = [
    { id: "", name: t("edit.move.root"), depth: 0 },
    ...nodes.filter((n) => n.kind === "folder" && !inside(n.id)).map((n) => ({ id: n.id, name: n.name, depth: depth(n) + 1 })),
  ];

  return (
    <Dialog
      title={title}
      lead={lead}
      onClose={onClose}
      actions={
        <>
          <Button variant="outline" onClick={onClose}>
            {t("edit.cancel")}
          </Button>
          <Button variant="primary" disabled={choice === undefined} onClick={() => choice !== undefined && onMove(choice)}>
            {t("edit.move.confirm")}
          </Button>
        </>
      }
    >
      <fieldset className={styles.group}>
        <legend className={styles.legend}>{t("edit.move.destination")}</legend>
        <div className={styles.list}>
          {destinations.map((d) => {
            const current = items.every((n) => n.parentId === d.id);
            return (
              <label key={d.id || "root"} className={styles.row} style={{ paddingLeft: 12 + d.depth * 22 }} data-current={current || undefined}>
                <RadioControl name="destination" checked={choice === d.id} disabled={current} onChange={() => setChoice(d.id)} />
                <Icon name="folder" size={16} />
                <span className={styles.name}>{d.name}</span>
                {current && <span className={styles.note}>{t("edit.move.current")}</span>}
              </label>
            );
          })}
        </div>
      </fieldset>
    </Dialog>
  );
}
