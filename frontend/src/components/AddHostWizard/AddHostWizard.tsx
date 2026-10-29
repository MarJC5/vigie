import { useState } from "react";
import { useTranslation } from "react-i18next";

import type {
  AddResult,
  Choice,
  DiscoverResult,
  DiscoveryProgress,
  HostDetails,
  Install,
  KeysResult,
  Problem,
  TestResult,
} from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { type CheckItem, CheckList } from "../CheckList/CheckList";
import { Checkbox } from "../Checkbox/Checkbox";
import { ChoiceCard } from "../ChoiceCard/ChoiceCard";
import { Column, Columns } from "../Columns/Columns";
import { Fingerprint } from "../Fingerprint/Fingerprint";
import { type InstallItem, InstallList } from "../InstallList/InstallList";
import { Notice } from "../Notice/Notice";
import { Progress } from "../Progress/Progress";
import { SelectField } from "../SelectField/SelectField";
import { TextField } from "../TextField/TextField";
import { WizardStep } from "../WizardStep/WizardStep";

/** The Go service the wizard calls (app.HostWizard); stories and tests give a fake. */
export interface WizardApi {
  Check(d: HostDetails): Promise<Problem | null>;
  HostKeys(d: HostDetails): Promise<KeysResult>;
  Trust(name: string, fingerprint: string): Promise<Problem | null>;
  TestConnection(d: HostDetails): Promise<TestResult>;
  Discover(d: HostDetails): Promise<DiscoverResult>;
  Add(d: HostDetails, fingerprint: string, choices: Choice[] | null): Promise<AddResult>;
  /** Follows the checks of Discover (the "vigie:discovery" event); returns the unsubscribe. */
  onDiscovery?(listener: (p: DiscoveryProgress) => void): () => void;
}

export interface AddHostWizardProps {
  api: WizardApi;
  /** The folders a host can go in, with their path ("Clients/Valais"). */
  folders: Array<{ id: string; path: string }>;
  onClose: () => void;
  onAdded: (hostId: string) => void;
}

type Step = "connection" | "key" | "test" | "sites";
const steps: Step[] = ["connection", "key", "test", "sites"];

interface Form {
  mode: "host" | "alias";
  name: string;
  alias: string;
  hostname: string;
  port: string;
  user: string;
  keyPath: string;
  wpCli: string;
  folderId: string;
}

type Field = "name" | "alias" | "hostname" | "port" | "user" | "folder";

// Problems of the details, shown under their field rather than in a notice.
const fieldOf: Record<string, (mode: Form["mode"]) => Field> = {
  "problem.emptyName": () => "name",
  "problem.slashInName": () => "name",
  "problem.duplicateName": () => "name",
  "problem.noConnection": (mode) => (mode === "alias" ? "alias" : "hostname"),
  "problem.noUser": () => "user",
  "problem.invalidPort": () => "port",
  "problem.folderNotFound": () => "folder",
};

function details(form: Form): HostDetails {
  const byAlias = form.mode === "alias";
  return {
    name: form.name.trim(),
    alias: byAlias ? form.alias.trim() : "",
    hostname: byAlias ? "" : form.hostname.trim(),
    port: byAlias ? 0 : Number(form.port) || 0,
    user: byAlias ? "" : form.user.trim(),
    keyPath: form.keyPath.trim(),
    wpCli: form.wpCli.trim(),
    folderId: form.folderId,
  };
}

