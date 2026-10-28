import type { Decorator, Preview } from "@storybook/react-vite";
import { I18nextProvider } from "react-i18next";

import "../src/styles/base.css";
import { createI18n } from "../src/i18n";

// Every story renders in both themes side by side, so the accessibility check covers both.
const themes: Decorator = (Story, context) => {
  const i18n = createI18n(String(context.globals.lang ?? "en"));
  return (
    <I18nextProvider i18n={i18n}>
      <div style={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))" }}>
        {(["dark", "light"] as const).map((theme) => (
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
  },
  initialGlobals: { lang: "en" },
  parameters: {
    layout: "fullscreen",
    a11y: { test: "error" },
  },
};

export default preview;
