import type { Install, TestResult } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";
import type { CheckItem, CheckState } from "../components/CheckList/CheckList";
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

/** What each state of a check is called, next to its dot and for screen readers. */
export function checkStateLabels(t: T): Record<CheckState, string> {
  return {
    ok: t("wizard.state.ok"),
    warn: t("wizard.state.warn"),
    failed: t("wizard.state.failed"),
    waiting: t("wizard.state.waiting"),
    running: t("wizard.state.running"),
  };
}

/** The three checks of a connection test, from its result: what failed stops the ones after it.
 installed: WP-CLI answers because Vigie just installed it (ADR-0039). */
export function checkItems(test: TestResult | null, running: boolean, t: T, installed = false): CheckItem[] {
  const labels = [t("wizard.check.ssh"), t("wizard.check.php"), t("wizard.check.wpCli")];
  if (!test) {
    return labels.map((label, i) => ({ label, value: "", state: running && i === 0 ? "running" : "waiting" }));
  }
  const code = test.problem?.code ?? "";
  const detail = test.problem?.detail ?? "";
  const sshFailed = code.startsWith("failure.") || code === "problem.error";
  const phpFailed = code === "problem.noPhp";
  const phpTooOld = code === "problem.wpCliPhpTooOld";
  const failedText = test.problem ? t(test.problem.code, { message: detail }) : "";
  const wpCli = (): CheckItem => {
    const label = labels[2]!;
    switch (code) {
      case "problem.noWpCli":
        return { label, value: t("wizard.check.wpCliNotFound", { tried: detail }), state: "failed" };
      case "problem.wpCliExists":
        return { label, value: t("wizard.check.wpCliExists", { path: detail }), state: "failed" };
      case "":
        return {
          label,
          value: installed
            ? t("wizard.check.wpCliInstalled", { version: test.wpCliVersion, command: test.wpCli })
            : `${test.wpCliVersion} (${test.wpCli})`,
          state: "ok",
        };
    }
    return { label, value: failedText, state: "failed" };
  };
  return [
    sshFailed
      ? { label: labels[0]!, value: failedText, state: "failed" }
      : { label: labels[0]!, value: t("wizard.check.connected"), state: "ok" },
    sshFailed
      ? { label: labels[1]!, value: "", state: "waiting" }
      : phpFailed
        ? { label: labels[1]!, value: failedText, state: "failed" }
        : { label: labels[1]!, value: test.phpVersion, state: phpTooOld ? "warn" : "ok" },
    sshFailed || phpFailed || phpTooOld ? { label: labels[2]!, value: "", state: "waiting" } : wpCli(),
  ];
}