/** The "add a host" wizard: connection, server key, test, then the installs to add. */
export function AddHostWizard({ api, folders, onClose, onAdded }: AddHostWizardProps) {
  const { t } = useTranslation();
  const [step, setStep] = useState<Step>("connection");
  const [form, setForm] = useState<Form>({
    mode: "host", name: "", alias: "", hostname: "", port: "22", user: "", keyPath: "", wpCli: "wp", folderId: "",
  });
  const [errors, setErrors] = useState<Partial<Record<Field, string>>>({});
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);
  // What the wizard waits for, shown with a progress bar.
  const [waiting, setWaiting] = useState<"keys" | "search" | "adding" | null>(null);
  const [checking, setChecking] = useState<DiscoveryProgress | null>(null);
  const [keys, setKeys] = useState<KeysResult | null>(null);
  const [compared, setCompared] = useState(false);
  const [test, setTest] = useState<TestResult | null>(null);
  const [installs, setInstalls] = useState<Array<Install & { checked: boolean }>>([]);

  const set = (field: keyof Form) => (value: string) => setForm((f) => ({ ...f, [field]: value }));
  const say = (p: Problem) => t(p.code, { message: p.detail });

  // Runs a call to the Go side: busy meanwhile, any failure becomes a notice.
  const call = async <T,>(run: () => Promise<T>): Promise<T | null> => {
    setBusy(true);
    setProblem(null);
    try {
      return await run();
    } catch (err) {
      setProblem({ code: "problem.error", detail: err instanceof Error ? err.message : String(err) });
      return null;
    } finally {
      setBusy(false);
    }
  };

  const keyList = keys?.keys ?? [];
  const shown = keyList.find((k) => k.type === "ssh-ed25519") ?? keyList[0];

  const connect = async () => {
    const port = Number(form.port);
    if (form.mode === "host" && (!Number.isInteger(port) || port < 1 || port > 65535)) {
      setErrors({ port: t("problem.invalidPort") });
      return;
    }
    const d = details(form);
    const checked = await call(() => api.Check(d));
    if (checked) {
      const field = fieldOf[checked.code];
      if (field) setErrors({ [field(form.mode)]: say(checked) });
      else setProblem(checked);
      return;
    }
    setErrors({});
    setWaiting("keys");
    const result = await call(() => api.HostKeys(d));
    setWaiting(null);
    if (!result) return;
    if (result.problem) return setProblem(result.problem);
    setKeys(result);
    setCompared(false);
    setStep("key");
  };

  const runTest = async () => {
    setTest(null);
    const result = await call(() => api.TestConnection(details(form)));
    if (!result) return;
    setTest(result);
    if (result.problem) return setProblem(result.problem);
    // WP-CLI may answer under another command than the one entered: the host keeps that one.
    if (result.wpCli) setForm((f) => ({ ...f, wpCli: result.wpCli }));
  };

  const trust = async () => {
    if (!keys || !shown) return;
    const refused = await call(() => api.Trust(keys.name, shown.fingerprint));
    if (refused) return setProblem(refused);
    setStep("test");
    // The test starts by itself once the key is trusted.
    await runTest();
  };

  const search = async () => {
    setWaiting("search");
    const unsubscribe = api.onDiscovery?.(setChecking);
    const result = await call(() => api.Discover(details(form)));
    unsubscribe?.();
    setChecking(null);
    setWaiting(null);
    if (!result) return;
    if (result.problem) return setProblem(result.problem);
    setInstalls((result.installs ?? []).map((i) => ({ ...i, checked: i.state === "new" })));
    setStep("sites");
  };

  const chosen = installs.filter((i) => i.checked && i.state === "new");
  const add = async () => {
    if (!shown) return;
    const choices: Choice[] = chosen.map((i) => ({ path: i.path, name: i.name.trim(), url: i.url, multisite: i.multisite }));
    setWaiting("adding");
    const result = await call(() => api.Add(details(form), shown.fingerprint, choices));
    setWaiting(null);
    if (!result) return;
    if (result.problem) return setProblem(result.problem);
    onAdded(result.hostId);
  };

  const back = (to: Step) => () => {
    setProblem(null);
    setStep(to);
  };

  const ready =
    form.name.trim() !== "" && (form.mode === "alias" ? form.alias.trim() !== "" : form.hostname.trim() !== "" && form.user.trim() !== "");
  const testPassed = test !== null && !test.problem;
  const host = form.mode === "alias" ? form.alias.trim() : form.hostname.trim();

  const primary = {
    connection: { label: t("wizard.action.continue"), onClick: connect, disabled: !ready },
    key: { label: t("wizard.action.trust"), onClick: trust, disabled: !compared },
    test: { label: t("wizard.action.search"), onClick: search, disabled: !testPassed },
    sites: {
      label: chosen.length > 0 ? t("wizard.action.add", { count: chosen.length }) : t("wizard.action.addHostOnly"),
      onClick: add,
    },
  }[step];

  const previous: Partial<Record<Step, Step>> = { key: "connection", test: "key", sites: "test" };
  const backTo = previous[step];

  const progress =
    waiting === "search" && checking && checking.total > 0 ? (
      <Progress
        label={t("wizard.progress.checking", { checked: checking.checked, total: checking.total })}
        value={checking.checked}
        max={checking.total}
      />
    ) : (
      waiting && <Progress label={t(`wizard.progress.${waiting}`)} />
    );

  const notice = problem && (
    <Notice
      title={step === "test" ? t("wizard.testFailed") : t("wizard.title." + step)}
      text={say(problem)}
      detail={problem.detail && problem.detail !== say(problem) ? problem.detail : undefined}
      action={step === "test" ? { label: t("wizard.retry"), onClick: () => void runTest() } : undefined}
    />
  );

  return (
    <WizardStep
      title={t(`wizard.title.${step}`)}
      lead={
        step === "sites"
          ? t("wizard.lead.sites", { count: installs.length })
          : t(`wizard.lead.${step}`, { host })
      }
      steps={steps.map((s) => t(`wizard.steps.${s}`))}
      current={steps.indexOf(step)}
      stepsLabel={t("wizard.stepsLabel")}
      doneLabel={t("wizard.done")}
      closeLabel={t("wizard.close")}
      onClose={onClose}
      back={backTo && { label: t("wizard.back"), onClick: back(backTo), disabled: busy }}
      primary={{ ...primary, busy }}
      width={step === "sites" ? "wide" : "narrow"}
    >
      {step === "connection" && (
        <>
          <Columns count={2}>
            <ChoiceCard
              title={t("wizard.mode.host")}
              text={t("wizard.mode.hostText")}
              selected={form.mode === "host"}
              onSelect={() => setForm((f) => ({ ...f, mode: "host" }))}
            />
            <ChoiceCard
              title={t("wizard.mode.alias")}
              text={t("wizard.mode.aliasText")}
              selected={form.mode === "alias"}
              onSelect={() => setForm((f) => ({ ...f, mode: "alias" }))}
            />
          </Columns>
          <Columns count={6}>
            <Column span={3}>
              <TextField label={t("wizard.field.name")} value={form.name} onChange={set("name")} error={errors.name} autoFocus />
            </Column>
            <Column span={3}>
              <SelectField
                label={t("wizard.field.folder")}
                value={form.folderId}
                options={[{ value: "", label: t("wizard.field.root") }, ...folders.map((f) => ({ value: f.id, label: f.path }))]}
                onChange={set("folderId")}
              />
            </Column>
            {form.mode === "alias" ? (
              <Column span={6}>
                <TextField
                  label={t("wizard.field.alias")}
                  value={form.alias}
                  onChange={set("alias")}
                  hint={t("wizard.field.aliasHint")}
                  error={errors.alias}
                />
              </Column>
            ) : (
              <>
                <Column span={3}>
                  <TextField label={t("wizard.field.hostname")} value={form.hostname} onChange={set("hostname")} error={errors.hostname} />
                </Column>
                <Column span={1}>
                  <TextField label={t("wizard.field.port")} value={form.port} onChange={set("port")} inputMode="numeric" error={errors.port} />
                </Column>
                <Column span={2}>
                  <TextField label={t("wizard.field.user")} value={form.user} onChange={set("user")} error={errors.user} />
                </Column>
              </>
            )}
            <Column span={3}>
              <TextField
                label={t("wizard.field.keyPath")}
                value={form.keyPath}
                onChange={set("keyPath")}
                placeholder={t("wizard.field.keyPathPlaceholder")}
                hint={t("wizard.field.keyPathHint")}
              />
            </Column>
            <Column span={3}>
              <TextField label={t("wizard.field.wpCli")} value={form.wpCli} onChange={set("wpCli")} hint={t("wizard.field.wpCliHint")} />
            </Column>
          </Columns>
          {progress}
          {notice}
        </>
      )}

      {step === "key" && shown && keys && (
        <>
          <Fingerprint
            label={t("wizard.keyOf", { host })}
            type={keyType(shown.type)}
            value={shown.fingerprint}
            others={keyList
              .filter((k) => k !== shown)
              .map((k) => ({ type: keyType(k.type), value: k.fingerprint }))}
            othersLabel={t("wizard.otherKeys")}
            note={t("wizard.keyWhere")}
          />
          <Checkbox label={t("wizard.compared")} checked={compared} onChange={setCompared} />
          {progress}
          {notice}
        </>
      )}

      {step === "test" && (
        <>
          <CheckList label={t("wizard.checks")} items={checkItems(test, busy, t)} stateLabels={{
            ok: t("wizard.state.ok"),
            failed: t("wizard.state.failed"),
            waiting: t("wizard.state.waiting"),
            running: t("wizard.state.running"),
          }} />
          {progress}
          {notice}
        </>
      )}

      {step === "sites" && (
        <>
          <InstallList
            label={t("wizard.found")}
            columns={{
              add: t("wizard.col.add"),
              name: t("wizard.col.name"),
              path: t("wizard.col.path"),
              state: t("wizard.col.state"),
            }}
            addLabel={t("wizard.col.add")}
            items={installs.map((i) => installItem(i, t))}
            onToggle={(path) => setInstalls((list) => list.map((i) => (i.path === path ? { ...i, checked: !i.checked } : i)))}
            onRename={(path, name) => setInstalls((list) => list.map((i) => (i.path === path ? { ...i, name } : i)))}
          />
          {progress}
          {notice}
        </>
      )}
    </WizardStep>
  );
}

