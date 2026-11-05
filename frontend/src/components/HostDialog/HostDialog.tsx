import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import type {
  EditHostResult,
  HostDetails,
  KeysResult,
  Problem,
  TestResult,
} from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { keyType } from "../../lib/hosts";
import { Button } from "../Button/Button";
import { Checkbox } from "../Checkbox/Checkbox";
import { Column, Columns } from "../Columns/Columns";
import { Dialog } from "../Dialog/Dialog";
import { Fingerprint } from "../Fingerprint/Fingerprint";
import { LinkButton } from "../LinkButton/LinkButton";
import { Notice } from "../Notice/Notice";
import { TextField } from "../TextField/TextField";
import styles from "./HostDialog.module.css";

/** The Go services the dialog calls (app.Editor and app.HostWizard); stories give a fake. */
export interface HostEditApi {
  Host(id: string): Promise<HostDetails>;
  EditHost(id: string, d: HostDetails): Promise<EditHostResult>;
  HostKeys(d: HostDetails): Promise<KeysResult>;
  Trust(name: string, fingerprint: string): Promise<Problem | null>;
  KeepKey(hostId: string, fingerprint: string): Promise<Problem | null>;
  TestConnection(d: HostDetails): Promise<TestResult>;
}

export interface HostDialogProps {
  api: HostEditApi;
  hostId: string;
  /** How many sites use the connection, for the lead. */
  sites: number;
  onClose: () => void;
  /** Saved, and its key trusted again when the address changed. */
  onSaved: () => void;
  /** Tests the connection as it is in the form, before saving it; takeCommand puts in the form the
   command of a WP-CLI installed from the test (ADR-0039). */
  onTest?: (d: HostDetails, takeCommand: (command: string) => void) => void;
}

interface Form {
  name: string;
  alias: string;
  hostname: string;
  port: string;
  user: string;
  keyPath: string;
  wpCli: string;
  maxConcurrent: string;
}

type Field = "name" | "alias" | "hostname" | "port" | "user" | "maxConcurrent";

// Problems of the details, shown under their field rather than in a notice.
const fieldOf: Record<string, (alias: boolean) => Field> = {
  "problem.emptyName": () => "name",
  "problem.slashInName": () => "name",
  "problem.duplicateName": () => "name",
  "problem.noConnection": (alias) => (alias ? "alias" : "hostname"),
  "problem.noUser": () => "user",
  "problem.invalidPort": () => "port",
};

function formOf(d: HostDetails): Form {
  return {
    name: d.name, alias: d.alias, hostname: d.hostname, port: d.port ? String(d.port) : "22", user: d.user,
    keyPath: d.keyPath, wpCli: d.wpCli, maxConcurrent: d.maxConcurrent ? String(d.maxConcurrent) : "",
  };
}

