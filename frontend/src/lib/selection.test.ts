import { describe, expect, it } from "vitest";

import { sampleTree } from "../fixtures/tree";
import { deletion, kindsOf, sitesOf } from "./selection";

const names = (nodes: { name: string }[]) => nodes.map((n) => n.name);

describe("selection", () => {
  it("holds each site once, in the tree's order", () => {
    expect(names(sitesOf(["boulangerie", "shared", "vps"], sampleTree))).toEqual(["atelier-exemple.ch", "boulangerie-exemple.ch", "reseau-ecoles.ch"]);
    expect(sitesOf(["personal"], sampleTree)).toEqual([]);
  });

  it("counts folders, hosts and sites", () => {
    expect(kindsOf(["clients", "vps", "atelier"], sampleTree)).toEqual({ folders: 1, hosts: 1, sites: 1 });
  });

  it("deletes a folder only when nothing is left in it", () => {
    // Personal holds only the VPS, deleted too: it goes. Clients keeps its host: it stays.
    const d = deletion(["personal", "vps", "clients", "atelier"], sampleTree);
    expect(names(d.going)).toEqual(["atelier-exemple.ch", "Personal", "Managed VPS"]);
    expect(names(d.shown)).toEqual(["atelier-exemple.ch", "Personal"]);
    expect(names(d.staying)).toEqual(["Clients"]);
  });

  it("shows a host, not its sites selected too", () => {
    const d = deletion(["shared", "boulangerie"], sampleTree);
    expect(names(d.shown)).toEqual(["Shared hosting"]);
    expect(names(d.going)).toEqual(["Shared hosting", "boulangerie-exemple.ch"]);
  });
});
