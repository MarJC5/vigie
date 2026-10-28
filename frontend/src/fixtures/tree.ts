// Fictional site manager used by stories and tests: never real hosts or sites.
import type { Node } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";

const node = (n: Partial<Node> & Pick<Node, "id" | "kind" | "name">): Node => ({
  parentId: "",
  color: "none",
  path: n.name,
  multisite: false,
  subsites: 0,
  ...n,
});

export const sampleTree: Node[] = [
  node({ id: "clients", kind: "folder", name: "Clients", color: "blue", path: "Clients" }),
  node({ id: "shared", parentId: "clients", kind: "host", name: "Shared hosting", color: "blue", path: "Clients/Shared hosting" }),
  node({ id: "atelier", parentId: "shared", kind: "site", name: "atelier-exemple.ch", color: "blue", path: "Clients/Shared hosting/atelier-exemple.ch" }),
  node({ id: "boulangerie", parentId: "shared", kind: "site", name: "boulangerie-exemple.ch", color: "orange", path: "Clients/Shared hosting/boulangerie-exemple.ch" }),
  node({ id: "ecoles", parentId: "shared", kind: "site", name: "reseau-ecoles.ch", color: "blue", path: "Clients/Shared hosting/reseau-ecoles.ch", multisite: true, subsites: 14 }),
  node({ id: "personal", kind: "folder", name: "Personal", path: "Personal" }),
  node({ id: "vps", parentId: "personal", kind: "host", name: "Managed VPS", path: "Personal/Managed VPS" }),
];
