import { Browser, Events } from "@wailsio/runtime";
import { useCallback, useEffect, useState } from "react";
import { I18nextProvider } from "react-i18next";

import { VigieDiscovery, VigieMenu, VigieScans, VigieUpdates } from "../bindings/events";
import { Data, Editor, HostWizard, Preferences, Scanner, Security, SiteManager, Updates } from "../bindings/github.com/MarJC5/vigie/internal/app";
import type { LookupsView, Node, ScansState, UpdatesState } from "../bindings/github.com/MarJC5/vigie/internal/app/models";
import { AddHostWizard, type WizardApi } from "./components/AddHostWizard/AddHostWizard";
import { AppShell, type EditApi, type SecurityApi, type ShellData } from "./components/AppShell/AppShell";
import { ConsentScreen } from "./components/ConsentScreen/ConsentScreen";
import { type DataApi, DataFlow, type DataNote, type DataTask } from "./components/DataFlow/DataFlow";
import type { Section } from "./components/NavRail/NavRail";
import { SettingsPage, type SettingsTab } from "./components/SettingsPage/SettingsPage";
import type { UpdatesApi } from "./components/UpdateFlow/UpdateFlow";
import { createI18n } from "./i18n";

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
  SetColor: Editor.SetColor,
  Delete: Editor.Delete,
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

// What sets findings aside, from the security service.
const securityApi: SecurityApi = { Ignore: Security.Ignore, Unignore: Security.Unignore };

// Links open in the user's browser, never in the window.
const openURL = (href: string) => void Browser.OpenURL(href);

// The commands of the native menu bar.
const menuCommands = (handler: (action: string) => void) => Events.On(VigieMenu, (event) => handler(event.data.action));

interface Loaded {
  i18n: ReturnType<typeof createI18n>;
  nodes: Node[];
  version: string;
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
  const [dataTask, setDataTask] = useState<DataTask>();
  const [dataNote, setDataNote] = useState<DataNote>();
  // The preview of an import takes the whole window, as the wizard does.
  const [dataWindow, setDataWindow] = useState(false);

  useEffect(() => {
    void Promise.all([Preferences.Language(), SiteManager.Tree(), Preferences.Version()]).then(([lang, tree, version]) => {
      const i18n = createI18n(lang);
      document.documentElement.lang = i18n.language;
      setLoaded({ i18n, nodes: tree.nodes ?? [], version });
    });
    void Security.Lookups().then(setLookups);
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
    void SiteManager.Tree().then((tree) => setLoaded((l) => l && { ...l, nodes: tree.nodes ?? [] }));
  }, [ended]);

  // Reads the tree again, after an edit or a host added.
  const reload = async () => {
    const tree = await SiteManager.Tree();
    setLoaded((l) => l && { ...l, nodes: tree.nodes ?? [] });
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
    setDataTask(undefined);
    setDataWindow(false);
    if (note) setDataNote(note);
    if (changed) void reload();
  };

  const showDataWindow = useCallback(() => setDataWindow(true), []);

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
          onAddHost={() => setAdding(true)}
          initialSelectedId={added}
          data={shellData}
          scans={scans}
          onScan={(nodeId) => scan(nodeId)}
          onCheckFiles={(nodeId) => scan(nodeId, true)}
          security={securityApi}
          onOpen={openURL}
          section={section}
          onSection={setSection}
          settingsPage={
            lookups && (
              <SettingsPage
                tab={settingsTab}
                onTab={setSettingsTab}
                language={loaded.i18n.language}
                onLanguage={(lang) => void changeLanguage(lang)}
                lookups={lookups}
                onAgree={() => void changeLookups(() => Security.Agree((lookups.destinations ?? []).filter((d) => d.on).map((d) => d.id)))}
                onWithdraw={() => void changeLookups(() => Security.Withdraw())}
                onDestination={(id, on) => void changeLookups(() => Security.SetDestination(id, on))}
                version={loaded.version}
                onOpen={openURL}
                onExport={() => setDataTask({ kind: "export", nodeId: "" })}
                onImport={(source) => setDataTask(source === "ssh" ? { kind: "ssh" } : { kind: "import", source })}
                dataNote={dataNote}
              />
            )
          }
          onExport={(nodeId) => setDataTask({ kind: "export", nodeId })}
          onStopAll={() => void Scanner.CancelAll()}
          commands={menuCommands}
          edit={editApi}
          onChanged={reload}
          updates={updates}
          updatesApi={updatesApi}
        />
      )}
      {dataTask && !waiting && !adding && <DataFlow api={dataApi} task={dataTask} nodes={loaded.nodes} onClose={dataDone} onWindow={showDataWindow} />}
    </I18nextProvider>
  );
}
