import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import type { HostDetails, TestResult } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { checkItems } from "../../lib/hosts";
import { Button } from "../Button/Button";
import { CheckList } from "../CheckList/CheckList";
import { Dialog } from "../Dialog/Dialog";
import { Notice } from "../Notice/Notice";

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
}

/** Tests the connection of a host already added (ADR-0023, point 4), as the wizard does. */
export function HostTestDialog({ name, details, test, onClose, onHelp }: HostTestDialogProps) {
  const { t } = useTranslation();
  const [result, setResult] = useState<TestResult | null>(null);
  const [running, setRunning] = useState(true);
  // Each attempt runs the test once: the first as the dialog opens, then "Test again".
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let current = true;
    const run = async (): Promise<TestResult> => {
      try {
        return await test(await details());
      } catch (err) {
        return { phpVersion: "", wpCli: "", wpCliVersion: "", problem: { code: "problem.error", detail: err instanceof Error ? err.message : String(err) } };
      }
    };
    void run().then((r) => {
      if (!current) return;
      setResult(r);
      setRunning(false);
    });
    return () => {
      current = false;
    };
  }, [attempt, details, test]);

  const again = () => {
    setResult(null);
    setRunning(true);
    setAttempt((n) => n + 1);
  };

  const problem = result?.problem;
  const say = (code: string, detail: string) => t(code, { message: detail });
  return (
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
      <CheckList
        label={t("wizard.checks")}
        items={checkItems(result, running, t)}
        stateLabels={{
          ok: t("wizard.state.ok"),
          failed: t("wizard.state.failed"),
          waiting: t("wizard.state.waiting"),
          running: t("wizard.state.running"),
        }}
      />
      {problem && (
        <Notice
          title={t("wizard.testFailed")}
          text={say(problem.code, problem.detail)}
          detail={problem.detail && problem.detail !== say(problem.code, problem.detail) ? problem.detail : undefined}
          action={onHelp ? { label: t("hostTest.help"), onClick: onHelp } : undefined}
        />
      )}
    </Dialog>
  );
}
