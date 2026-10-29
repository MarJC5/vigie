import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { Node } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { Icon } from "../Icon/Icon";
import styles from "./MoveDialog.module.css";

export interface MoveDialogProps {
  /** The folder or host to move. */
  node: Node;
  /** The whole tree, depth first. */
  nodes: Node[];
  /** The chosen folder, "" for the root. */
  onMove: (folderId: string) => void;
  onClose: () => void;
}

/** "Move to...": the root and the folders a folder or host can go into; its own place is shown but not offered. */
export function MoveDialog({ node, nodes, onMove, onClose }: MoveDialogProps) {
  const { t } = useTranslation();
  const [choice, setChoice] = useState<string>();
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const inside = (id: string) => {
    for (let f: string | undefined = id; f; f = byId.get(f)?.parentId) if (f === node.id) return true;
    return false;
  };
  const depth = (n: Node): number => (n.parentId ? depth(byId.get(n.parentId)!) + 1 : 0);
  const destinations = [
    { id: "", name: t("edit.move.root"), depth: 0 },
    ...nodes.filter((n) => n.kind === "folder" && !inside(n.id)).map((n) => ({ id: n.id, name: n.name, depth: depth(n) + 1 })),
  ];

  return (
    <Dialog
      title={t("edit.move.title", { name: node.name })}
      lead={t(node.kind === "folder" ? "edit.move.leadFolder" : "edit.move.leadHost")}
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
            const current = d.id === node.parentId;
            return (
              <label key={d.id || "root"} className={styles.row} style={{ paddingLeft: 12 + d.depth * 22 }} data-current={current || undefined}>
                <input
                  type="radio"
                  name="destination"
                  className={styles.radio}
                  checked={choice === d.id}
                  disabled={current}
                  onChange={() => setChoice(d.id)}
                />
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
