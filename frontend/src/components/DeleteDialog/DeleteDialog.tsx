import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { Node, SiteBackupsView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sizeText } from "../../lib/data";
import { deletion, sitesOf } from "../../lib/selection";
import { hasTag, whereOf } from "../../lib/tags";
import { Button } from "../Button/Button";
import { Checkbox } from "../Checkbox/Checkbox";
import { Dialog } from "../Dialog/Dialog";
import { ItemList } from "../ItemList/ItemList";
import { Notice } from "../Notice/Notice";
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

export interface DeleteManyDialogProps {
  /** The items selected. */
  ids: string[];
  /** The whole tree, depth first. */
  nodes: Node[];
  /** What the sites of the sites and hosts that go keep on their server (ADR-0033). */
  backups?: SiteBackupsView[];
  /** going: the items to delete, the folders left empty among them. */
  onDelete: (going: string[], withBackups: boolean) => void;
  onClose: () => void;
}

/** Asks before removing several items at once (ADR-0034): the sites, the hosts with their sites,
 the folders left empty, and says which folders stay because they hold something else. */
export function DeleteManyDialog({ ids, nodes, backups, onDelete, onClose }: DeleteManyDialogProps) {
  const { t, i18n } = useTranslation();
  const [withBackups, setWithBackups] = useState(true);
  const size = (bytes: number) => sizeText(bytes, i18n.language);
  const { going, shown, staying } = deletion(ids, nodes);
  const sites = going.some((n) => n.kind === "site" || n.kind === "host");
  const offered = backups !== undefined && sites;
  const read = (backups ?? []).filter((b) => b.read);
  const count = read.reduce((n, b) => n + b.count, 0);
  const bytes = read.reduce((n, b) => n + b.bytes, 0);
  const names = staying.map((n) => t("selection.delete.quoted", { name: n.name })).join(", ");

  return (
    <Dialog
      title={t("selection.delete.title", { count: shown.length || ids.length })}
      lead={shown.length > 0 ? t("selection.delete.lead") : t("edit.delete.notEmpty")}
      onClose={onClose}
      actions={
        <>
          <Button variant="outline" onClick={onClose}>
            {t("edit.cancel")}
          </Button>
          {shown.length > 0 && (
            <Button variant="danger" onClick={() => onDelete(going.map((n) => n.id), offered && withBackups)}>
              {t("selection.delete.confirm", { count: shown.length })}
            </Button>
          )}
        </>
      }
    >
      {shown.length > 0 && (
        <ItemList
          items={shown.map((n) => ({
            key: n.id,
            icon: n.kind === "host" ? "host" : n.kind === "folder" ? "folder" : "sites",
            name: n.name,
            note:
              n.kind === "host"
                ? t("selection.sites", { count: sitesOf([n.id], nodes).length })
                : n.kind === "folder"
                  ? t("selection.delete.empty")
                  : whereOf(n.path),
          }))}
        />
      )}
      {staying.length > 0 && shown.length > 0 && (
        <Notice tone="neutral" role="note" text={t("selection.delete.stays", { count: staying.length, names })} />
      )}
      {offered && (
        <Checkbox
          label={t("edit.delete.backupsHost")}
          note={read.length === (backups ?? []).length && read.length > 0 ? t("edit.delete.backupsNote", { count, size: size(bytes) }) : t("edit.delete.backupsNoteUnread")}
          checked={withBackups}
          onChange={setWithBackups}
        />
      )}
      {sites && !(offered && withBackups) && <p className={styles.keep}>{t("edit.delete.keep")}</p>}
    </Dialog>
  );
}
