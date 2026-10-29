import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { Node } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { Button } from "../Button/Button";
import { EmptyState } from "../EmptyState/EmptyState";
import { InfoList, InfoRow } from "../InfoList/InfoList";
import { NavRail } from "../NavRail/NavRail";
import { SiteTree } from "../SiteTree/SiteTree";
import styles from "./AppShell.module.css";

export interface AppShellProps {
  /** The site manager, depth first, as SiteManager.Tree returns it. */
  nodes: Node[];
  /** Opens the "add a host" wizard. */
  onAddHost?: () => void;
  /** Selected when the shell opens, for example a host just added. */
  initialSelectedId?: string;
}

/** The main window: navigation rail, site manager pane and the detail of the selected item. */
export function AppShell({ nodes, onAddHost, initialSelectedId }: AppShellProps) {
  const { t } = useTranslation();
  const [selectedId, setSelectedId] = useState<string | undefined>(initialSelectedId);
  const [paneOpen, setPaneOpen] = useState(true);
  const selected = nodes.find((n) => n.id === selectedId);

  return (
    <div className={styles.shell}>
      <NavRail current="sites" onAddHost={onAddHost} />
      {paneOpen && (
        <aside aria-label={t("manager.label")} className={styles.pane}>
          <div className={styles.paneHeader}>
            <h2 className={styles.paneTitle}>{t("nav.sites")}</h2>
            <Button variant="icon" icon="sidebar" label={t("manager.collapse")} onClick={() => setPaneOpen(false)} />
          </div>
          <div className={styles.paneBody}>
            {nodes.length > 0 ? (
              <SiteTree label={t("manager.label")} nodes={nodes} selectedId={selectedId} onSelect={setSelectedId} />
            ) : (
              <p className={styles.note}>{t("manager.noHosts")}</p>
            )}
          </div>
          <div className={styles.paneFooter}>{selected && <Summary node={selected} nodes={nodes} />}</div>
        </aside>
      )}
      {/* Focusable so the keyboard can scroll a long detail, WebKit included. */}
      <main className={styles.main} tabIndex={0}>
        {!paneOpen && (
          <Button variant="icon" icon="sidebar" label={t("manager.expand")} onClick={() => setPaneOpen(true)} />
        )}
        {nodes.length === 0 ? (
          <EmptyState title={t("empty.title")} text={t("empty.text")}>
            <Button variant="primary" icon="plus" disabled={!onAddHost} onClick={onAddHost}>
              {t("action.addHost")}
            </Button>
          </EmptyState>
        ) : selected ? (
          <Detail node={selected} />
        ) : (
          <p className={styles.note}>{t("detail.select")}</p>
        )}
      </main>
    </div>
  );
}

function Detail({ node }: { node: Node }) {
  const { t } = useTranslation();
  return (
    <section className={styles.detail}>
      <h1 className={styles.title}>{node.name}</h1>
      <InfoList>
        <InfoRow label={t("detail.kind")}>{t(node.multisite ? "kind.network" : `kind.${node.kind}`)}</InfoRow>
        <InfoRow label={t("detail.location")}>{node.path}</InfoRow>
        {/* Subsites are read at the first scan of a network. */}
        {node.multisite && node.subsites > 0 && <InfoRow label={t("detail.subsites")}>{node.subsites}</InfoRow>}
        {node.kind === "site" && <InfoRow label={t("detail.lastScan")}>{t("detail.notScanned")}</InfoRow>}
      </InfoList>
    </section>
  );
}

/** The bottom line of the pane: the selected folder or host and how many sites it holds. */
function Summary({ node, nodes }: { node: Node; nodes: Node[] }) {
  const { t } = useTranslation();
  if (node.kind === "site") return null;
  const prefix = `${node.path}/`;
  const count = nodes.filter((n) => n.kind === "site" && n.path.startsWith(prefix)).length;
  return <span>{t("bottom.sites", { name: node.name, count })}</span>;
}
