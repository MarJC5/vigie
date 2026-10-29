import type { Install } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";
import type { InstallItem } from "../components/InstallList/InstallList";

type T = (key: string, vars?: Record<string, unknown>) => string;

/** Key types by the names people know: "ssh-ed25519" is ED25519, "ecdsa-sha2-nistp256" is ECDSA. */
export function keyType(type: string): string {
  if (type.startsWith("ecdsa-")) return "ECDSA";
  return type.replace(/^ssh-/, "").toUpperCase();
}

/** A WordPress install found on an account, as a row of the install list: only a new one can be added. */
export function installItem(i: Install & { checked: boolean }, t: T): InstallItem {
  const state = {
    new: i.multisite ? t("wizard.network") : t("found.new"),
    "already-added": t("found.alreadyAdded"),
    "not-installed": t("found.notInstalled"),
    "cannot-load": t("found.cannotLoad"),
  }[i.state] ?? i.state;
  const detail = i.state === "cannot-load" ? i.message : i.multisite ? t("manager.subsites", { count: i.subsites }) : i.url;
  return {
    path: i.path,
    name: i.name,
    state,
    detail,
    checked: i.checked,
    selectable: i.state === "new",
    problem: i.state === "cannot-load",
  };
}
