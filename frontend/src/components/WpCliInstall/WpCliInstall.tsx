import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import type {
  TestResult,
  WPCLIInstallProgress,
  WPCLIInstallResult,
  WPCLIOffer,
} from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sizeText } from "../../lib/data";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { InfoList, InfoRow } from "../InfoList/InfoList";
import { Notice } from "../Notice/Notice";
import { StepList, type StepState } from "../StepList/StepList";

/** Where WP-CLI goes on the account, as the dialog and the notices say it (ADR-0039). */
const path = "~/bin/wp";

export interface WpCliNoticeProps {
  /** The connection test, when it found no WP-CLI. */
  test: TestResult | null;
  /** WP-CLI was just installed: the test passed with it. */
  installed: boolean;
  onInstall: () => void;
  onRetry: () => void;
}

/** Whether WpCliNotice has something to say about this test, in place of the test's own notice. */
export function wpCliNoticeShown(test: TestResult | null, installed: boolean): boolean {
  const code = test?.problem?.code;
  if (installed && test && !test.problem) return true;
  return (code === "problem.noWpCli" && !!test?.install) || code === "problem.wpCliPhpTooOld" || code === "problem.wpCliExists";
}

/** What the connection test says about WP-CLI when it is missing: the offer to install it, or why
 Vigie cannot; once installed, the host's new command. Null for any other result. */
export function WpCliNotice({ test, installed, onInstall, onRetry }: WpCliNoticeProps) {
  const { t } = useTranslation();
  const problem = test?.problem;
  if (installed && test && !problem) {
    return <Notice tone="ok" role="note" title={t("wpCliInstall.doneTitle")} text={t("wpCliInstall.doneText", { command: test.wpCli, path })} />;
  }
  if (problem?.code === "problem.noWpCli" && test?.install) {
    return (
      <Notice
        tone="warn"
        title={t("wpCliInstall.offerTitle")}
        text={t("wpCliInstall.offerText", { path: test.install.path, version: test.install.version })}
        action={{ label: t("wpCliInstall.offerAction"), onClick: onInstall }}
      />
    );
  }
  if (problem?.code === "problem.wpCliPhpTooOld" || problem?.code === "problem.wpCliExists") {
    return (
      <Notice
        tone="warn"
        title={problem.code === "problem.wpCliExists" ? t("wpCliInstall.existsTitle", { path: problem.detail }) : t("wpCliInstall.oldTitle")}
        text={t(problem.code, { message: problem.detail })}
        action={{ label: t("wizard.retry"), onClick: onRetry }}
      />
    );
  }
  return null;
}

const steps = ["download", "check", "send", "test"] as const;
type Step = (typeof steps)[number];

export interface WpCliInstallDialogProps {
  offer: WPCLIOffer;
  /** The host's name, and its account as the user knows it (alias, or user@host). */
  name: string;
  account: string;
  /** Installs WP-CLI on the account (app.HostWizard.InstallWPCLI). */
  install: () => Promise<WPCLIInstallResult>;
  /** Follows the install (the "vigie:wpcli-install" event); returns the unsubscribe. */
  onProgress?: (listener: (p: WPCLIInstallProgress) => void) => () => void;
  /** WP-CLI answers: the command that did, and its version. */
  onInstalled: (result: WPCLIInstallResult) => void;
  onClose: () => void;
}

/** Asks before installing WP-CLI on an account (what is written, where it comes from, how it is
 checked), then shows the install step by step, and where it stopped if it failed (ADR-0039). */