/** Edits the name and connection of a host; a new address asks to check the server's key again. */
export function HostDialog({ api, hostId, sites, onClose, onSaved, onTest }: HostDialogProps) {
  const { t } = useTranslation();
  const [stored, setStored] = useState<HostDetails | null>(null);
  const [form, setForm] = useState<Form | null>(null);
  const [errors, setErrors] = useState<Partial<Record<Field, string>>>({});
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);
  const [keys, setKeys] = useState<KeysResult | null>(null);
  const [compared, setCompared] = useState(false);

  useEffect(() => {
    let current = true;
    void api.Host(hostId).then((d) => {
      if (!current) return;
      setStored(d);
      setForm(formOf(d));
    });
    return () => {
      current = false;
    };
  }, [api, hostId]);

  if (!stored || !form) return null;
  const alias = stored.alias !== "";
  const set = (field: keyof Form) => (value: string) => setForm((f) => f && { ...f, [field]: value });
  const say = (p: Problem) => t(p.code, { message: p.detail });
  const details = (): HostDetails => ({
    ...stored,
    name: form.name.trim(),
    alias: alias ? form.alias.trim() : "",
    hostname: alias ? "" : form.hostname.trim(),
    port: alias ? 0 : Number(form.port) || 0,
    user: alias ? "" : form.user.trim(),
    keyPath: form.keyPath.trim(),
    wpCli: form.wpCli.trim(),
    maxConcurrent: Number(form.maxConcurrent) || 0,
  });
  const address = alias ? form.alias.trim() : form.hostname.trim();
  const moved = alias
    ? form.alias.trim() !== stored.alias
    : form.hostname.trim() !== stored.hostname || (Number(form.port) || 22) !== (stored.port || 22);

  // Runs a call to the Go side: busy meanwhile, a failure becomes a notice.
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

  const save = async () => {
    const port = Number(form.port);
    const max = Number(form.maxConcurrent);
    const invalid: Partial<Record<Field, string>> = {};
    if (!alias && (!Number.isInteger(port) || port < 1 || port > 65535)) invalid.port = t("problem.invalidPort");
    // Empty means the default; otherwise 1 to 10.
    if (form.maxConcurrent.trim() !== "" && (!Number.isInteger(max) || max < 1 || max > 10)) {
      invalid.maxConcurrent = t("edit.host.maxInvalid");
    }
    setErrors(invalid);
    if (Object.keys(invalid).length > 0) return;
    const d = details();
    const result = await call(() => api.EditHost(hostId, d));
    if (!result) return;
    if (result.problem) {
      const field = fieldOf[result.problem.code];
      if (field) setErrors({ [field(alias)]: say(result.problem) });
      else setProblem(result.problem);
      return;
    }
    if (!result.keyCleared) return onSaved();
    const found = await call(() => api.HostKeys(d));
    if (!found) return;
    if (found.problem) return setProblem(found.problem);
    setKeys(found);
  };

  const keyList = keys?.keys ?? [];
  const shown = keyList.find((k) => k.type === "ssh-ed25519") ?? keyList[0];
  const trust = async () => {
    if (!keys || !shown) return;
    const refused = await call(() => api.Trust(keys.name, shown.fingerprint));
    if (refused) return setProblem(refused);
    const kept = await call(() => api.KeepKey(hostId, shown.fingerprint));
    if (kept) return setProblem(kept);
    onSaved();
  };

  const notice = problem && <Notice title={t("edit.host.title")} text={say(problem)} detail={problem.detail && problem.detail !== say(problem) ? problem.detail : undefined} />;

  if (keys && shown) {
    return (
      <Dialog
        wide
        title={t("edit.key.title", { host: address })}
        lead={t("edit.key.lead")}
        onClose={onClose}
        actions={
          <>
            <Button variant="outline" onClick={onClose}>
              {t("edit.cancel")}
            </Button>
            <Button variant="primary" disabled={!compared || busy} aria-busy={busy || undefined} onClick={() => void trust()}>
              {t("edit.key.trust")}
            </Button>
          </>
        }
      >
        <Fingerprint
          label={t("wizard.keyOf", { host: address })}
          type={keyType(shown.type)}
          value={shown.fingerprint}
          others={keyList.filter((k) => k !== shown).map((k) => ({ type: keyType(k.type), value: k.fingerprint }))}
          othersLabel={t("wizard.otherKeys")}
          note={t("wizard.keyWhere")}
        />
        <Checkbox label={t("wizard.compared")} checked={compared} onChange={setCompared} />
        {notice}
      </Dialog>
    );
  }

  return (
    <Dialog
      wide
      title={t("edit.host.title")}
      lead={t("edit.host.lead", { name: stored.name, count: sites })}
      onClose={onClose}
      actions={
        <>
          {onTest && (
            <LinkButton className={styles.test} icon="ok" onClick={() => onTest(details(), set("wpCli"))}>
              {t("edit.host.test")}
            </LinkButton>
          )}
          <Button variant="outline" onClick={onClose}>
            {t("edit.cancel")}
          </Button>
          <Button variant="primary" disabled={busy} aria-busy={busy || undefined} onClick={() => void save()}>
            {t("edit.host.save")}
          </Button>
        </>
      }
    >
      <Columns count={6}>
        <Column span={6}>
          <TextField label={t("wizard.field.name")} value={form.name} onChange={set("name")} error={errors.name} />
        </Column>
        {alias ? (
          <Column span={6}>
            <TextField label={t("wizard.field.alias")} value={form.alias} onChange={set("alias")} hint={t("wizard.field.aliasHint")} error={errors.alias} />
          </Column>
        ) : (
          <>
            <Column span={4}>
              <TextField label={t("wizard.field.hostname")} value={form.hostname} onChange={set("hostname")} error={errors.hostname} />
            </Column>
            <Column span={2}>
              <TextField label={t("wizard.field.port")} value={form.port} onChange={set("port")} inputMode="numeric" error={errors.port} />
            </Column>
            <Column span={3}>
              <TextField label={t("wizard.field.user")} value={form.user} onChange={set("user")} error={errors.user} />
            </Column>
          </>
        )}
        <Column span={alias ? 6 : 3}>
          <TextField label={t("wizard.field.wpCli")} value={form.wpCli} onChange={set("wpCli")} placeholder="wp" hint={t("wizard.field.wpCliHint")} />
        </Column>
        <Column span={6}>
          <TextField
            label={t("wizard.field.keyPath")}
            value={form.keyPath}
            onChange={set("keyPath")}
            placeholder={t("wizard.field.keyPathPlaceholder")}
            hint={t("wizard.field.keyPathHint")}
          />
        </Column>
        <Column span={3}>
          <TextField
            label={t("edit.host.maxConcurrent")}
            value={form.maxConcurrent}
            onChange={set("maxConcurrent")}
            placeholder="2"
            inputMode="numeric"
            hint={t("edit.host.maxConcurrentHint")}
            error={errors.maxConcurrent}
          />
        </Column>
      </Columns>
      {moved && (
        <Notice tone="warn" role="status" text={t("edit.host.addressChanged", { host: address })} />
      )}
      {notice}
    </Dialog>
  );
}
