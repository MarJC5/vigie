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
  WPCLIInstallProgress,
  WPCLIInstallResult,
} from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { CheckList } from "../CheckList/CheckList";
import { Checkbox } from "../Checkbox/Checkbox";
import { ChoiceCard } from "../ChoiceCard/ChoiceCard";
import { Column, Columns } from "../Columns/Columns";
import { Fingerprint } from "../Fingerprint/Fingerprint";
import { InstallList } from "../InstallList/InstallList";
import { Notice } from "../Notice/Notice";
import type { ProgressProps } from "../Progress/Progress";
import { SelectField } from "../SelectField/SelectField";
import { TextField } from "../TextField/TextField";
import { WizardStep } from "../WizardStep/WizardStep";
import { WpCliInstallDialog, WpCliNotice, accountOf, wpCliNoticeShown } from "../WpCliInstall/WpCliInstall";
import { checkItems, checkStateLabels, installItem, keyType } from "../../lib/hosts";

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
  /** Installs WP-CLI on the account when the test found none (ADR-0039); hostId "" here. */
  InstallWPCLI(d: HostDetails, hostId: string): Promise<WPCLIInstallResult>;
  /** Follows that install (the "vigie:wpcli-install" event); returns the unsubscribe. */
  onWpCliInstall?(listener: (p: WPCLIInstallProgress) => void): () => void;
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
    // Commands at a time: the default (2) until the host is edited.
    maxConcurrent: 0,
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
  // The test passed but the search of the account failed: the notice and its retry are the search's.
  const [searchFailed, setSearchFailed] = useState(false);
  const [checking, setChecking] = useState<DiscoveryProgress | null>(null);
  const [keys, setKeys] = useState<KeysResult | null>(null);
  const [compared, setCompared] = useState(false);
  const [test, setTest] = useState<TestResult | null>(null);
  // WP-CLI being installed from the test step, then installed: the test passes with it.
  const [installing, setInstalling] = useState(false);
  const [installed, setInstalled] = useState(false);
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

  // Tests the connection, then, when it passes, searches the account at once: both only read.
  const runTest = async () => {
    setTest(null);
    setInstalled(false);
    const result = await call(() => api.TestConnection(details(form)));
    if (!result) return;
    setTest(result);
    if (result.problem) return setProblem(result.problem);
    // WP-CLI may answer under another command than the one entered: the host keeps that one.
    const tested = result.wpCli ? { ...form, wpCli: result.wpCli } : form;
    setForm(tested);
    await search(tested);
  };

  const trust = async () => {
    if (!keys || !shown) return;
    const refused = await call(() => api.Trust(keys.name, shown.fingerprint));
    if (refused) return setProblem(refused);
    setStep("test");
    // The test starts by itself once the key is trusted.
    await runTest();
  };

  const search = async (of: Form = form) => {
    setSearchFailed(false);
    setWaiting("search");
    const unsubscribe = api.onDiscovery?.(setChecking);
    const result = await call(() => api.Discover(details(of)));
    unsubscribe?.();
    setChecking(null);
    setWaiting(null);
    if (!result || result.problem) {
      setSearchFailed(true);
      if (result?.problem) setProblem(result.problem);
      return;
    }
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

  // WP-CLI answers once installed: the host keeps its command, the test passes; the search waits
  // for the user, who reads the notice first.
  const wpCliInstalled = (r: WPCLIInstallResult) => {
    setInstalling(false);
    setForm((f) => ({ ...f, wpCli: r.command }));
    setTest((before) => before && { ...before, wpCli: r.command, wpCliVersion: r.version, problem: null, install: null });
    setProblem(null);
    setInstalled(true);
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
    // Searches again after a search that failed; a passing test searches by itself.
    test: { label: t("wizard.action.search"), onClick: () => void search(), disabled: !testPassed },
    sites: {
      label: chosen.length > 0 ? t("wizard.action.add", { count: chosen.length }) : t("wizard.action.addHostOnly"),
      onClick: add,
    },
  }[step];

  const previous: Partial<Record<Step, Step>> = { key: "connection", test: "key", sites: "test" };
  const backTo = previous[step];

  // The wait under way, its bar along the bottom of the window.
  const progress: ProgressProps | undefined =
    waiting === "search" && checking && checking.total > 0
      ? {
          label: t("wizard.progress.checking", { checked: checking.checked, total: checking.total }),
          value: checking.checked,
          max: checking.total,
        }
      : waiting
        ? { label: t(`wizard.progress.${waiting}`) }
        : undefined;

  const notice = problem && (
    <Notice
      title={step === "test" ? (searchFailed ? t("wizard.searchFailed") : t("wizard.testFailed")) : t("wizard.title." + step)}
      text={say(problem)}
      detail={problem.detail && problem.detail !== say(problem) ? problem.detail : undefined}
      action={step === "test" ? { label: t("wizard.retry"), onClick: () => void (searchFailed ? search() : runTest()) } : undefined}
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
      progress={progress}
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
          {notice}
        </>
      )}

      {step === "test" && (
        <>
          <CheckList label={t("wizard.checks")} items={checkItems(test, busy, t, installed)} stateLabels={checkStateLabels(t)} />
          {wpCliNoticeShown(test, installed) ? (
            <WpCliNotice test={test} installed={installed} onInstall={() => setInstalling(true)} onRetry={() => void runTest()} />
          ) : (
            notice
          )}
          {installing && test?.install && (
            <WpCliInstallDialog
              offer={test.install}
              name={form.name.trim()}
              account={accountOf(details(form))}
              install={() => api.InstallWPCLI(details(form), "")}
              onProgress={api.onWpCliInstall}
              onInstalled={wpCliInstalled}
              onClose={() => setInstalling(false)}
            />
          )}
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
          {notice}
        </>
      )}
    </WizardStep>
  );
}


