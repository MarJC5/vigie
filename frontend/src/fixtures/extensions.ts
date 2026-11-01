// Fictional plugins and themes of a site's scan, for stories and tests: never a real site.
import type { ExtensionView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { exploited } from "./findings";

const extension = (e: Partial<ExtensionView> & Pick<ExtensionView, "kind" | "slug" | "title" | "version">): ExtensionView => ({
  status: "active",
  update: "none",
  updateVersion: "",
  autoUpdate: false,
  onWordPressOrg: true,
  worst: null,
  ...e,
});

export const extensions: ExtensionView[] = [
  extension({ kind: "plugin", slug: "woocommerce", title: "WooCommerce", version: "9.8.1", update: "available", updateVersion: "9.8.5", autoUpdate: true, worst: exploited }),
  extension({ kind: "plugin", slug: "akismet", title: "Akismet", version: "5.3", status: "inactive", update: "available", updateVersion: "5.7.2" }),
  extension({ kind: "plugin", slug: "theme-builder-pro", title: "Theme Builder Pro", version: "2.1.0", onWordPressOrg: false }),
  extension({ kind: "plugin", slug: "query-monitor", title: "Query Monitor", version: "3.16.4", status: "must-use", onWordPressOrg: false }),
  extension({ kind: "theme", slug: "kirkibase", title: "Kirki Base", version: "1.0.2", status: "parent", onWordPressOrg: false }),
  extension({ kind: "theme", slug: "twentytwentyfive", title: "Twenty Twenty-Five", version: "1.2", status: "inactive", autoUpdate: true }),
];
