import { describe, expect, it } from "vitest";

import type { ExtensionView, Node } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { activation, listOf, openKey, subsiteId, subsiteOf, withSubsites } from "./subsites";

const node = (n: Partial<Node>): Node => ({
  id: "", parentId: "", kind: "site", name: "", color: "none", ownColor: "none", path: "", notes: "", tags: [],
  multisite: false, subsites: 0, subsiteList: [], status: "", updates: 0, ...n,
});

describe("subsites", () => {
  it("address a subsite by its network and ID", () => {
    expect(subsiteId("net", 2)).toBe("subsite:net:2");
    expect(subsiteOf("subsite:net:2")).toEqual({ siteId: "net", blogId: 2 });
    expect(subsiteOf("net")).toBeUndefined();
    expect(subsiteOf("subsite:net:x")).toBeUndefined();
    expect(openKey("net")).toBe("open:net");
  });

  it("put a network's subsites right under it, as nodes of their own", () => {
    const net = node({ id: "net", parentId: "host", name: "reseau.example.test", path: "Clients/Bench/reseau.example.test", multisite: true,
      subsites: 2, subsiteList: [{ blogId: 1, label: "/", state: "public" }, { blogId: 4, label: "/gamma", state: "archived" }] });
    const other = node({ id: "other", parentId: "host", name: "atelier.example.test" });
    const got = withSubsites([node({ id: "host", kind: "host" }), net, other]);
    expect(got.map((n) => [n.id, n.parentId, n.kind, n.name, n.status])).toEqual([
      ["host", "", "host", "", ""],
      ["net", "host", "site", "reseau.example.test", ""],
      ["subsite:net:1", "net", "subsite", "/", "public"],
      ["subsite:net:4", "net", "subsite", "/gamma", "archived"],
      ["other", "host", "site", "atelier.example.test", ""],
    ]);
    expect(got[3]!.path).toBe("Clients/Bench/reseau.example.test/gamma");
  });

  it("say where a plugin or theme is active on a network", () => {
    const e = (x: Partial<ExtensionView>): ExtensionView => ({
      kind: "plugin", slug: "x", title: "", status: "inactive", version: "", update: "none", updateVersion: "", autoUpdate: false,
      onWordPressOrg: true, worst: null, ignore: null, subsites: [], ...x,
    });
    expect(activation(e({ status: "active-network" }))).toEqual({ kind: "network", sites: [] });
    expect(activation(e({ subsites: [2, 3] }))).toEqual({ kind: "some", sites: [2, 3] });
    expect(activation(e({}))).toEqual({ kind: "nowhere", sites: [] });
    expect(activation(e({ subsites: null }))).toBeUndefined();
  });

  it("list subsites in words", () => {
    expect(listOf(["/alpha", "/beta"], "fr")).toBe("/alpha et /beta");
    expect(listOf(["/alpha", "/beta", "/gamma"], "en")).toBe("/alpha, /beta, and /gamma");
  });
});
