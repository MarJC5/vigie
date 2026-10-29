import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import type {
  HostView,
  Node,
  ScansState,
  SiteRow,
  SiteView,
} from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { Button } from "../Button/Button";
import { EmptyState } from "../EmptyState/EmptyState";
import { InfoList, InfoRow } from "../InfoList/InfoList";
import { NavRail } from "../NavRail/NavRail";
import { SearchField } from "../SearchField/SearchField";
import { SiteOverview } from "../SiteOverview/SiteOverview";
import { SitesOverview } from "../SitesOverview/SitesOverview";
import { SiteTree } from "../SiteTree/SiteTree";
import { StatusDot } from "../StatusDot/StatusDot";
import styles from "./AppShell.module.css";

/** What the detail pane reads from the Go side (SiteManager). */
export interface ShellData {
  site(id: string): Promise<SiteView>;
  host(id: string): Promise<HostView>;
  sites(nodeId: string): Promise<SiteRow[]>;
}

export interface AppShellProps {
  /** The site manager, depth first, as SiteManager.Tree returns it. */
  nodes: Node[];
  /** Opens the "add a host" wizard. */
  onAddHost?: () => void;
  /** Selected when the shell opens, for example a host just added. */
  initialSelectedId?: string;
  /** Without it, the detail pane shows what the tree knows. */
  data?: ShellData;
  scans?: ScansState;
  onScan?: (nodeId: string) => void;
  onStopAll?: () => void;
  /** Subscribes to the commands of the menu bar ("search", "scan"...); returns the unsubscribe. */
  commands?: (handler: (action: string) => void) => () => void;
}

const idle: ScansState = { running: [], completed: 0 };

/** The main window: navigation rail, site manager pane and the detail of the selected item. */
export function AppShell({
  nodes,
  onAddHost,
  initialSelectedId,
  data,
  scans = idle,
  onScan,
  onStopAll,
  commands,
}: AppShellProps) {
  const { t } = useTranslation();
  const [selectedId, setSelectedId] = useState<string | undefined>(initialSelectedId);
  const [paneOpen, setPaneOpen] = useState(true);
  const [searching, setSearching] = useState(false);
  const [query, setQuery] = useState("");
  const searchField = useRef<HTMLInputElement>(null);

  const openSearch = () => {
    setPaneOpen(true);
    setSearching(true);
    // A field already open takes the focus now; a new one takes it as it appears (autoFocus).
    searchField.current?.focus();
  };
  const closeSearch = () => {
    setSearching(false);
    setQuery("");
  };

  // The menu bar's commands; the latest handler sees the latest selection.
  const onCommand = useRef<(action: string) => void>(() => {});
  onCommand.current = (action) => {
    if (action === "search") openSearch();
    if (action === "scan" && selectedId) onScan?.(selectedId);
    if (action === "scan-all") onScan?.("");
    if (action === "new-host") onAddHost?.();
  };
  useEffect(() => commands?.((action) => onCommand.current(action)), [commands]);
  const selected = nodes.find((n) => n.id === selectedId);
  const running = scans.running ?? [];
  const left = running.reduce((n, r) => n + r.sites - r.done, 0);

  // A node is being scanned when it, one of its parents, or everything is.
  const scanning = (node: Node) => {
    const ids = new Set(running.map((r) => r.nodeId));
    if (ids.has("")) return true;
    for (let n: Node | undefined = node; n; n = nodes.find((p) => p.id === n!.parentId)) {
      if (ids.has(n.id)) return true;
    }
    return false;
  };

  return (
    <div className={styles.shell}>
      <NavRail current="sites" onAddHost={onAddHost} />
      {paneOpen && (
        <aside aria-label={t("manager.label")} className={styles.pane}>
          <div className={styles.paneHeader}>
            <h2 className={styles.paneTitle}>{t("nav.sites")}</h2>
            <Button variant="icon" icon="search" label={t("manager.search")} onClick={() => (searching ? closeSearch() : openSearch())} />
            <Button variant="icon" icon="sidebar" label={t("manager.collapse")} onClick={() => setPaneOpen(false)} />
          </div>
          {searching && (
            <div className={styles.search}>
              <SearchField
                ref={searchField}
                label={t("manager.search")}
                value={query}
                onChange={setQuery}
                onClose={closeSearch}
                autoFocus
              />
            </div>
          )}
          <div className={styles.paneBody}>
            {nodes.length > 0 ? (
              <SiteTree label={t("manager.label")} nodes={nodes} selectedId={selectedId} onSelect={setSelectedId} filter={query} />
            ) : (
              <p className={styles.note}>{t("manager.noHosts")}</p>
            )}
          </div>
          <div className={styles.paneFooter}>
            {left > 0 ? (
              <>
                <StatusDot status="unknown" label={t("manager.running", { count: left })} />
                <span className={styles.running}>{t("manager.running", { count: left })}</span>
                {onStopAll && (
                  <Button variant="text" onClick={onStopAll}>
                    {t("manager.stopAll")}
                  </Button>
                )}
              </>
            ) : (
              selected && <Summary node={selected} nodes={nodes} />
            )}
          </div>
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
          data ? (
            <section className={styles.detail}>
              <Loaded
                node={selected}
                data={data}
                revision={scans.completed}
                scanning={scanning(selected)}
                onScan={() => onScan?.(selected.id)}
                onSelect={setSelectedId}
              />
            </section>
          ) : (
            <Detail node={selected} />
          )
        ) : (
          <p className={styles.note}>{t("detail.select")}</p>
        )}
      </main>
    </div>
  );
}

type View = { kind: "site"; site: SiteView } | { kind: "host"; host: HostView } | { kind: "folder"; rows: SiteRow[] };

/** The detail of a node, read from the Go side, again whenever a scan completes. */
function Loaded({
  node,
  data,
  revision,
  scanning,
  onScan,
  onSelect,
}: {
  node: Node;
  data: ShellData;
  revision: number;
  scanning: boolean;
  onScan: () => void;
  onSelect: (id: string) => void;
}) {
  const { t } = useTranslation();
  const [view, setView] = useState<{ id: string; view: View }>();

  useEffect(() => {
    let current = true;
    const load = async (): Promise<View> => {
      switch (node.kind) {
        case "site":
          return { kind: "site", site: await data.site(node.id) };
        case "host":
          return { kind: "host", host: await data.host(node.id) };
      }
      return { kind: "folder", rows: await data.sites(node.id) };
    };
    void load().then((v) => {
      if (current) setView({ id: node.id, view: v });
    });
    return () => {
      current = false;
    };
  }, [node.id, node.kind, data, revision]);

  // Nothing until the first answer, which comes at once from the local database.
  if (!view || view.id !== node.id) return null;
  const v = view.view;
  if (v.kind === "site") return <SiteOverview site={v.site} scanning={scanning} onScan={onScan} />;
  if (v.kind === "host") {
    const h = v.host;
    const subtitle = h.alias
      ? t("host.alias", { alias: h.alias, wpCli: h.wpCli })
      : t("host.connection", { account: `${h.user}@${h.hostname}`, port: h.port, wpCli: h.wpCli });
    return (
      <SitesOverview
        title={h.name}
        icon="host"
        subtitle={subtitle}
        rows={h.sites ?? []}
        scanLabel={t("action.scanHost")}
        scanning={scanning}
        onScan={onScan}
        onSelect={onSelect}
      />
    );
  }
  return (
    <SitesOverview
      title={node.name}
      icon="folder"
      rows={v.rows}
      scanLabel={t("action.scanNow")}
      scanning={scanning}
      onScan={onScan}
      onSelect={onSelect}
    />
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
