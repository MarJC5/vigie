import { Events } from "@wailsio/runtime";
import { useEffect, useState } from "react";
import { I18nextProvider } from "react-i18next";

import { VigieDiscovery } from "../bindings/events";
import { HostWizard, Preferences, SiteManager } from "../bindings/github.com/MarJC5/vigie/internal/app";
import type { Node } from "../bindings/github.com/MarJC5/vigie/internal/app/models";
import { AddHostWizard, type WizardApi } from "./components/AddHostWizard/AddHostWizard";
import { AppShell } from "./components/AppShell/AppShell";
import { createI18n } from "./i18n";

// The wizard's service, with the progress of discovery as the Go side sends it.
const wizardApi: WizardApi = {
  ...HostWizard,
  onDiscovery: (listener) => Events.On(VigieDiscovery, (event) => listener(event.data)),
};

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

  useEffect(() => {
    void Promise.all([Preferences.Language(), SiteManager.Tree()]).then(([lang, tree]) => {
      const i18n = createI18n(lang);
      document.documentElement.lang = i18n.language;
      setLoaded({ i18n, nodes: tree.nodes ?? [] });
    });
  }, []);

  const hostAdded = async (hostId: string) => {
    const tree = await SiteManager.Tree();
    setLoaded((l) => l && { ...l, nodes: tree.nodes ?? [] });
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
        <AppShell key={added} nodes={loaded.nodes} onAddHost={() => setAdding(true)} initialSelectedId={added} />
      )}
    </I18nextProvider>
  );
}
