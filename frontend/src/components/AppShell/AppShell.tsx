import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import type {
  CreateResult,
  HostView,
  Node,
  OperationView,
  Problem,
  ScansState,
  SitePreviewView,
  SiteResultView,
  SiteRow,
  SiteView,
  UpdatesState,
} from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { namesOf } from "../../lib/updates";
import { BulkDialog } from "../BulkDialog/BulkDialog";
import { Button } from "../Button/Button";
import { DeleteDialog } from "../DeleteDialog/DeleteDialog";
import { EmptyState } from "../EmptyState/EmptyState";
import type { IconName } from "../Icon/Icon";
import { type FindSitesApi, FindSitesDialog } from "../FindSitesDialog/FindSitesDialog";
import { HostDialog, type HostEditApi } from "../HostDialog/HostDialog";
import { InfoList, InfoRow } from "../InfoList/InfoList";
import { type ColorChoice, Menu, type MenuEntry } from "../Menu/Menu";
import { MoveDialog } from "../MoveDialog/MoveDialog";
import { NavRail } from "../NavRail/NavRail";
import { SearchField } from "../SearchField/SearchField";
import { SiteOverview } from "../SiteOverview/SiteOverview";
import { SitesOverview } from "../SitesOverview/SitesOverview";
import { type Drag, SiteTree } from "../SiteTree/SiteTree";
import { StatusDot } from "../StatusDot/StatusDot";
import { UpdateFlow, type UpdatesApi } from "../UpdateFlow/UpdateFlow";
import styles from "./AppShell.module.css";

/** What the detail pane reads from the Go side (SiteManager). */
export interface ShellData {
  site(id: string): Promise<SiteView>;
  host(id: string): Promise<HostView>;
  sites(nodeId: string): Promise<SiteRow[]>;
  /** The updates and rollbacks of a site, the newest first; without it, no History tab. */
  history?(siteId: string): Promise<OperationView[]>;
}

/** The Go services that edit the site manager (app.Editor, and app.HostWizard for a stored host). */
export interface EditApi extends HostEditApi, FindSitesApi {
  CreateFolder(parentId: string, name: string): Promise<CreateResult>;
  Rename(id: string, name: string): Promise<Problem | null>;
  Move(id: string, folderId: string): Promise<Problem | null>;
  SetColor(id: string, color: string): Promise<Problem | null>;
  Delete(id: string): Promise<Problem | null>;
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
  /** Without it, the tree is read only. */
  edit?: EditApi;
  /** Reads the tree again after an edit. */
  onChanged?: () => Promise<void> | void;
  updates?: UpdatesState;
  /** Without it, sites are not updated from the window. */
  updatesApi?: UpdatesApi;
}

const idle: ScansState = { running: [], completed: 0 };
const noUpdates: UpdatesState = { running: [], active: 0, bulk: [], completed: 0, sites: [] };

/** The update of a site open in the window. */
interface Flow {
  site: { id: string; name: string };
  keys: Set<string>;
  names: Map<string, string>;
  follow: boolean;
}

