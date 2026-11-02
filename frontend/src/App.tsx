import { Browser, Clipboard, Events } from "@wailsio/runtime";
import { useCallback, useEffect, useRef, useState } from "react";
import { I18nextProvider } from "react-i18next";

import { VigieDiscovery, VigieMenu, VigieScans, VigieUpdates } from "../bindings/events";
import {
  Addons,
  Data,
  Editor,
  HostWizard,
  LoginItem,
  Preferences,
  Scanner,
  Security,
  SiteManager,
  Activity,
  Updates,
  Watch,
} from "../bindings/github.com/MarJC5/vigie/internal/app";
import type { BackgroundSettings, DatabaseView, Journal, LoginItemState, LookupsView, Node, ScansState, TagView, TreeView, UpdatesState } from "../bindings/github.com/MarJC5/vigie/internal/app/models";
import { ActivityPage } from "./components/ActivityPage/ActivityPage";
import { AddHostWizard, type WizardApi } from "./components/AddHostWizard/AddHostWizard";
import { type AddonsApi, AddonsFlow } from "./components/AddonsFlow/AddonsFlow";
import { AppShell, type EditApi, type SecurityApi, type ShellData } from "./components/AppShell/AppShell";
import { ConsentScreen } from "./components/ConsentScreen/ConsentScreen";
import { type DataApi, DataFlow, type DataNote, type DataTask } from "./components/DataFlow/DataFlow";
import type { Section } from "./components/NavRail/NavRail";
import { HelpPage, type HelpTab } from "./components/HelpPage/HelpPage";
import { SettingsPage, type SettingsTab, type Theme } from "./components/SettingsPage/SettingsPage";
import type { UpdatesApi } from "./components/UpdateFlow/UpdateFlow";
import { createI18n } from "./i18n";
import { sizeText } from "./lib/data";

// The wizard's service, with the progress of discovery as the Go side sends it.
const wizardApi: WizardApi = {
  ...HostWizard,
  onDiscovery: (listener) => Events.On(VigieDiscovery, (event) => listener(event.data)),
};

// What the detail pane reads, from the site manager service.
const shellData: ShellData = {
  site: (id) => SiteManager.SiteDetail(id),
  host: (id) => SiteManager.HostDetail(id),
  sites: (id) => SiteManager.Sites(id).then((rows) => rows ?? []),
  history: (id) => Updates.History(id).then((ops) => ops ?? []),
  backups: (id) => Updates.Backups(id),
  removeOldBackups: (id) => Updates.RemoveOldBackups(id),
  extensions: (id) => SiteManager.Extensions(id),
};

// What updates sites, from the updates service.
const updatesApi: UpdatesApi = {
  Preview: Updates.Preview,
  Update: Updates.Update,
  UpdateAll: Updates.UpdateAll,
  Rollback: Updates.Rollback,
  CancelAll: Updates.CancelAll,
  History: Updates.History,
};

// What edits the site manager: the editor, and the wizard's services for a stored host.
const editApi: EditApi = {
  CreateFolder: Editor.CreateFolder,
  Rename: Editor.Rename,
  Move: Editor.Move,
  MoveMany: Editor.MoveMany,
  SetColor: Editor.SetColor,
  Delete: Editor.Delete,
  Backups: Editor.Backups,
  SetAdminURL: Editor.SetAdminURL,
  SetNotes: Editor.SetNotes,
  SetTags: Editor.SetTags,
  RenameTag: Editor.RenameTag,
  RemoveTag: Editor.RemoveTag,
  TestConnection: HostWizard.TestConnection,
  Host: Editor.Host,
  EditHost: Editor.EditHost,
  HostKeys: HostWizard.HostKeys,
  Trust: HostWizard.Trust,
  KeepKey: HostWizard.KeepKey,
  DiscoverHost: HostWizard.DiscoverHost,
  AddSites: HostWizard.AddSites,
  onDiscovery: (listener) => Events.On(VigieDiscovery, (event) => listener(event.data)),
};

// What exports and imports, from the data service.
const dataApi: DataApi = {
  ExportSummary: Data.ExportSummary,
  Export: Data.Export,
  ChooseImport: Data.ChooseImport,
  PreviewImport: Data.PreviewImport,
  ApplyImport: Data.ApplyImport,
  CancelImport: Data.CancelImport,
  SSHConfig: Data.SSHConfig,
  ImportSSHConfig: Data.ImportSSHConfig,
};

