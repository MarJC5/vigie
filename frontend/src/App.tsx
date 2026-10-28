import { useEffect, useState } from "react";
import { I18nextProvider } from "react-i18next";

import { Preferences, SiteManager } from "../bindings/github.com/MarJC5/vigie/internal/app";
import type { Node } from "../bindings/github.com/MarJC5/vigie/internal/app/models";
import { AppShell } from "./components/AppShell/AppShell";
import { createI18n } from "./i18n";

interface Loaded {
  i18n: ReturnType<typeof createI18n>;
  nodes: Node[];
}

/** Reads the interface language and the site manager from the Go side, then shows the window. */
export function App() {
  const [loaded, setLoaded] = useState<Loaded>();

  useEffect(() => {
    void Promise.all([Preferences.Language(), SiteManager.Tree()]).then(([lang, tree]) => {
      const i18n = createI18n(lang);
      document.documentElement.lang = i18n.language;
      setLoaded({ i18n, nodes: tree.nodes ?? [] });
    });
  }, []);

  // Both calls stay in the process and answer at once: nothing to show while they run.
  if (!loaded) return null;
  return (
    <I18nextProvider i18n={loaded.i18n}>
      <AppShell nodes={loaded.nodes} />
    </I18nextProvider>
  );
}
