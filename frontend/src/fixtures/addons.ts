// Fictional add-ons used by stories and tests: never real ones.
import type { AddonView, AddonsView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";

const both = (en: string, fr: string) => ({ en, fr });

// A folder into a server, the kind of image an add-on brings.
const image = "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAzNjAgMjAwIiB3aWR0aD0iMzYwIiBoZWlnaHQ9IjIwMCI+CiAgPHJlY3Qgd2lkdGg9IjM2MCIgaGVpZ2h0PSIyMDAiIGZpbGw9IiNlMjZmNTYiLz4KICA8ZyBmaWxsPSJub25lIiBzdHJva2U9IiNmZmZmZmYiIHN0cm9rZS13aWR0aD0iNyIgc3Ryb2tlLWxpbmVqb2luPSJyb3VuZCIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIj4KICAgIDxwYXRoIGQ9Ik03MCA3MGgyOGwxMCAxMmg0NnY1OEg3MHoiLz4KICAgIDxwYXRoIGQ9Ik0xNjggMTA0aDI2bS0xMC0xMiAxMiAxMi0xMiAxMiIvPgogICAgPHJlY3QgeD0iMjE0IiB5PSI2NiIgd2lkdGg9Ijc2IiBoZWlnaHQ9IjMwIiByeD0iNSIvPgogICAgPHJlY3QgeD0iMjE0IiB5PSIxMDYiIHdpZHRoPSI3NiIgaGVpZ2h0PSIzMCIgcng9IjUiLz4KICA8L2c+CiAgPGcgZmlsbD0iI2ZmZmZmZiI+CiAgICA8Y2lyY2xlIGN4PSIyNzIiIGN5PSI4MSIgcj0iNSIvPgogICAgPGNpcmNsZSBjeD0iMjcyIiBjeT0iMTIxIiByPSI1Ii8+CiAgPC9nPgo8L3N2Zz4K";

export const listSource: AddonView = {
  name: "sitelist",
  title: both("Site list", "Liste de sites"),
  author: "Someone",
  release: "0.2.0",
  kind: "source",
  summary: both("Turns a list of servers into hosts of Vigie.", "Transforme une liste de serveurs en hébergements Vigie."),
  description: both(
    "Reads a list of servers and writes a Vigie export: each server with a user becomes a host. Vigie then finds the WordPress installs of each host.",
    "Lit une liste de serveurs et écrit un export Vigie\u00a0: chaque serveur avec un utilisateur devient un hébergement. Vigie trouve ensuite les installations WordPress de chaque hébergement.",
  ),
  details: [
    { label: both("Reads", "Lit"), value: both("The list you choose", "La liste que vous choisissez") },
    { label: both("Network", "Réseau"), value: both("Connects to nothing", "Ne se connecte à rien") },
  ],
  image,
  settings: [
    { id: "file", kind: "file", label: both("List", "Liste"), help: both("The list to read", "La liste à lire"), default: "", required: true, choices: [] },
    { id: "all", kind: "checkbox", label: both("Every server, also without a user", "Tous les serveurs, même sans utilisateur"), help: {}, default: "false", required: false, choices: [] },
  ],
  installed: true,
  path: "/opt/homebrew/bin/vigie-sitelist",
  install: "https://example.test/vigie-sitelist",
  problem: null,
};

export const cloudDestination: AddonView = {
  ...listSource,
  name: "cloudsave",
  title: both("Cloud save", "Sauvegarde en ligne"),
  kind: "destination",
  summary: both("Saves the exports of the site manager to a cloud storage.", "Enregistre les exports du gestionnaire de sites dans un stockage en ligne."),
  image: "",
  settings: [],
  installed: false,
  path: "",
  install: "https://example.test/vigie-cloudsave",
};

export const addonsView: AddonsView = {
  installed: [listSource],
  library: [{ ...listSource, path: "", settings: [] }, cloudDestination],
  problem: null,
};
