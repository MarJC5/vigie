import { useTranslation } from "react-i18next";

import type { Node } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { Icon } from "../Icon/Icon";
import styles from "./DeleteDialog.module.css";

export interface DeleteDialogProps {
  node: Node;
  /** The whole tree, depth first: what goes with the node. */
  nodes: Node[];
  onDelete: () => void;
  onClose: () => void;
}

/** Asks before removing a site, a host with its sites, or an empty folder; servers are not touched. */
export function DeleteDialog({ node, nodes, onDelete, onClose }: DeleteDialogProps) {
  const { t } = useTranslation();
  const children = nodes.filter((n) => n.parentId === node.id);
  const kind = node.kind === "host" ? "Host" : node.kind === "site" ? "Site" : "Folder";
  const blocked = node.kind === "folder" && children.length > 0;
  const lead = {
    Host: t("edit.delete.leadHost", { count: children.length }),
    Site: t("edit.delete.leadSite"),
    Folder: blocked ? t("edit.delete.notEmpty") : t("edit.delete.leadFolder"),
  }[kind];

  return (
    <Dialog
      title={t(`edit.delete.title${kind}`, { name: node.name })}
      lead={lead}
      onClose={onClose}
      actions={
        <>
          <Button variant="outline" onClick={onClose}>
            {t("edit.cancel")}
          </Button>
          {!blocked && (
            <Button variant="danger" onClick={onDelete}>
              {t(`edit.delete.confirm${kind}`)}
            </Button>
          )}
        </>
      }
    >
      {node.kind === "host" && children.length > 0 && (
        <ul className={styles.sites}>
          {children.map((site) => (
            <li key={site.id} className={styles.site}>
              <Icon name="sites" size={14} />
              <span className={styles.name}>{site.name}</span>
              {site.multisite && site.subsites > 0 && (
                <span className={styles.note}>{t("edit.delete.network", { count: site.subsites })}</span>
              )}
            </li>
          ))}
        </ul>
      )}
      {!blocked && node.kind !== "folder" && <p className={styles.keep}>{t("edit.delete.keep")}</p>}
    </Dialog>
  );
}
