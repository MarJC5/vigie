import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { Node, SiteBackupsView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sizeText } from "../../lib/data";
import { hasTag, whereOf } from "../../lib/tags";
import { Button } from "../Button/Button";
import { Checkbox } from "../Checkbox/Checkbox";
import { Dialog } from "../Dialog/Dialog";
import { ItemList } from "../ItemList/ItemList";
import styles from "./DeleteDialog.module.css";

export interface DeleteDialogProps {
  node: Node;
  /** The whole tree, depth first: what goes with the node. */
  nodes: Node[];
  /** What the sites of a site or host keep on their server (ADR-0033): offers to remove it too. */
  backups?: SiteBackupsView[];
  /** withBackups: the sites' backups on their server are removed first. */
  onDelete: (withBackups: boolean) => void;
  onClose: () => void;
}

/** Asks before removing a site, a host with its sites, or an empty folder, or a tag from every
 site (ADR-0025); the backups Vigie made on the server go too when asked (ADR-0033), nothing else
 there changes. */
export function DeleteDialog({ node, nodes, backups, onDelete, onClose }: DeleteDialogProps) {
  const { t, i18n } = useTranslation();
  const [withBackups, setWithBackups] = useState(true);
  const size = (bytes: number) => sizeText(bytes, i18n.language);
  const bySite = new Map((backups ?? []).map((b) => [b.siteId, b]));
  const offered = backups !== undefined && (node.kind === "site" || node.kind === "host");
  const read = (backups ?? []).filter((b) => b.read);
  const count = read.reduce((n, b) => n + b.count, 0);
  const bytes = read.reduce((n, b) => n + b.bytes, 0);
  const tag = node.kind === "tag";
  const children = tag ? nodes.filter((n) => n.kind === "site" && hasTag(n.tags, node.name)) : nodes.filter((n) => n.parentId === node.id);
  const kind = tag ? "Tag" : node.kind === "host" ? "Host" : node.kind === "site" ? "Site" : "Folder";
  const blocked = node.kind === "folder" && children.length > 0;
  const lead = {
    Host: t("edit.delete.leadHost", { count: children.length }),
    Site: t("edit.delete.leadSite"),
    Folder: blocked ? t("edit.delete.notEmpty") : t("edit.delete.leadFolder"),
    Tag: t("edit.delete.leadTag", { count: children.length }),
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
            <Button variant="danger" onClick={() => onDelete(offered && withBackups)}>
              {t(`edit.delete.confirm${kind}`)}
            </Button>
          )}
        </>
      }
    >
      {(node.kind === "host" || tag) && children.length > 0 && (
        <ItemList
          items={children.map((site) => {
            const b = bySite.get(site.id);
            const note = tag
              ? whereOf(site.path)
              : b?.read && b.count > 0
                ? t("edit.delete.siteBackups", { count: b.count, size: size(b.bytes) })
                : site.multisite && site.subsites > 0
                  ? t("edit.delete.network", { count: site.subsites })
                  : undefined;
            return { key: site.id, icon: "sites", name: site.name, note };
          })}
        />
      )}
      {offered && (
        <Checkbox
          label={t(node.kind === "host" ? "edit.delete.backupsHost" : "edit.delete.backupsSite")}
          note={read.length === (backups ?? []).length && read.length > 0 ? t("edit.delete.backupsNote", { count, size: size(bytes) }) : t("edit.delete.backupsNoteUnread")}
          checked={withBackups}
          onChange={setWithBackups}
        />
      )}
      {!blocked && node.kind !== "folder" && !(offered && withBackups) && (
        <p className={styles.keep}>{t(tag ? "edit.delete.keepTag" : "edit.delete.keep")}</p>
      )}
    </Dialog>
  );
}
