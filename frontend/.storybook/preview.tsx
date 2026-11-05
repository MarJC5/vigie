import type { Decorator, Preview } from "@storybook/react-vite";
import { I18nextProvider } from "react-i18next";

import "../src/styles/base.css";
import { createI18n } from "../src/i18n";

type Theme = "dark" | "light";

// In Storybook, the theme chosen in the toolbar. The tests (make frontend-test) render every story
// in both themes side by side, so the plays and the accessibility check cover both.
const themes: Decorator = (Story, context) => {
  const i18n = createI18n(String(context.globals.lang ?? "en"));
  const chosen = String(context.globals.theme ?? "dark");
  const shown: Theme[] = import.meta.env.STORYBOOK_BOTH_THEMES || chosen === "both" ? ["dark", "light"] : [chosen === "light" ? "light" : "dark"];
  return (
    <I18nextProvider i18n={i18n}>
      <div style={{ display: "grid", gridTemplateColumns: `repeat(${shown.length}, minmax(0, 1fr))` }}>
        {shown.map((theme) => (
          <div
            key={theme}
            data-theme={theme}
            lang={i18n.language}
            style={{ padding: 24, background: "var(--vg-surface-main)", color: "var(--vg-text)" }}
          >
            <Story />
          </div>
        ))}
      </div>
    </I18nextProvider>
  );
};

const preview: Preview = {
  decorators: [themes],
  globalTypes: {
    lang: {
      description: "Interface language",
      toolbar: { title: "Language", items: ["en", "fr"], dynamicTitle: true },
    },
    theme: {
      description: "Theme of the window",
      toolbar: {
        title: "Theme",
        icon: "mirror",
        items: [
          { value: "dark", title: "Dark", icon: "moon" },
          { value: "light", title: "Light", icon: "sun" },
          { value: "both", title: "Side by side", icon: "sidebar" },
        ],
        dynamicTitle: true,
      },
    },
  },
  initialGlobals: { lang: "en", theme: "dark" },
  parameters: {
    layout: "fullscreen",
    a11y: { test: "error" },
  },
};

export default preview;