type DialogKind = "move" | "delete" | "host" | "find";

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
  edit,
  onChanged,
  updates = noUpdates,
  updatesApi,
}: AppShellProps) {
  const { t } = useTranslation();
  const [selectedId, setSelectedId] = useState<string | undefined>(initialSelectedId);
  const [paneOpen, setPaneOpen] = useState(true);
  const [searching, setSearching] = useState(false);
  const [query, setQuery] = useState("");
  const searchField = useRef<HTMLInputElement>(null);
  const shell = useRef<HTMLDivElement>(null);
  const [menu, setMenu] = useState<{ id: string; x: number; y: number } | null>(null);
  const [editingId, setEditingId] = useState<string>();
  const [dialog, setDialog] = useState<{ kind: DialogKind; id: string } | null>(null);
  const [problem, setProblem] = useState<Problem | null>(null);
  const [drag, setDrag] = useState<Drag | null>(null);
  const [flow, setFlow] = useState<Flow | null>(null);
  const [bulkDialog, setBulkDialog] = useState<{ id: string; name: string; previews: SitePreviewView[] | null } | null>(null);
  // The host or folder whose last update of every site shows, until dismissed.
  const [bulkFor, setBulkFor] = useState<string>();

  const byId = new Map(nodes.map((n) => [n.id, n]));
  const say = (p: Problem) => t(p.code, { message: p.detail });

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

  // Runs an edit on the Go side, then reads the tree again; a refusal shows in the pane's footer.
  const run = async (action: () => Promise<Problem | null>) => {
    setProblem(null);
    const refused = await action();
    if (refused) setProblem(refused);
    await onChanged?.();
  };

  // The folder a new folder goes in: the selected folder, else the folder of the selection.
  const folderOf = (id: string | undefined): string => {
    const n = id ? byId.get(id) : undefined;
    if (!n) return "";
    if (n.kind === "folder") return n.id;
    if (n.kind === "host") return n.parentId;
    return byId.get(n.parentId)?.parentId ?? "";
  };

  const createFolder = async (parentId: string) => {
    if (!edit) return;
    setProblem(null);
    const taken = new Set(nodes.filter((n) => n.parentId === parentId && n.kind !== "site").map((n) => n.name.toLowerCase()));
    const base = t("edit.newFolder");
    let name = base;
    for (let i = 2; taken.has(name.toLowerCase()); i++) name = `${base} ${i}`;
    const created = await edit.CreateFolder(parentId, name);
    if (created.problem) return setProblem(created.problem);
    await onChanged?.();
    setSelectedId(created.id);
    setEditingId(created.id);
  };

  // The menu bar's commands; the latest handler sees the latest selection.
  const onCommand = useRef<(action: string) => void>(() => {});
  useEffect(() => {
    onCommand.current = (action) => {
      if (action === "search") openSearch();
      if (action === "scan" && selectedId) scan(selectedId);
      if (action === "scan-all") scan("");
      if (action === "new-host") onAddHost?.();
      if (action === "new-folder") void createFolder(folderOf(selectedId));
    };
  });
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

  // A node with a site in an update not over yet: it cannot be scanned or updated.
  const busySites = new Set(updates.sites ?? []);
  const updating = (node: Node) =>
    node.kind === "site" ? busySites.has(node.id) : nodes.some((n) => busySites.has(n.id) && n.path.startsWith(`${node.path}/`));
  const scan = (id: string) => {
    const node = id ? byId.get(id) : undefined;
    if (id ? node && !updating(node) : busySites.size === 0) onScan?.(id);
  };

  const updateAll = (node: Node) => {
    if (!updatesApi) return;
    setBulkDialog({ id: node.id, name: node.name, previews: null });
    void updatesApi.Preview(node.id).then((previews) => {
      setBulkDialog((d) => (d && d.id === node.id ? { ...d, previews: previews ?? [] } : d));
    });
  };
  const confirmBulk = async () => {
    if (!updatesApi || !bulkDialog) return;
    const id = bulkDialog.id;
    setBulkDialog(null);
    setProblem(null);
    const refused = await updatesApi.UpdateAll(id);
    if (refused) return setProblem(refused);
    setBulkFor(id);
  };

  // Opens the menu of a node at a point of the window; "" is the site list itself (its empty space).
  // A row of a table keeps the page shown; the tree selects what it opens the menu of.
  const openMenu = (id: string, point: { x: number; y: number }, select = true) => {
    const rect = shell.current?.getBoundingClientRect();
    if (select && id) setSelectedId(id);
    setMenu({ id, x: point.x - (rect?.left ?? 0), y: point.y - (rect?.top ?? 0) });
  };

  const choose = (id: string, action: string) => {
    setMenu(null);
    if (action === "addHost") return onAddHost?.();
    if (action === "scanAll") return scan("");
    if (action === "scan") scan(id);
    // In the folder itself, or next to a host or site; at the top level from the empty space.
    if (action === "newFolder") void createFolder(folderOf(id));
    if (action === "rename") setEditingId(id);
    if (action === "move" || action === "delete" || action === "host" || action === "find") setDialog({ kind: action, id });
  };

  const menuNode = menu ? byId.get(menu.id) : undefined;
  const dialogNode = dialog ? byId.get(dialog.id) : undefined;

  // What the pane's footer says: a refusal first, then a drag, a rename, the scans, the selection.
  const footer = () => {
    if (problem) {
      return (
        <span role="alert" className={styles.problem}>
          {say(problem)}
        </span>
      );
    }
    const dragged = drag && byId.get(drag.id);
    if (dragged && drag.target !== undefined) {
      const text = drag.target
        ? t("edit.dropInto", { name: dragged.name, folder: byId.get(drag.target)?.name })
        : t("edit.dropAtRoot", { name: dragged.name });
      return <span className={styles.running}>{text}</span>;
    }
    if (editingId) return <span className={styles.running}>{t("edit.renameHint")}</span>;
    if (busySites.size > 0) {
      return (
        <>
          <StatusDot status="unknown" label={t("manager.updating", { count: busySites.size })} />
          <span className={styles.running}>{t("manager.updating", { count: busySites.size })}</span>
          {updatesApi && (
            <Button variant="text" onClick={() => void updatesApi.CancelAll()}>
              {t("manager.stopAll")}
            </Button>
          )}
        </>
      );
    }
    if (left > 0) {
      return (
        <>
          <StatusDot status="unknown" label={t("manager.running", { count: left })} />
          <span className={styles.running}>{t("manager.running", { count: left })}</span>
          {onStopAll && (
            <Button variant="text" onClick={onStopAll}>
              {t("manager.stopAll")}
            </Button>
          )}
        </>
      );
    }
    return selected && <Summary node={selected} nodes={nodes} />;
  };

  const inert = (dialog !== null || bulkDialog !== null) || undefined;

  // The update of a site takes the whole window; the shell keeps its state behind it.
  if (flow && updatesApi) {
    return (
      <UpdateFlow
        api={updatesApi}
        site={flow.site}
        keys={flow.keys}
        names={flow.names}
        state={updates}
        follow={flow.follow}
        onClose={() => setFlow(null)}
      />
    );
  }

  return (
    <div ref={shell} className={styles.shell}>
      <NavRail current="sites" onAddHost={onAddHost} inert={inert} />
      {paneOpen && (
        <aside aria-label={t("manager.label")} className={styles.pane} inert={inert}>
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
          <div
            className={styles.paneBody}
            data-pane-body
            onContextMenu={(event) => {
              // A node handled its own right-click; the empty space opens the list's menu.
              if (!edit || event.defaultPrevented) return;
              event.preventDefault();
              openMenu("", { x: event.clientX, y: event.clientY });
            }}
          >
            {nodes.length > 0 ? (
              <SiteTree
                label={t("manager.label")}
                nodes={nodes}
                selectedId={selectedId}
                onSelect={setSelectedId}
                filter={query}
                {...(edit && {
                  editingId,
                  onEdit: setEditingId,
                  onRename: (id: string, name: string) => {
                    setEditingId(undefined);
                    void run(() => edit.Rename(id, name));
                  },
                  onCancelEdit: () => setEditingId(undefined),
                  onDelete: (id: string) => setDialog({ kind: "delete", id }),
                  onMenu: openMenu,
                  onMove: (id: string, folderId: string) => void run(() => edit.Move(id, folderId)),
                  onDragChange: setDrag,
                })}
              />
            ) : (
              <p className={styles.note}>{t("manager.noHosts")}</p>
            )}
          </div>
          <div className={styles.paneFooter}>{footer()}</div>
        </aside>
      )}
      {/* Focusable so the keyboard can scroll a long detail, WebKit included. */}
      <main className={styles.main} tabIndex={0} inert={inert}>
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
                revision={scans.completed + updates.completed}
                scanning={scanning(selected)}
                onScan={() => scan(selected.id)}
                onSelect={setSelectedId}
                onMore={edit && ((point) => openMenu(selected.id, point))}
                onSiteMenu={edit && ((id, point) => openMenu(id, point, false))}
                onRemove={edit && (() => setDialog({ kind: "delete", id: selected.id }))}
                updating={updating(selected)}
                {...(updatesApi && {
                  onUpdate: (site: SiteView, keys: Set<string>) =>
                    setFlow({ site: { id: site.id, name: site.name }, keys, names: namesOf(site.scan?.updates ?? []), follow: false }),
                  onFollow: (site: SiteView) =>
                    setFlow({ site: { id: site.id, name: site.name }, keys: new Set(), names: namesOf(site.scan?.updates ?? []), follow: true }),
                  // The site turns busy at once; its page then offers to follow the rollback.
                  onRollback: (site: SiteView, operationID: string) => {
                    setProblem(null);
                    void updatesApi.Rollback(site.id, operationID).then((p) => p && setProblem(p));
                  },
                  onUpdateAll: () => updateAll(selected),
                  bulk: bulkFor === selected.id ? (updates.bulk ?? []) : undefined,
                  onDismissBulk: () => setBulkFor(undefined),
                })}
              />
            </section>
          ) : (
            <Detail node={selected} />
          )
        ) : (
          <p className={styles.note}>{t("detail.select")}</p>
        )}
      </main>

      {menu && menu.id === "" && (
        <Menu
          label={t("manager.label")}
          entries={rootEntries(t, !!onAddHost, nodes.length > 0)}
          x={menu.x}
          y={menu.y}
          onSelect={(action) => choose("", action)}
          onColor={() => setMenu(null)}
          onClose={() => setMenu(null)}
        />
      )}
      {menu && menuNode && (
        <Menu
          label={t("edit.menu.label", { name: menuNode.name })}
          entries={menuEntries(menuNode, nodes, t)}
          x={menu.x}
          y={menu.y}
          onSelect={(action) => choose(menuNode.id, action)}
          onColor={(color: ColorChoice) => {
            setMenu(null);
            void run(() => edit!.SetColor(menuNode.id, color));
          }}
          onClose={() => setMenu(null)}
        />
      )}

      {bulkDialog && (
        <BulkDialog
          name={bulkDialog.name}
          previews={bulkDialog.previews}
          onConfirm={() => void confirmBulk()}
          onClose={() => setBulkDialog(null)}
        />
      )}
      {edit && dialog && dialogNode && dialog.kind === "move" && (
        <MoveDialog
          node={dialogNode}
          nodes={nodes}
          onMove={(folderId) => {
            setDialog(null);
            void run(() => edit.Move(dialogNode.id, folderId));
          }}
          onClose={() => setDialog(null)}
        />
      )}
      {edit && dialog && dialogNode && dialog.kind === "delete" && (
        <DeleteDialog
          node={dialogNode}
          nodes={nodes}
          onDelete={() => {
            setDialog(null);
            // The selection goes when it was the node or under it.
            for (let n = selected; n; n = byId.get(n.parentId)) if (n.id === dialogNode.id) setSelectedId(undefined);
            void run(() => edit.Delete(dialogNode.id));
          }}
          onClose={() => setDialog(null)}
        />
      )}
      {edit && dialog && dialogNode && dialog.kind === "host" && (
        <HostDialog
          api={edit}
          hostId={dialogNode.id}
          sites={nodes.filter((n) => n.parentId === dialogNode.id).length}
          onClose={() => setDialog(null)}
          onSaved={() => {
            setDialog(null);
            void onChanged?.();
          }}
        />
      )}
      {edit && dialog && dialogNode && dialog.kind === "find" && (
        <FindSitesDialog
          api={edit}
          hostId={dialogNode.id}
          hostName={dialogNode.name}
          onClose={() => setDialog(null)}
          onAdded={() => {
            setDialog(null);
            void onChanged?.();
          }}
        />
      )}
    </div>
  );
}