// What lists and runs add-ons, from the add-ons service.
const addonsApi: AddonsApi = { List: Addons.List, ChooseFile: Addons.ChooseFile, Run: Addons.Run };

// What sets findings aside, from the security service.
const securityApi: SecurityApi = { Ignore: Security.Ignore, Unignore: Security.Unignore };

// Links open in the user's browser, never in the window.
const openURL = (href: string) => void Browser.OpenURL(href);
const copyText = (text: string) => Clipboard.SetText(text);

// The commands of the native menu bar.
const menuCommands = (handler: (action: string) => void) => Events.On(VigieMenu, (event) => handler(event.data.action));

// The theme chosen goes on the root element, which the tokens follow over the system's.
const applyTheme = (theme: Theme) => {
  if (theme === "system") delete document.documentElement.dataset.theme;
  else document.documentElement.dataset.theme = theme;
};

interface Loaded {
  i18n: ReturnType<typeof createI18n>;
  theme: Theme;
  nodes: Node[];
  tags: TagView[];
  version: string;
  treeView: TreeView;
}

/** A scan asked for before the user answered the consent screen. */
interface Waiting {
  nodeId: string;
  checkFiles: boolean;
}

/** Reads the interface language and the site manager from the Go side, then shows the window. */
export function App() {
  const [loaded, setLoaded] = useState<Loaded>();
  const [adding, setAdding] = useState(false);
  // A host just added: selected when the shell opens again.
  const [added, setAdded] = useState<string>();
  const [scans, setScans] = useState<ScansState>({ running: [], completed: 0 });
  const [updates, setUpdates] = useState<UpdatesState>({ running: [], active: 0, bulk: [], completed: 0, sites: [] });
  const [lookups, setLookups] = useState<LookupsView>();
  const [waiting, setWaiting] = useState<Waiting>();
  const [section, setSection] = useState<Section>("sites");
  const [settingsTab, setSettingsTab] = useState<SettingsTab>("general");
  const [helpTab, setHelpTab] = useState<HelpTab>("start");
  const [dataTask, setDataTask] = useState<DataTask>();
  const [dataNote, setDataNote] = useState<DataNote>();
  const [database, setDatabase] = useState<DatabaseView>();
  const [backupDays, setBackupDays] = useState(30);
  const [compacting, setCompacting] = useState(false);
  // The preview of an import takes the whole window, as the wizard does.
  const [dataWindow, setDataWindow] = useState(false);
  const [background, setBackground] = useState<BackgroundSettings>({ on: false, autoScan: 0, notify: false });
  const [loginItem, setLoginItem] = useState<LoginItemState>({ supported: false, on: false });
  const [nextScan, setNextScan] = useState("");
  const [canOpenSSH, setCanOpenSSH] = useState(false);
  // The Activity page: its days, its filter, the journal read.
  const [journal, setJournal] = useState<Journal>();
  const [journalDays, setJournalDays] = useState(30);
  const [journalFilter, setJournalFilter] = useState("");

  useEffect(() => {
    void Promise.all([Preferences.Language(), SiteManager.Tree(), Preferences.Version(), Preferences.Theme(), Preferences.TreeView()]).then(
      ([lang, tree, version, theme, treeView]) => {
        const i18n = createI18n(lang);
        document.documentElement.lang = i18n.language;
        // Before the first render: the window never shows the system's theme first.
        applyTheme(theme as Theme);
        setLoaded({ i18n, nodes: tree.nodes ?? [], tags: tree.tags ?? [], version, theme: theme as Theme, treeView });
      },
    );
    void Security.Lookups().then(setLookups);
    void SiteManager.CanOpenSSH().then(setCanOpenSSH);
    void Preferences.Background().then(setBackground);
    void LoginItem.State().then(setLoginItem);
    void Scanner.State().then(setScans);
    void Updates.State().then(setUpdates);
    const offScans = Events.On(VigieScans, (event) => setScans(event.data));
    const offUpdates = Events.On(VigieUpdates, (event) => setUpdates(event.data));
    return () => {
      offScans();
      offUpdates();
    };
  }, []);

  // A scan stored, or an update ended (and the site scanned again): the statuses of the tree change.
  const ended = scans.completed + updates.completed;
  useEffect(() => {
    if (ended === 0) return;
    void SiteManager.Tree().then((tree) => setLoaded((l) => l && { ...l, nodes: tree.nodes ?? [], tags: tree.tags ?? [] }));
  }, [ended]);

  // The next automatic scan: read again after each scan, each change of the settings, and each minute.
  const readNextScan = useCallback(() => void Watch.State().then((w) => setNextScan(w.next)), []);
  useEffect(() => {
    readNextScan();
    const timer = setInterval(readNextScan, 60_000);
    return () => clearInterval(timer);
  }, [readNextScan, ended]);

  // Saved one after the other, in the order chosen: two quick changes never land the wrong way round.
  const savingBackground = useRef(Promise.resolve());
  // The journal, read when the page opens, after each scan or update, and when its span or filter change.
  useEffect(() => {
    if (section !== "activity") return;
    void Activity.Journal(journalDays, journalFilter).then(setJournal);
  }, [section, journalDays, journalFilter, ended]);

  // The database's size, read when the Data tab opens and after each scan or update.
  useEffect(() => {
    if (section !== "settings" || settingsTab !== "data") return;
    void Data.Database().then(setDatabase);
    void Data.BackupDays().then(setBackupDays);
  }, [section, settingsTab, ended]);

  const compact = async () => {
    const i18n = loaded?.i18n;
    if (!i18n) return;
    setCompacting(true);
    const result = await Data.Compact();
    setCompacting(false);
    const size = (bytes: number) => sizeText(bytes, i18n.language);
    setDataNote({
      row: "compact",
      text: result.problem
        ? i18n.t(result.problem.code, { message: result.problem.detail })
        : i18n.t("data.compacted", { before: size(result.before), after: size(result.after) }),
    });
    setDatabase(await Data.Database());
  };

  const changeBackground = (b: BackgroundSettings) => {
    setBackground(b);
    savingBackground.current = savingBackground.current.then(async () => {
      await Preferences.SetBackground(b);
      // Turning the mode off also removes the login item.
      setLoginItem(await LoginItem.State());
      readNextScan();
    });
  };
  const changeLoginItem = async (on: boolean) => {
    await LoginItem.Set(on);
    setLoginItem(await LoginItem.State());
  };

  // Reads the tree again, after an edit or a host added.
  const reload = async () => {
    const tree = await SiteManager.Tree();
    setLoaded((l) => l && { ...l, nodes: tree.nodes ?? [], tags: tree.tags ?? [] });
  };

  // A scan starts at once, unless the user never answered the consent screen: it asks first.
  const start = (w: Waiting) => void (w.checkFiles ? Scanner.CheckFiles(w.nodeId) : Scanner.Scan(w.nodeId));
  const scan = (nodeId: string, checkFiles = false) => {
    if (lookups && !lookups.agreed && !lookups.asked) return setWaiting({ nodeId, checkFiles });
    start({ nodeId, checkFiles });
  };
  const answered = async (answer: () => Promise<unknown>) => {
    await answer();
    setLookups(await Security.Lookups());
    if (waiting) start(waiting);
    setWaiting(undefined);
  };
  const changeLookups = async (change: () => Promise<unknown>) => {
    await change();
    setLookups(await Security.Lookups());
  };
  const changeLanguage = async (lang: string) => {
    await Preferences.SetLanguage(lang);
    await loaded?.i18n.changeLanguage(lang);
    document.documentElement.lang = lang;
    setLoaded((l) => l && { ...l });
  };

  const dataDone = (note?: DataNote, changed?: boolean) => {
    // What an add-on brought shows in the site list.
    if (dataTask?.kind === "preview" && changed) setSection("sites");
    setDataTask(undefined);
    setDataWindow(false);
    if (note) setDataNote(note);
    if (changed) void reload();
  };

  const showDataWindow = useCallback(() => setDataWindow(true), []);

  const changeTreeView = (treeView: TreeView) => {
    setLoaded((l) => l && { ...l, treeView });
    void Preferences.SetTreeView(treeView);
  };

  const changeTheme = async (theme: Theme) => {
    applyTheme(theme);
    setLoaded((l) => l && { ...l, theme });
    await Preferences.SetTheme(theme);
  };

  const hostAdded = async (hostId: string) => {
    await reload();
    setAdded(hostId);
    setAdding(false);
  };

  // Both calls stay in the process and answer at once: nothing to show while they run.
  if (!loaded) return null;
  const folders = loaded.nodes.filter((n) => n.kind === "folder").map((n) => ({ id: n.id, path: n.path }));
  const destinations = (lookups?.destinations ?? []).map((d) => d.id);
  return (
    <I18nextProvider i18n={loaded.i18n}>
      {waiting ? (
        <ConsentScreen
          destinations={destinations}
          onAgree={(on) => void answered(() => Security.Agree(on))}
          onNotNow={() => void answered(() => Security.NotNow())}
        />
      ) : adding ? (
        <AddHostWizard api={wizardApi} folders={folders} onClose={() => setAdding(false)} onAdded={hostAdded} />
      ) : dataWindow ? null : (
        <AppShell
          key={added}
          nodes={loaded.nodes}
          tags={loaded.tags}
          treeView={loaded.treeView}
          onTreeView={changeTreeView}
          onAddHost={() => setAdding(true)}
          initialSelectedId={added}
          data={shellData}
          scans={scans}
          onScan={(nodeId) => scan(nodeId)}
          onCheckFiles={(nodeId) => scan(nodeId, true)}
          security={securityApi}
          onOpen={openURL}
          onCopy={copyText}
          canOpenSSH={canOpenSSH}
          section={section}
          onSection={setSection}
          settingsPage={
            lookups && (
              <SettingsPage
                tab={settingsTab}
                onTab={setSettingsTab}
                language={loaded.i18n.language}
                onLanguage={(lang) => void changeLanguage(lang)}
                theme={loaded.theme}
                onTheme={(theme) => void changeTheme(theme)}
                lookups={lookups}
                onAgree={() => void changeLookups(() => Security.Agree((lookups.destinations ?? []).filter((d) => d.on).map((d) => d.id)))}
                onWithdraw={() => void changeLookups(() => Security.Withdraw())}
                onDestination={(id, on) => void changeLookups(() => Security.SetDestination(id, on))}
                version={loaded.version}
                onOpen={openURL}
                onExport={() => setDataTask({ kind: "export", nodeId: "" })}
                onImport={(source) => setDataTask(source === "ssh" ? { kind: "ssh" } : { kind: "import", source })}
                dataNote={dataNote}
                database={database}
                compacting={compacting}
                busy={(scans.running ?? []).length > 0 || updates.active > 0}
                onCompact={() => void compact()}
                backupDays={backupDays}
                onBackupDays={(days) => void Data.SetBackupDays(days).then((p) => !p && setBackupDays(days))}
                background={background}
                onBackground={changeBackground}
                loginItem={loginItem}
                onLoginItem={(on) => void changeLoginItem(on)}
              />
            )
          }
          activityPage={
            journal && (
              <ActivityPage
                journal={journal}
                days={journalDays}
                filter={journalFilter}
                nodes={loaded.nodes}
                onFilter={setJournalFilter}
                onOlder={() => setJournalDays((d) => d + 30)}
                onOpen={(nodeId) => {
                  setSection("sites");
                  setAdded(nodeId);
                }}
              />
            )
          }
          helpPage={<HelpPage tab={helpTab} onTab={setHelpTab} onOpen={openURL} onAddHost={() => setAdding(true)} />}
          addonsPage={<AddonsFlow api={addonsApi} onLink={openURL} onPreview={(preview) => setDataTask({ kind: "preview", preview })} />}
          onExport={(nodeId) => setDataTask({ kind: "export", nodeId })}
          onStopAll={() => void Scanner.CancelAll()}
          commands={menuCommands}
          edit={editApi}
          onChanged={reload}
          updates={updates}
          updatesApi={updatesApi}
          nextScan={nextScan}
        />
      )}
      {dataTask && !waiting && !adding && <DataFlow api={dataApi} task={dataTask} nodes={loaded.nodes} onClose={dataDone} onWindow={showDataWindow} />}
    </I18nextProvider>
  );
}
