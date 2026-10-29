import { Events } from "@wailsio/runtime";
import { useEffect, useState } from "react";
import { I18nextProvider } from "react-i18next";

import { VigieDiscovery, VigieMenu, VigieScans } from "../bindings/events";
import { Editor, HostWizard, Preferences, Scanner, SiteManager } from "../bindings/github.com/MarJC5/vigie/internal/app";
import type { Node, ScansState } from "../bindings/github.com/MarJC5/vigie/internal/app/models";
import { AddHostWizard, type WizardApi } from "./components/AddHostWizard/AddHostWizard";
import { AppShell, type EditApi, type ShellData } from "./components/AppShell/AppShell";
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

// The commands of the native menu bar.
const menuCommands = (handler: (action: string) => void) => Events.On(VigieMenu, (event) => handler(event.data.action));

interface Loaded {
  i18n: ReturnType<typeof createI18n>;
  nodes: Node[];
}

/** Reads the interface language and the site manager from the Go side, then shows the window. */
export function App() {
  const [loaded, setLoaded] = useState<Loaded>();
  const [adding, setAdding] = useState(false);
  // A host just added: selected when the shell opens again.
  const [added, setAdded] = useState<string>();
  const [scans, setScans] = useState<ScansState>({ running: [], completed: 0 });

  useEffect(() => {
    void Promise.all([Preferences.Language(), SiteManager.Tree()]).then(([lang, tree]) => {
      const i18n = createI18n(lang);
      document.documentElement.lang = i18n.language;
      setLoaded({ i18n, nodes: tree.nodes ?? [] });
    });
    void Scanner.State().then(setScans);
    return Events.On(VigieScans, (event) => setScans(event.data));
  }, []);

  // A scan stored: the statuses of the tree change.
  useEffect(() => {
    if (scans.completed === 0) return;
    void SiteManager.Tree().then((tree) => setLoaded((l) => l && { ...l, nodes: tree.nodes ?? [] }));
  }, [scans.completed]);

  // Reads the tree again, after an edit or a host added.
  const reload = async () => {
    const tree = await SiteManager.Tree();
    setLoaded((l) => l && { ...l, nodes: tree.nodes ?? [] });
  };

  const hostAdded = async (hostId: string) => {
    await reload();
    setAdded(hostId);
    setAdding(false);
  };

  // Both calls stay in the process and answer at once: nothing to show while they run.
  if (!loaded) return null;
  const folders = loaded.nodes.filter((n) => n.kind === "folder").map((n) => ({ id: n.id, path: n.path }));
  return (
    <I18nextProvider i18n={loaded.i18n}>
      {adding ? (
        <AddHostWizard api={wizardApi} folders={folders} onClose={() => setAdding(false)} onAdded={hostAdded} />
      ) : (
        <AppShell
          key={added}
          nodes={loaded.nodes}
          onAddHost={() => setAdding(true)}
          initialSelectedId={added}
          data={shellData}
          scans={scans}
          onScan={(nodeId) => void Scanner.Scan(nodeId)}
          onStopAll={() => void Scanner.CancelAll()}
          commands={menuCommands}
          edit={editApi}
          onChanged={reload}
        />
      )}
    </I18nextProvider>
  );
}
