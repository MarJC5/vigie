import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import type {
  HostDetails,
  TestResult,
  WPCLIInstallProgress,
  WPCLIInstallResult,
} from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { checkItems, checkStateLabels } from "../../lib/hosts";
import { Button } from "../Button/Button";
import { CheckList } from "../CheckList/CheckList";
import { Dialog } from "../Dialog/Dialog";
import { Notice } from "../Notice/Notice";
import { WpCliInstallDialog, WpCliNotice, accountOf, wpCliNoticeShown } from "../WpCliInstall/WpCliInstall";

export interface HostTestDialogProps {
  /** The host's name, in the title. */
  name: string;
  /** The connection to test: the stored one, or the one being edited. */
  details: () => Promise<HostDetails>;
  /** Runs the checks of the "add a host" wizard: SSH, PHP, WP-CLI, read-only. */
  test: (d: HostDetails) => Promise<TestResult>;
  onClose: () => void;
  /** Opens the help, where a host that refuses the connection is explained. */
  onHelp?: () => void;
  /** Installs WP-CLI when the test found none (ADR-0039); without it, nothing is offered. */
  install?: (d: HostDetails) => Promise<WPCLIInstallResult>;
  /** Follows that install (the "vigie:wpcli-install" event); returns the unsubscribe. */
  onInstallProgress?: (listener: (p: WPCLIInstallProgress) => void) => () => void;
  /** WP-CLI answers once installed, as this command: a host being edited takes it in its form. */
  onInstalled?: (command: string) => void;
}

/** Tests the connection of a host already added (ADR-0023, point 4), as the wizard does; offers to
 install WP-CLI when the account has none (ADR-0039). */
export function HostTestDialog({ name, details, test, onClose, onHelp, install, onInstallProgress, onInstalled }: HostTestDialogProps) {
  const { t } = useTranslation();
  const [result, setResult] = useState<TestResult | null>(null);
  const [tested, setTested] = useState<HostDetails | null>(null);
  const [running, setRunning] = useState(true);
  // Each attempt runs the test once: the first as the dialog opens, then "Test again".
  const [attempt, setAttempt] = useState(0);
  const [installing, setInstalling] = useState(false);
  const [installed, setInstalled] = useState(false);

  useEffect(() => {
    let current = true;
    const run = async (): Promise<[TestResult, HostDetails | null]> => {
      let d: HostDetails | null = null;
      try {
        d = await details();
        return [await test(d), d];
      } catch (err) {
        return [
          { phpVersion: "", wpCli: "", wpCliVersion: "", install: null, problem: { code: "problem.error", detail: err instanceof Error ? err.message : String(err) } },
          d,
        ];
      }
    };
    void run().then(([r, d]) => {
      if (!current) return;
      setResult(r);
      setTested(d);
      setRunning(false);
    });
    return () => {
      current = false;
    };
  }, [attempt, details, test]);

  const again = () => {
    setResult(null);
    setRunning(true);
    setInstalled(false);
    setAttempt((n) => n + 1);
  };

  const wpCliInstalled = (r: WPCLIInstallResult) => {
    setInstalling(false);
    setResult((before) => before && { ...before, wpCli: r.command, wpCliVersion: r.version, problem: null, install: null });
    setInstalled(true);
    onInstalled?.(r.command);
  };

  const problem = result?.problem;
  const say = (code: string, detail: string) => t(code, { message: detail });
  // The offer only where an install can be run.
  const offered = result && !install ? { ...result, install: null } : result;
  return (
    <>
      <Dialog
        title={t("hostTest.title", { name })}
        lead={t("hostTest.lead")}
        wide
        onClose={onClose}
        actions={
          <>
            <Button onClick={again} disabled={running} aria-busy={running || undefined}>
              {t("hostTest.again")}
            </Button>
            <Button variant="primary" onClick={onClose}>
              {t("overview.close")}
            </Button>
          </>
        }
      >
        <CheckList label={t("wizard.checks")} items={checkItems(result, running, t, installed)} stateLabels={checkStateLabels(t)} />
        {wpCliNoticeShown(offered, installed) ? (
          <WpCliNotice test={offered} installed={installed} onInstall={() => setInstalling(true)} onRetry={again} />
        ) : (
          problem && (
            <Notice
              title={t("wizard.testFailed")}
              text={say(problem.code, problem.detail)}
              detail={problem.detail && problem.detail !== say(problem.code, problem.detail) ? problem.detail : undefined}
              action={onHelp ? { label: t("hostTest.help"), onClick: onHelp } : undefined}
            />
          )
        )}
      </Dialog>
      {installing && install && tested && result?.install && (
        <WpCliInstallDialog
          offer={result.install}
          name={name}
          account={accountOf(tested)}
          install={() => install(tested)}
          onProgress={onInstallProgress}
          onInstalled={wpCliInstalled}
          onClose={() => setInstalling(false)}
        />
      )}
    </>
  );
}
