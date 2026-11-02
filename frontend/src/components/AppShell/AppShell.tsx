import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import type {
  BackupsView,
  BackupView,
  CreateResult,
  DeleteResult,
  ExtensionsAcross,
  FindingView,
  HostDetails,
  HostView,
  Node,
  OperationView,
  Problem,
  ScansState,
  SitePreviewView,
  SiteBackupsView,
  SiteResultView,
  SiteRow,
  SiteView,
  TagView,
  TreeView,
  UpdatesState,
} from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { clock, day } from "../../lib/findings";
import { openKey, subsiteId, subsiteOf, withSubsites } from "../../lib/subsites";
import { hasTag, spread, tagId, tagOf, tagsGroupId } from "../../lib/tags";
import { fold } from "../../lib/text";
import { type Choice, knownOf, namesOf } from "../../lib/updates";
import { AdminAddressDialog } from "../AdminAddressDialog/AdminAddressDialog";
import { BackupsLeftDialog } from "../BackupsLeftDialog/BackupsLeftDialog";
import { BulkDialog } from "../BulkDialog/BulkDialog";
import { Button } from "../Button/Button";
import { DeleteDialog } from "../DeleteDialog/DeleteDialog";
import { RemoveBackupsDialog } from "../RemoveBackupsDialog/RemoveBackupsDialog";
import { EmptyState } from "../EmptyState/EmptyState";
import { Icon, type IconName } from "../Icon/Icon";
import { type FindSitesApi, FindSitesDialog } from "../FindSitesDialog/FindSitesDialog";
import { HostDialog, type HostEditApi } from "../HostDialog/HostDialog";
import { HostTestDialog } from "../HostTestDialog/HostTestDialog";
import { IgnoreDialog } from "../IgnoreDialog/IgnoreDialog";
import { InfoList, InfoRow } from "../InfoList/InfoList";
import { type ColorChoice, Menu, type MenuEntry } from "../Menu/Menu";
import { MoveDialog } from "../MoveDialog/MoveDialog";
import { NavRail, type Section } from "../NavRail/NavRail";
import { RenameTagDialog } from "../RenameTagDialog/RenameTagDialog";
import { SearchField } from "../SearchField/SearchField";
import { SiteOverview } from "../SiteOverview/SiteOverview";
import { SitesOverview } from "../SitesOverview/SitesOverview";
import { carriedKey, type Drag, SiteTree } from "../SiteTree/SiteTree";
import { StatusDot } from "../StatusDot/StatusDot";
import { SubsitePage } from "../SubsitePage/SubsitePage";
import { UpdateFlow, type UpdatesApi } from "../UpdateFlow/UpdateFlow";
import styles from "./AppShell.module.css";

/** What the detail pane reads from the Go side (SiteManager). */
export interface ShellData {
  site(id: string): Promise<SiteView>;
  host(id: string): Promise<HostView>;
  sites(nodeId: string): Promise<SiteRow[]>;
  /** The updates and rollbacks of a site, the newest first; without it, no History tab. */
  history?(siteId: string): Promise<OperationView[]>;
  /** A site's backups on its server (ADR-0033), shown in its History tab. */
  backups?(siteId: string): Promise<BackupsView>;
  /** Removes a site's backups past the limits; without it, they are only listed. */
  removeOldBackups?(siteId: string): Promise<Problem | null>;
  /** The plugins and themes across the sites of a host or folder; without it, no Plugins tab. */
  extensions?(nodeId: string): Promise<ExtensionsAcross>;
}

/** The Go service that sets findings aside (app.Security). */
export interface SecurityApi {
  Ignore(siteId: string, key: string, untilVersion: string, reason: string): Promise<Problem | null>;
  Unignore(siteId: string, key: string): Promise<Problem | null>;
}