type T = (key: string, vars?: Record<string, unknown>) => string;

type Item = Extract<MenuEntry, { kind: "item" }>;

// The actions of the site list itself, from its empty space.
function rootEntries(t: T, canAddHost: boolean, hasSites: boolean): MenuEntry[] {
  const entries: MenuEntry[] = [
    { kind: "item", id: "newFolder", label: t("edit.menu.newFolder"), icon: "folder", shortcut: "⇧⌘N", keyshortcuts: "Shift+Meta+N" },
  ];
  if (canAddHost) entries.push({ kind: "item", id: "addHost", label: t("edit.menu.addHost"), icon: "plus", shortcut: "⌘N", keyshortcuts: "Meta+N" });
  if (hasSites) entries.push({ kind: "separator" }, { kind: "item", id: "scanAll", label: t("edit.menu.scanAll"), icon: "refresh" });
  return entries;
}

// The actions on a node, as the menu lists them.
function menuEntries(node: Node, nodes: Node[], t: T): MenuEntry[] {
  const item = (id: string, label: string, icon: IconName, extra: Partial<Item> = {}): Item => ({ kind: "item", id, label, icon, ...extra });
  const colors: MenuEntry = {
    kind: "colors",
    label: t("edit.menu.color"),
    value: node.ownColor as ColorChoice,
    names: Object.fromEntries(
      (["none", "red", "green", "blue", "yellow", "cyan", "magenta", "orange"] as const).map((c) => [c, t(`edit.menu.colors.${c}`)]),
    ) as Record<ColorChoice, string>,
  };
  const separator: MenuEntry = { kind: "separator" };
  const rename = item("rename", t("edit.menu.rename"), "rename", { shortcut: "↩", keyshortcuts: "Enter" });
  const move = item("move", t("edit.menu.move"), "move");
  const scan = item("scan", t("edit.menu.scan"), "refresh", { shortcut: "⌘R", keyshortcuts: "Meta+R" });
  const remove = (key: string, disabled = false) =>
    item("delete", t(key), "trash", { danger: true, shortcut: "⌘⌫", keyshortcuts: "Meta+Backspace", disabled });
  const newFolder = item("newFolder", t("edit.menu.newFolder"), "folder", { shortcut: "⇧⌘N", keyshortcuts: "Shift+Meta+N" });
  if (node.kind === "site") {
    return [scan, newFolder, separator, rename, colors, separator, remove("edit.menu.deleteSite")];
  }
  if (node.kind === "host") {
    return [
      scan, newFolder, separator, rename, move, colors,
      item("host", t("edit.menu.edit"), "host"), item("find", t("edit.menu.find"), "findSites"),
      separator, remove("edit.menu.deleteHost"),
    ];
  }
  // Only an empty folder can be deleted.
  const empty = !nodes.some((n) => n.parentId === node.id);
  return [
    scan, newFolder,
    separator, rename, move, colors, separator, remove("edit.menu.deleteFolder", !empty),
  ];
}