type T = (key: string, vars?: Record<string, unknown>) => string;

// Key types by the names people know: "ssh-ed25519" is ED25519, "ecdsa-sha2-nistp256" is ECDSA.
function keyType(type: string): string {
  if (type.startsWith("ecdsa-")) return "ECDSA";
  return type.replace(/^ssh-/, "").toUpperCase();
}

// The three checks of the test, from its result: what failed stops the ones after it.
function checkItems(test: TestResult | null, running: boolean, t: T): CheckItem[] {
  const labels = [t("wizard.check.ssh"), t("wizard.check.php"), t("wizard.check.wpCli")];
  if (!test) {
    return labels.map((label, i) => ({ label, value: "", state: running && i === 0 ? "running" : "waiting" }));
  }
  const code = test.problem?.code ?? "";
  const sshFailed = code.startsWith("failure.") || code === "problem.error";
  const phpFailed = code === "problem.noPhp";
  const wpFailed = code === "problem.noWpCli";
  const failedText = test.problem ? t(test.problem.code, { message: test.problem.detail }) : "";
  return [
    sshFailed
      ? { label: labels[0]!, value: failedText, state: "failed" }
      : { label: labels[0]!, value: t("wizard.check.connected"), state: "ok" },
    sshFailed
      ? { label: labels[1]!, value: "", state: "waiting" }
      : phpFailed
        ? { label: labels[1]!, value: failedText, state: "failed" }
        : { label: labels[1]!, value: test.phpVersion, state: "ok" },
    sshFailed || phpFailed
      ? { label: labels[2]!, value: "", state: "waiting" }
      : wpFailed
        ? { label: labels[2]!, value: failedText, state: "failed" }
        : { label: labels[2]!, value: `${test.wpCliVersion} (${test.wpCli})`, state: "ok" },
  ];
}

function installItem(i: Install & { checked: boolean }, t: T): InstallItem {
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