/** The Go services that edit the site manager (app.Editor, and app.HostWizard for a stored host). */
export interface EditApi extends HostEditApi, FindSitesApi {
  CreateFolder(parentId: string, name: string): Promise<CreateResult>;
  Rename(id: string, name: string): Promise<Problem | null>;
  Move(id: string, folderId: string): Promise<Problem | null>;
  /** The folders and hosts among ids into a folder, all or none (ADR-0034). */
  MoveMany(ids: string[], folderId: string): Promise<Problem | null>;
  SetColor(id: string, color: string): Promise<Problem | null>;
  /** One item or several, all or none; withBackups: the sites' backups on their server are removed
   first (ADR-0033). */
  Delete(ids: string[], withBackups: boolean): Promise<DeleteResult>;
  /** What the sites of sites and hosts keep on their server, for the delete dialog. */
  Backups(ids: string[]): Promise<SiteBackupsView[] | null>;
  SetAdminURL(id: string, address: string): Promise<Problem | null>;
  SetNotes(id: string, notes: string): Promise<Problem | null>;
  SetTags(id: string, tags: string[]): Promise<Problem | null>;
  RenameTag(from: string, to: string): Promise<Problem | null>;
  RemoveTag(tag: string): Promise<Problem | null>;
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
  /** The section shown: the sites, or the settings page given. */
  section?: Section;
  onSection?: (section: Section) => void;
  settingsPage?: ReactNode;
  /** The Add-ons page, in the place of the site list and the detail. */
  addonsPage?: ReactNode;
  /** The Activity page, shown in place of the site list and the detail. */
  activityPage?: ReactNode;
  /** The Help page, shown in place of the site list and the detail. */
  helpPage?: ReactNode;
  /** Scans a node comparing its files again, even within the day. */
  onCheckFiles?: (nodeId: string) => void;
  /** "Export…" in the menu of a folder or host. */
  onExport?: (nodeId: string) => void;
  /** Without it, findings are listed but not set aside. */
  security?: SecurityApi;
  /** Opens an address in the browser. */
  onOpen?: (href: string) => void;
  /** Puts a text on the clipboard (the SSH command of a site). */
  onCopy?: (text: string) => Promise<unknown> | void;
  /** The system opens ssh:// addresses in a terminal. */
  canOpenSSH?: boolean;
  /** When the next automatic scan is due (RFC 3339), at the foot of the site list; none while off. */
  nextScan?: string;
  /** The tags in use, with how many sites have each (SiteManager.Tree). */
  tags?: TagView[];
  /** How the user left the site list (Preferences.TreeView); told of each change. */
  treeView?: TreeView;
  onTreeView?: (view: TreeView) => void;
}

const idle: ScansState = { running: [], completed: 0 };
const noUpdates: UpdatesState = { running: [], active: 0, bulk: [], completed: 0, sites: [] };

/** The update of a site open in the window. */
interface Flow {
  site: { id: string; name: string };
  keys: Set<string>;
  names: Map<string, string>;
  /** The plugins wordpress.org knows, for their changelogs. */
  known: Set<string>;
  follow: boolean;
}