/** The detail of a node, read from the Go side, again whenever a scan completes. */
function Loaded({
  node,
  data,
  revision,
  scanning,
  onScan,
  onSelect,
  onMore,
  onSiteMenu,
  onRemove,
  updating,
  onUpdate,
  onFollow,
  onRollback,
  onUpdateAll,
  bulk,
  onDismissBulk,
}: {
  node: Node;
  data: ShellData;
  revision: number;
  scanning: boolean;
  onScan: () => void;
  onSelect: (id: string) => void;
  onMore?: (point: { x: number; y: number }) => void;
  onSiteMenu?: (id: string, point: { x: number; y: number }) => void;
  onRemove?: () => void;
  updating: boolean;
  onUpdate?: (site: SiteView, keys: Set<string>) => void;
  onFollow?: (site: SiteView) => void;
  onRollback?: (site: SiteView, operationID: string) => void;
  onUpdateAll?: () => void;
  bulk?: SiteResultView[];
  onDismissBulk?: () => void;
}) {
  const { t } = useTranslation();
  const [view, setView] = useState<{ id: string; view: View }>();

  useEffect(() => {
    let current = true;
    const load = async (): Promise<View> => {
      switch (node.kind) {
        case "site": {
          const [site, history] = await Promise.all([data.site(node.id), data.history?.(node.id)]);
          return { kind: "site", site, history };
        }
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
    // The name too: a rename shows at once.
  }, [node.id, node.kind, node.name, data, revision]);

  // Nothing until the first answer, which comes at once from the local database.
  if (!view || view.id !== node.id) return null;
  const v = view.view;
  if (v.kind === "site") {
    const site = v.site;
    return (
      <SiteOverview
        site={site}
        scanning={scanning}
        onScan={onScan}
        onMore={onMore}
        updating={updating}
        onUpdate={onUpdate && ((keys) => onUpdate(site, keys))}
        onFollow={onFollow && (() => onFollow(site))}
        history={v.history}
        onRollback={onRollback && ((id) => onRollback(site, id))}
        onRemove={onRemove}
      />
    );
  }
  const bulkProps = { onUpdateAll, updating, bulk, onDismissBulk, onSiteMenu };
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
        onMore={onMore}
        {...bulkProps}
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
      onMore={onMore}
      {...bulkProps}
    />
  );
}

type View = { kind: "site"; site: SiteView; history?: OperationView[] } | { kind: "host"; host: HostView } | { kind: "folder"; rows: SiteRow[] };

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