export function WpCliInstallDialog({ offer, name, account, install, onProgress, onInstalled, onClose }: WpCliInstallDialogProps) {
  const { t, i18n } = useTranslation();
  const [phase, setPhase] = useState<"confirm" | "running" | "failed">("confirm");
  const [progress, setProgress] = useState<WPCLIInstallProgress>({ step: "download", done: 0, total: 0 });
  const [result, setResult] = useState<WPCLIInstallResult | null>(null);
  const size = (bytes: number) => sizeText(bytes, i18n.language);

  useEffect(() => {
    if (phase !== "running" || !onProgress) return;
    return onProgress(setProgress);
  }, [phase, onProgress]);

  const start = async () => {
    setPhase("running");
    let r: WPCLIInstallResult;
    try {
      r = await install();
    } catch (err) {
      r = { command: "", version: "", failedStep: progress.step, problem: { code: "problem.error", detail: err instanceof Error ? err.message : String(err) } };
    }
    if (!r.problem) {
      onInstalled(r);
      return;
    }
    setResult(r);
    setPhase("failed");
  };

  if (phase === "confirm") {
    return (
      <Dialog
        title={t("wpCliInstall.confirmTitle", { name })}
        lead={t("wpCliInstall.confirmLead", { account })}
        wide
        onClose={onClose}
        actions={
          <>
            <Button onClick={onClose}>{t("wpCliInstall.cancel")}</Button>
            <Button variant="primary" onClick={() => void start()}>
              {t("wpCliInstall.confirm")}
            </Button>
          </>
        }
      >
        <InfoList>
          <InfoRow label={t("wpCliInstall.file")}>
            <code>{offer.path}</code>
          </InfoRow>
          <InfoRow label={t("wpCliInstall.wpCli")}>{t("wpCliInstall.wpCliValue", { version: offer.version, size: size(offer.size) })}</InfoRow>
          <InfoRow label={t("wpCliInstall.source")}>{t("wpCliInstall.sourceValue")}</InfoRow>
          <InfoRow label={t("wpCliInstall.check")}>{t("wpCliInstall.checkValue")}</InfoRow>
          <InfoRow label={t("wpCliInstall.command")}>{t("wpCliInstall.commandValue", { path: offer.path })}</InfoRow>
          <InfoRow label={t("wpCliInstall.remove")}>
            <code>{t("wpCliInstall.removeCommand", { path: offer.path })}</code>
          </InfoRow>
        </InfoList>
      </Dialog>
    );
  }

  const running = phase === "running";
  const at = steps.indexOf((running ? progress.step : result?.failedStep || progress.step) as Step);
  const stateOf = (i: number): StepState => (i < at ? "done" : i > at ? "waiting" : running ? "running" : "failed");
  const detail = (step: Step): string => {
    if (step === "send" && running && progress.step === "send" && progress.total > 0) {
      return t("wpCliInstall.detail.sendRunning", { done: size(progress.done), total: size(progress.total) });
    }
    return t(`wpCliInstall.detail.${step}`, { size: size(offer.size), version: offer.version, path: offer.path });
  };
  const problem = result?.problem;
  const said = problem ? t(problem.code, { message: problem.detail }) : "";
  return (
    // An install under way cannot be stopped halfway: Escape waits for its end.
    <Dialog
      title={t("wpCliInstall.runTitle")}
      lead={t("wpCliInstall.runLead", { name, account })}
      wide
      onClose={running ? () => {} : onClose}
      progress={running ? { label: t("wpCliInstall.progress", { step: at + 1, total: steps.length }), value: at, max: steps.length } : undefined}
      actions={
        <Button variant="primary" onClick={onClose} disabled={running} aria-busy={running || undefined}>
          {running ? t("wpCliInstall.installing") : t("wpCliInstall.close")}
        </Button>
      }
    >
      <StepList
        label={t("wpCliInstall.stepsLabel")}
        ended={!running}
        stateLabels={{
          done: t("wpCliInstall.state.done"),
          running: t("wpCliInstall.state.running"),
          failed: t("wpCliInstall.state.failed"),
          waiting: t("wpCliInstall.state.waiting"),
        }}
        rows={steps.map((step, i) => ({
          key: step,
          state: stateOf(i),
          label: t(`wpCliInstall.step.${step}`),
          detail: detail(step),
          end: t(`wpCliInstall.state.${stateOf(i)}`),
        }))}
      />
      {problem && (
        <Notice
          title={t(`wpCliInstall.failed.${result?.failedStep || "download"}`)}
          text={said}
          detail={problem.detail && !said.includes(problem.detail) ? problem.detail : undefined}
        />
      )}
    </Dialog>
  );
}

/** The account of a host as the user knows it: its alias, else user@host. */
export function accountOf(d: { alias: string; user: string; hostname: string }): string {
  return d.alias || `${d.user}@${d.hostname}`;
}