type DialogKind = "move" | "delete" | "host" | "find" | "renameTag";

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
  section = "sites",
  onSection,
  settingsPage,
  addonsPage,
  activityPage,
  helpPage,
  onCheckFiles,
  onExport,
  security,
  onOpen,
  onCopy,
  canOpenSSH,
  nextScan,
  tags,
  treeView,
  onTreeView,
}: AppShellProps) {
  const { t } = useTranslation();
  // Something to show at once: the item asked for, else the first host, else the first item.
  // Several items can be selected (ADR-0034); selectedId is the one when there is one.
  const [selection, setSelection] = useState<string[]>(() => {
    const first = initialSelectedId ?? (nodes.find((n) => n.kind === "host") ?? nodes[0])?.id;
    return first ? [first] : [];
  });
  const selectedId = selection.length === 1 ? selection[0] : undefined;
  const setSelectedId = (id: string | undefined) => setSelection(id ? [id] : []);
  // The list hidden and the folders folded, as the user left them; each change kept for next time,
  // without the items deleted since.
  const [paneOpen, setPaneOpen] = useState(!treeView?.hidden);
  const [folded, setFolded] = useState(() => new Set(treeView?.folded ?? []));
  const leave = (open: boolean, next: Set<string>) => {
    setPaneOpen(open);
    setFolded(next);
    // The networks unfolded (openKey) are kept too, for the networks still there.
    const kept = nodes.flatMap((n) => [n.id, openKey(n.id)]).filter((id) => next.has(id));
    onTreeView?.({ hidden: !open, folded: next.has(tagsGroupId) ? [...kept, tagsGroupId] : kept });
  };
  const [searching, setSearching] = useState(false);
  const [query, setQuery] = useState("");
  const searchField = useRef<HTMLInputElement>(null);
  const shell = useRef<HTMLDivElement>(null);
  const [menu, setMenu] = useState<{ id: string; x: number; y: number } | null>(null);
  const [editingId, setEditingId] = useState<string>();
  const [dialog, setDialog] = useState<{ kind: DialogKind; id: string } | null>(null);
  // Tags changed: a tag's page reads its sites again.
  const [tagEdits, setTagEdits] = useState(0);
  const [problem, setProblem] = useState<Problem | null>(null);
  // The backups past the limits a site's History tab asks to remove, confirmed first (ADR-0033).
  const [removing, setRemoving] = useState<{ site: SiteView; backups: BackupView[] } | null>(null);
  const [backupEdits, setBackupEdits] = useState(0);
  // What the sites of the node in the delete dialog keep on their server.
  const [deleteBackups, setDeleteBackups] = useState<{ id: string; sites: SiteBackupsView[] }>();
  // A node deleted while its server could not be reached: the command that removes its backups.
  const [leftBackups, setLeftBackups] = useState<{ name: string; kind: "site" | "host"; command: string } | null>(null);
  const [drag, setDrag] = useState<Drag | null>(null);
  const [flow, setFlow] = useState<Flow | null>(null);
  const [bulkDialog, setBulkDialog] = useState<{ id: string; name: string; previews: SitePreviewView[] | null } | null>(null);
  // The host or folder whose last update of every site shows, until dismissed.
  const [bulkFor, setBulkFor] = useState<string>();
  // The finding being set aside, and why the last attempt was refused.
  // A finding to set aside, or an update offered to ignore (ADR-0026).
  const [ignoring, setIgnoring] = useState<{ site: SiteView; finding?: FindingView; update?: Choice; problem: Problem | null } | null>(null);
  // Grows when an ignore changes: the detail is read again.
  const [ignores, setIgnores] = useState(0);
  // The admin address of a site being given, with the site's own and computed ones.
  const [admin, setAdmin] = useState<{ id: string; own: string; computed: string } | null>(null);
  // The connection test of a host: the stored connection, or the one being edited.
  const [hostTest, setHostTest] = useState<{ name: string; details: () => Promise<HostDetails> } | null>(null);

  // A tag's page is a node of its own, addressed by tagId and not stored (ADR-0025).
  const tagNodes = useMemo(
    () =>
      (tags ?? []).map(
        (tag): Node => ({
          id: tagId(tag.name), parentId: "", kind: "tag", name: tag.name, color: "none", ownColor: "none", path: `#${tag.name}`,
          notes: "", tags: [], multisite: false, subsites: 0, subsiteList: [], status: "", updates: 0,
        }),
      ),
    [tags],
  );
  // Each network's subsites, under it in the list, each a page of its own (ADR-0029).
  const listed = useMemo(() => withSubsites(nodes), [nodes]);
  const byId = new Map([...listed, ...tagNodes].map((n) => [n.id, n]));
  const say = (p: Problem) => t(p.code, { message: p.detail });

  const openSearch = () => {
    if (!paneOpen) leave(true, folded);
    setSearching(true);
    // A field already open takes the focus now; a new one takes it as it appears (autoFocus).
    searchField.current?.focus();
  };
  const closeSearch = () => {
    setSearching(false);
    setQuery("");
  };

  // The delete dialog of a site or host offers its sites' backups too (ADR-0033): read when it opens.
  const deleting = dialog?.kind === "delete" ? byId.get(dialog.id) : undefined;
  useEffect(() => {
    if (!edit || !deleting || (deleting.kind !== "site" && deleting.kind !== "host")) return;
    let current = true;
    void edit.Backups([deleting.id]).then((sites) => {
      if (current) setDeleteBackups({ id: deleting.id, sites: sites ?? [] });
    });
    return () => {
      current = false;
    };
  }, [edit, deleting]);

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
      if (action === "settings") onSection?.("settings");
      if (action === "help") onSection?.("help");
    };
  });
  useEffect(() => commands?.((action) => onCommand.current(action)), [commands]);
  const selected = selectedId ? byId.get(selectedId) : undefined;
  const running = scans.running ?? [];
  const left = running.reduce((n, r) => n + r.sites - r.done, 0);

  // A node is being scanned when it, one of its parents, or everything is; a site, also when one
  // of its tags is.
  const scanning = (node: Node) => {
    // A scan of a selection runs on each of its items (ADR-0034).
    const ids = new Set(running.flatMap((r) => (r.nodeIds && r.nodeIds.length > 0 ? r.nodeIds : [r.nodeId])));
    if (ids.has("")) return true;
    if (node.kind === "site" && [...ids].some((id) => tagOf(id) !== undefined && hasTag(node.tags, tagOf(id)!))) return true;
    for (let n: Node | undefined = node; n; n = nodes.find((p) => p.id === n!.parentId)) {
      if (ids.has(n.id)) return true;
    }
    return false;
  };

  // A node with a site in an update not over yet: it cannot be scanned or updated.
  const busySites = new Set(updates.sites ?? []);
  const updating = (node: Node) =>
    node.kind === "site"
      ? busySites.has(node.id)
      : nodes.some((n) => busySites.has(n.id) && (node.kind === "tag" ? hasTag(n.tags, node.name) : n.path.startsWith(`${node.path}/`)));
  // A subsite opened from a page unfolds its network in the list, where it shows chosen.
  const select = (id: string) => {
    const sub = subsiteOf(id);
    if (sub && !folded.has(openKey(sub.siteId))) leave(paneOpen, new Set([...folded, openKey(sub.siteId)]));
    setSelectedId(id);
  };

  const scan = (id: string) => {
    // A subsite is scanned with its network.
    id = subsiteOf(id)?.siteId ?? id;
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
    if (action === "export") return onExport?.(id);
    // In the folder itself, or next to a host or site; at the top level from the empty space.
    if (action === "newFolder") void createFolder(folderOf(id));
    if (action === "rename") setEditingId(id);
    if (action === "move" || action === "delete" || action === "host" || action === "find" || action === "renameTag") setDialog({ kind: action, id });
    if (action === "updateAll" && byId.get(id)) updateAll(byId.get(id)!);
    if (action === "test" && edit) setHostTest({ name: byId.get(id)?.name ?? "", details: () => edit.Host(id) });
    if (action === "admin") void data?.site(id).then((site) => setAdmin({ id, own: site.ownAdminUrl, computed: site.computedAdminUrl }));
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
      // Several: how many, of which kind.
      const name =
        drag.ids.length > 1 ? t(carriedKey(drag.ids.map((id) => byId.get(id)?.kind)), { count: drag.ids.length }) : dragged.name;
      const text = drag.target
        ? t("edit.dropInto", { name, folder: byId.get(drag.target)?.name })
        : t("edit.dropAtRoot", { name });
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
    if (!nextScan) return selected && <Summary node={selected} nodes={nodes} />;
    return (
      <span className={styles.lines}>
        {selected && <Summary node={selected} nodes={nodes} />}
        <NextScan at={nextScan} />
      </span>
    );
  };

  const inert = (dialog !== null || bulkDialog !== null || ignoring !== null || admin !== null || hostTest !== null) || undefined;

  const ignore = async (untilVersion: string, reason: string) => {
    if (!security || !ignoring) return;
    const key = ignoring.finding?.key ?? ignoring.update?.ignoreKey ?? "";
    const refused = await security.Ignore(ignoring.site.id, key, untilVersion, reason);
    if (refused) return setIgnoring({ ...ignoring, problem: refused });
    setIgnoring(null);
    setIgnores((n) => n + 1);
    await onChanged?.();
  };
  const restore = async (site: SiteView, key: string) => {
    if (!security) return;
    setProblem(null);
    const refused = await security.Unignore(site.id, key);
    if (refused) setProblem(refused);
    setIgnores((n) => n + 1);
    await onChanged?.();
  };

  // The settings and the add-ons take the place of the site list and the detail.
  if (section !== "sites") {
    const pages = { settings: settingsPage, addons: addonsPage, activity: activityPage, help: helpPage };
    return (
      <div ref={shell} className={styles.shell}>
        <NavRail current={section} onAddHost={onAddHost} onNavigate={onSection} />
        <main className={styles.main} tabIndex={0}>
          {pages[section]}
        </main>
      </div>
    );
  }

  // The update of a site takes the whole window; the shell keeps its state behind it.
  if (flow && updatesApi) {
    return (
      <UpdateFlow
        api={updatesApi}
        site={flow.site}
        keys={flow.keys}
        known={flow.known}
        onOpen={onOpen}
        names={flow.names}
        state={updates}
        follow={flow.follow}
        onClose={() => setFlow(null)}
      />
    );
  }

  return (
    <div ref={shell} className={styles.shell}>
      <NavRail current="sites" onAddHost={onAddHost} inert={inert} onNavigate={onSection} />
      {paneOpen && (
        <aside aria-label={t("manager.label")} className={styles.pane} inert={inert}>
          <div className={styles.paneHeader}>
            <h2 className={styles.paneTitle}>{t("nav.sites")}</h2>
            <Button variant="icon" icon="search" label={t("manager.search")} onClick={() => (searching ? closeSearch() : openSearch())} />
            <Button variant="icon" icon="sidebar" label={t("manager.collapse")} onClick={() => leave(false, folded)} />
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
                nodes={listed}
                tags={tags}
                selectedId={selectedId}
                onSelect={setSelectedId}
                selectedIds={selection}
                onSelectionChange={setSelection}
                filter={query}
                collapsed={folded}
                onCollapsedChange={(next) => leave(paneOpen, next)}
                {...(edit && {
                  editingId,
                  // A tag is renamed in a dialog: it may merge with another.
                  onEdit: (id: string) => (tagOf(id) !== undefined ? setDialog({ kind: "renameTag", id }) : setEditingId(id)),
                  onRename: (id: string, name: string) => {
                    setEditingId(undefined);
                    void run(() => edit.Rename(id, name));
                  },
                  onCancelEdit: () => setEditingId(undefined),
                  onDelete: (id: string) => setDialog({ kind: "delete", id }),
                  onMenu: openMenu,
                  onMove: (ids: string[], folderId: string) =>
                    void run(() => (ids.length === 1 && ids[0] ? edit.Move(ids[0], folderId) : edit.MoveMany(ids, folderId))),
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
      <main className={paneOpen ? styles.main : `${styles.main} ${styles.folded}`} tabIndex={0} inert={inert}>
        {!paneOpen && (
          <Button variant="icon" icon="sidebar" label={t("manager.expand")} onClick={() => leave(true, folded)} />
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
                revision={scans.completed + updates.completed + ignores + tagEdits + backupEdits}
                scanning={scanning(selected)}
                onScan={() => scan(selected.id)}
                onSelect={select}
                onMore={edit && selected.kind !== "subsite" ? (point) => openMenu(selected.id, point) : undefined}
                onSiteMenu={edit && ((id, point) => openMenu(id, point, false))}
                onRemove={edit && (() => setDialog({ kind: "delete", id: selected.id }))}
                onRemoveBackups={data.removeOldBackups && ((site, backups) => setRemoving({ site, backups }))}
                updating={updating(selected)}
                onCheckFiles={onCheckFiles && (() => !updating(selected) && onCheckFiles(selected.id))}
                onIgnore={security && ((site, finding) => setIgnoring({ site, finding, problem: null }))}
                onRestore={security && ((site, finding) => void restore(site, finding.key))}
                onIgnoreUpdate={security && ((site: SiteView, update: Choice) => setIgnoring({ site, update, problem: null }))}
                onRestoreUpdate={security && ((site: SiteView, update: Choice) => void restore(site, update.ignoreKey))}
                onOpen={onOpen}
                onCopy={onCopy}
                terminal={canOpenSSH}
                onSaveNotes={
                  edit &&
                  (async (notes: string) => {
                    const refused = await edit.SetNotes(selected.id, notes);
                    if (!refused) await onChanged?.();
                    return refused;
                  })
                }
                usedTags={tags}
                onOpenTag={(tag: string) => setSelectedId(tagId(tag))}
                onSaveTags={
                  edit &&
                  (async (siteTags: string[]) => {
                    const refused = await edit.SetTags(selected.id, siteTags);
                    if (!refused) {
                      setTagEdits((n) => n + 1);
                      await onChanged?.();
                    }
                    return refused;
                  })
                }
                {...(updatesApi && {
                  onUpdate: (site: SiteView, keys: Set<string>) =>
                    setFlow({ site: { id: site.id, name: site.name }, keys, names: namesOf(site.scan?.updates ?? []), known: knownOf(site.scan?.updates ?? []), follow: false }),
                  onFollow: (site: SiteView) =>
                    setFlow({ site: { id: site.id, name: site.name }, keys: new Set(), names: namesOf(site.scan?.updates ?? []), known: knownOf(site.scan?.updates ?? []), follow: true }),
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
          entries={menuEntries(menuNode, nodes, t, !!onExport)}
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

      {ignoring && (
        <IgnoreDialog
          finding={ignoring.finding}
          update={ignoring.update && { name: ignoring.update.kind === "core" ? "WordPress" : ignoring.update.name, from: ignoring.update.from, to: ignoring.update.to }}
          site={ignoring.site.name}
          problem={ignoring.problem}
          onIgnore={(untilVersion, reason) => void ignore(untilVersion, reason)}
          onClose={() => setIgnoring(null)}
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
          backups={deleteBackups?.id === dialogNode.id ? deleteBackups.sites : undefined}
          onDelete={(withBackups) => {
            setDialog(null);
            // The selection goes when it was the node or under it.
            for (let n = selected; n; n = byId.get(n.parentId)) if (n.id === dialogNode.id) setSelectedId(undefined);
            if (dialogNode.kind === "tag") {
              setTagEdits((n) => n + 1);
              void run(() => edit.RemoveTag(dialogNode.name));
              return;
            }
            void run(async () => {
              const result = await edit.Delete([dialogNode.id], withBackups);
              if (result.command && (dialogNode.kind === "site" || dialogNode.kind === "host")) {
                setLeftBackups({ name: dialogNode.name, kind: dialogNode.kind, command: result.command });
              }
              return result.problem;
            });
          }}
          onClose={() => setDialog(null)}
        />
      )}
      {leftBackups && (
        <BackupsLeftDialog name={leftBackups.name} kind={leftBackups.kind} command={leftBackups.command} onCopy={onCopy} onClose={() => setLeftBackups(null)} />
      )}
      {removing && data?.removeOldBackups && (
        <RemoveBackupsDialog
          name={removing.site.name}
          backups={removing.backups}
          onClose={() => setRemoving(null)}
          onRemove={() => {
            const site = removing.site;
            setRemoving(null);
            void run(() => data.removeOldBackups!(site.id)).then(() => setBackupEdits((n) => n + 1));
          }}
        />
      )}
      {edit && dialog && dialogNode && dialog.kind === "renameTag" && (
        <RenameTagDialog
          tag={dialogNode.name}
          tags={tags ?? []}
          nodes={nodes}
          onRename={async (name) => {
            const refused = await edit.RenameTag(dialogNode.name, name);
            if (refused) return refused;
            setDialog(null);
            // The page follows the tag: renamed onto another, it is that one's.
            const merged = (tags ?? []).find((u) => fold(u.name) === fold(name.trim()) && fold(u.name) !== fold(dialogNode.name));
            setSelectedId(tagId(merged?.name ?? name.trim().replace(/\s+/g, " ")));
            setTagEdits((n) => n + 1);
            await onChanged?.();
            return null;
          }}
          onClose={() => setDialog(null)}
        />
      )}
      {edit && admin && (
        <AdminAddressDialog
          own={admin.own}
          computed={admin.computed}
          onClose={() => setAdmin(null)}
          onSave={async (address) => {
            const refused = await edit.SetAdminURL(admin.id, address);
            if (!refused) {
              setAdmin(null);
              setIgnores((n) => n + 1); // the site's page reads its address again
            }
            return refused;
          }}
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
          onTest={(d) => setHostTest({ name: d.name, details: async () => d })}
        />
      )}
      {edit && hostTest && (
        <HostTestDialog
          name={hostTest.name}
          details={hostTest.details}
          test={(d) => edit.TestConnection(d)}
          onClose={() => setHostTest(null)}
          onHelp={
            onSection &&
            (() => {
              setHostTest(null);
              setDialog(null);
              onSection("help");
            })
          }
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
function menuEntries(node: Node, nodes: Node[], t: T, canExport: boolean): MenuEntry[] {
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
  if (node.kind === "tag") {
    return [
      item("scan", t("tags.menu.scan"), "refresh", { shortcut: "⌘R", keyshortcuts: "Meta+R" }), item("updateAll", t("tags.menu.update"), "update"),
      separator, item("renameTag", t("tags.menu.rename"), "rename", { shortcut: "↩", keyshortcuts: "Enter" }),
      separator, item("delete", t("tags.menu.remove"), "trash", { danger: true, shortcut: "⌘⌫", keyshortcuts: "Meta+Backspace" }),
    ];
  }
  if (node.kind === "site") {
    return [scan, newFolder, separator, rename, colors, item("admin", t("admin.menu"), "external"), separator, remove("edit.menu.deleteSite")];
  }
  const exported = canExport ? [item("export", t("edit.menu.export"), "archive")] : [];
  if (node.kind === "host") {
    return [
      scan, newFolder, separator, rename, move, colors,
      item("host", t("edit.menu.edit"), "host"), item("test", t("edit.menu.test"), "ok"), item("find", t("edit.menu.find"), "findSites"), ...exported,
      separator, remove("edit.menu.deleteHost"),
    ];
  }
  // Only an empty folder can be deleted.
  const empty = !nodes.some((n) => n.parentId === node.id);
  return [
    scan, newFolder,
    separator, rename, move, colors, ...exported, separator, remove("edit.menu.deleteFolder", !empty),
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
  onRemoveBackups,
  updating,
  onUpdate,
  onFollow,
  onRollback,
  onUpdateAll,
  bulk,
  onDismissBulk,
  onCheckFiles,
  onIgnore,
  onRestore,
  onIgnoreUpdate,
  onRestoreUpdate,
  onOpen,
  onCopy,
  terminal,
  onSaveNotes,
  usedTags,
  onSaveTags,
  onOpenTag,
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
  /** Asks to remove a site's backups past the limits. */
  onRemoveBackups?: (site: SiteView, backups: BackupView[]) => void;
  updating: boolean;
  onUpdate?: (site: SiteView, keys: Set<string>) => void;
  onFollow?: (site: SiteView) => void;
  onRollback?: (site: SiteView, operationID: string) => void;
  onUpdateAll?: () => void;
  bulk?: SiteResultView[];
  onDismissBulk?: () => void;
  onCheckFiles?: () => void;
  onIgnore?: (site: SiteView, finding: FindingView) => void;
  onRestore?: (site: SiteView, finding: FindingView) => void;
  onIgnoreUpdate?: (site: SiteView, update: Choice) => void;
  onRestoreUpdate?: (site: SiteView, update: Choice) => void;
  onOpen?: (href: string) => void;
  onCopy?: (text: string) => Promise<unknown> | void;
  terminal?: boolean;
  /** Saves the notes of the node shown; without it, they are only read. */
  onSaveNotes?: (notes: string) => Promise<Problem | null>;
  usedTags?: TagView[];
  /** Saves the tags of the site shown; without it, they are only read. */
  onSaveTags?: (tags: string[]) => Promise<Problem | null>;
  /** Opens a tag's page. */
  onOpenTag?: (tag: string) => void;
}) {
  const { t } = useTranslation();
  const [view, setView] = useState<{ id: string; view: View }>();
  // The list across a host's or folder's sites, read again after a scan (revision).
  const extensions = useMemo(
    () => (data.extensions && node.kind !== "site" ? () => data.extensions!(node.id) : undefined),
    // eslint-disable-next-line react-hooks/exhaustive-deps -- revision: read again after a scan
    [data, node.id, node.kind, revision],
  );

  useEffect(() => {
    let current = true;
    const load = async (): Promise<View> => {
      switch (node.kind) {
        case "site": {
          const [site, history, backups] = await Promise.all([data.site(node.id), data.history?.(node.id), data.backups?.(node.id)]);
          return { kind: "site", site, history, backups };
        }
        case "host":
          return { kind: "host", host: await data.host(node.id) };
        case "tag":
          return { kind: "tag", rows: await data.sites(node.id) };
        case "subsite": {
          // Read from its network's latest scan (ADR-0029).
          const { siteId, blogId } = subsiteOf(node.id)!;
          return { kind: "subsite", site: await data.site(siteId), blogId };
        }
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
  if (v.kind === "subsite") {
    return <SubsitePage site={v.site} blogId={v.blogId} onNetwork={() => onSelect(v.site.id)} onOpen={onOpen} />;
  }
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
        backups={v.backups}
        onRemoveBackups={onRemoveBackups && v.backups && (() => onRemoveBackups(site, (v.backups!.items ?? []).filter((b) => b.expired)))}
        onRemove={onRemove}
        onCheckFiles={onCheckFiles}
        onIgnore={onIgnore && ((finding) => onIgnore(site, finding))}
        onRestore={onRestore && ((finding) => onRestore(site, finding))}
        onIgnoreUpdate={onIgnoreUpdate && ((update) => onIgnoreUpdate(site, update))}
        onRestoreUpdate={onRestoreUpdate && ((update) => onRestoreUpdate(site, update))}
        onOpen={onOpen}
        onCopy={onCopy}
        terminal={terminal}
        notes={node.notes}
        onSaveNotes={onSaveNotes}
        tags={node.tags ?? []}
        usedTags={usedTags}
        onSaveTags={onSaveTags}
        onOpenTag={onOpenTag}
        onOpenSubsite={(blogId) => onSelect(subsiteId(site.id, blogId))}
      />
    );
  }
  const bulkProps = { onUpdateAll, updating, bulk, onDismissBulk, onSiteMenu, notes: node.notes, onSaveNotes, onOpen, extensions };
  if (v.kind === "tag") {
    const where = spread(v.rows.map((r) => r.path));
    const subtitle = [
      t("tags.page.sites", { count: where.sites }),
      t("tags.page.hosts", { count: where.hosts }),
      ...(where.folders > 0 ? [t("tags.page.folders", { count: where.folders })] : []),
    ].join(" · ");
    // A tag has no notes of its own.
    return (
      <SitesOverview
        title={node.name}
        icon="tag"
        subtitle={subtitle}
        rows={v.rows}
        scanLabel={t("tags.page.scan")}
        scanning={scanning}
        onScan={onScan}
        onSelect={onSelect}
        onMore={onMore}
        {...bulkProps}
        notes={undefined}
        onSaveNotes={undefined}
      />
    );
  }
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

type View =
  | { kind: "site"; site: SiteView; history?: OperationView[]; backups?: BackupsView }
  | { kind: "host"; host: HostView }
  | { kind: "folder"; rows: SiteRow[] }
  | { kind: "tag"; rows: SiteRow[] }
  | { kind: "subsite"; site: SiteView; blogId: number };

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
  const count = nodes.filter((n) => n.kind === "site" && (node.kind === "tag" ? hasTag(n.tags, node.name) : n.path.startsWith(prefix))).length;
  return <span>{t("bottom.sites", { name: node.name, count })}</span>;
}

/** When the next automatic scan is due, quietly: a clock and its time (its day too when not today);
 * the whole sentence for screen readers and on hover. */
function NextScan({ at }: { at: string }) {
  const { t } = useTranslation();
  const today = new Date(at).toDateString() === new Date().toDateString();
  const sentence = today ? t("manager.nextScan", { time: clock(at) }) : t("manager.nextScanOn", { day: day(at), time: clock(at) });
  return (
    <span className={styles.next} title={sentence}>
      <Icon name="clock" size={12} />
      <span aria-hidden="true">{today ? clock(at) : `${day(at)} ${clock(at)}`}</span>
      <span className={styles.hidden}>{sentence}</span>
    </span>
  );
}
