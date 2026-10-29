import { describe, expect, it } from "vitest";

import { installItem, keyType } from "./hosts";

describe("keyType", () => {
  it.each([
    ["ssh-ed25519", "ED25519"],
    ["ecdsa-sha2-nistp256", "ECDSA"],
    ["ssh-rsa", "RSA"],
  ])("%s is %s", (type, want) => {
    expect(keyType(type)).toBe(want);
  });
});

describe("installItem", () => {
  const t = (key: string) => key;
  const base = { path: "~/sites/a", name: "a.example.test", url: "https://a.example.test", multisite: false, subsites: 0, message: "", checked: true };
  it("offers only a new install", () => {
    expect(installItem({ ...base, state: "new" }, t)).toMatchObject({ selectable: true, state: "found.new", detail: "https://a.example.test" });
    expect(installItem({ ...base, state: "already-added" }, t)).toMatchObject({ selectable: false, state: "found.alreadyAdded" });
  });
  it("shows why an install cannot be loaded", () => {
    expect(installItem({ ...base, state: "cannot-load", message: "Error establishing a database connection" }, t)).toMatchObject({
      selectable: false, problem: true, detail: "Error establishing a database connection",
    });
  });
});
