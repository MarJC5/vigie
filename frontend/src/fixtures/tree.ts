// Fictional site manager used by stories and tests: never real hosts or sites.
import type { Node, TagView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";

const node = (n: Partial<Node> & Pick<Node, "id" | "kind" | "name">): Node => ({
  parentId: "",
  color: "none",
  ownColor: "none",
  path: n.name,
  notes: "",
  tags: [],
  multisite: false,
  subsites: 0,
  status: "",
  updates: 0,
  ...n,
});

export const sampleTree: Node[] = [
  node({ id: "clients", kind: "folder", name: "Clients", color: "blue", ownColor: "blue", path: "Clients" }),
  node({ id: "shared", parentId: "clients", kind: "host", name: "Shared hosting", color: "blue", path: "Clients/Shared hosting", updates: 5 }),
  node({ id: "atelier", parentId: "shared", kind: "site", name: "atelier-exemple.ch", color: "blue", path: "Clients/Shared hosting/atelier-exemple.ch", status: "ok", tags: ["Client Dupont"] }),
  node({ id: "boulangerie", parentId: "shared", kind: "site", name: "boulangerie-exemple.ch", color: "orange", ownColor: "orange", path: "Clients/Shared hosting/boulangerie-exemple.ch", status: "warn", updates: 5, tags: ["Client Dupont", "Prioritaire"], notes: "**Client**: Boulangerie Exemple.\n\n### Contrat\n\n- Maintenance mensuelle" }),
  node({ id: "ecoles", parentId: "shared", kind: "site", name: "reseau-ecoles.ch", color: "blue", path: "Clients/Shared hosting/reseau-ecoles.ch", multisite: true, subsites: 14 }),
  node({ id: "personal", kind: "folder", name: "Personal", path: "Personal" }),
  node({ id: "vps", parentId: "personal", kind: "host", name: "Managed VPS", path: "Personal/Managed VPS" }),
];

// The tags in use on the fictional sites, with how many sites have each.
export const sampleTags: TagView[] = [
  { name: "Client Dupont", sites: 3 },
  { name: "Contrat de maintenance", sites: 6 },
  { name: "Hébergement à migrer", sites: 1 },
  { name: "Prioritaire", sites: 2 },
  { name: "Priorité basse", sites: 1 },
];
