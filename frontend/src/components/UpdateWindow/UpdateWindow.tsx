import { useTranslation } from "react-i18next";

import type {
  OperationView,
  Problem,
  SitePreviewView,
  UpdateItemView,
} from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { backupPath, canStart, failedStep, itemName, itemsDone, rescanRow, restoredStep, rolledBackBy, rowText, runRows } from "../../lib/updates";
import { Notice } from "../Notice/Notice";
import { RunSteps } from "../RunSteps/RunSteps";
import { UpdatePreview } from "../UpdatePreview/UpdatePreview";
import type { WizardAction } from "../WizardStep/WizardStep";
import { WizardStep } from "../WizardStep/WizardStep";

export interface UpdateWindowProps {
  /** The site's name. */
  name: string;
  /** The preview read from the server; null while Vigie reads it. */
  preview: SitePreviewView | null;
  /** The items of the preview chosen, in the order of the run. */
  planned: UpdateItemView[];
  /** The titles of the plugins and themes, by key. */
  names: Map<string, string>;
  /** The plugins wordpress.org knows, for their changelogs. */
  known?: Set<string>;
  /** Opens a changelog in the browser. */
  onOpen?: (href: string) => void;
  /** The update, then its rollback, as they run and once ended; null before the update starts. */
  operation: OperationView | null;
  /** The rollbacks Vigie ran on its own during the update (ADR-0042), as they run and once ended. */
  rollbacks?: OperationView[];
  /** The update or rollback was asked for and Vigie has not reported its first step yet. */
  starting?: boolean;
  /** The run asked for, or followed, is a rollback. */
  rollingBack?: boolean;
  /** "Stop after this item" was asked for. */
  stopping?: boolean;
  /** Why Vigie refused to start (the site is busy, the preview changed). */
  problem?: Problem | null;
  onClose: () => void;
  onConfirm: () => void;
  onStop: () => void;
  onRollback: (operationID: string) => void;
}

/** The update of one site: what it changes, its steps as they go, then how it ended, with the
 rollback offered after a failure. */
export function UpdateWindow({
  name,
  preview,
  planned,
  names,
  known,
  onOpen,
  operation,
  rollbacks = [],
  starting = false,
  rollingBack = false,
  stopping = false,
  problem,
  onClose,
  onConfirm,
  onStop,
  onRollback,
}: UpdateWindowProps) {
  const { t } = useTranslation();
  const rollback = rollingBack || operation?.kind === "rollback";
  const running = starting || operation?.outcome === "running";
  const ended = !!operation && !running;
  const steps = [t("update.steps.preview"), t("update.steps.run"), t("update.steps.result")];
  const close: WizardAction = { label: t("update.result.close"), onClick: onClose };

  const refusal = problem ? t(problem.code, { message: problem.detail }) : "";
  const refused = problem && (
    <Notice
      title={t("update.preview.cannot")}
      text={refusal}
      detail={problem.detail && !refusal.includes(problem.detail) ? problem.detail : undefined}
    />
  );

  if (!operation && !starting) {
    return (
      <WizardStep
        title={t("update.preview.title", { name })}
        lead={t("update.preview.lead")}
        steps={steps}
        current={0}
        stepsLabel={t("wizard.stepsLabel")}
        doneLabel={t("wizard.done")}
        closeLabel={t("wizard.close")}
        onClose={onClose}
        back={{ label: t("action.cancel"), onClick: onClose }}
        primary={{
          label: t("update.preview.confirm"),
          onClick: onConfirm,
          disabled: !preview || !canStart(preview, planned),
          busy: !preview,
        }}
        width="wide"
        progress={preview || problem ? undefined : { label: t("update.preview.loading") }}
      >
        {refused}
        {preview && <UpdatePreview preview={preview} planned={planned} names={names} known={known} onOpen={onOpen} />}
      </WizardStep>
    );
  }

  const recorded = runRows(operation, rollback ? [] : planned, rollbacks);
  // While it runs, the list ends with the scan that closes every run.
  const rows = running && operation ? [...recorded, rescanRow(recorded)] : recorded;
  // Every item planned is checked after its update, the translations once at the end.
  const total = planned.filter((it) => it.kind !== "language").length + (planned.some((it) => it.kind === "language") ? 1 : 0);
  const done = Math.min(itemsDone(operation), total);
  // The items gone back on their own (ADR-0042): each said in a notice, counted in the progress.
  const gone = operation && !rollback ? rolledBackBy(operation, rollbacks) : [];
  const goneNotices = gone.map((b) => {
    const item = itemName(t, b.kind, b.slug, names);
    return (
      <Notice
        key={`${b.kind}:${b.slug}`}
        tone="warn"
        role="note"
        title={t("update.run.rolledBackTitle", { item, version: b.version, backTo: b.backTo })}
        text={t(running ? "update.run.rolledBackText" : "update.result.rolledBackText", { item, version: b.version })}
        detail={running ? undefined : b.reason || undefined}
      />
    );
  });
  const progressText =
    t("update.run.progress", { count: done, done, total }) + (gone.length > 0 ? `, ${t("update.run.rolledBack", { count: gone.length })}` : "");
  const list = <RunSteps label={t("update.run.stepsLabel")} rows={rows} names={names} ended={ended} />;

  if (running) {
    return (
      <WizardStep
        title={rollback ? t("update.run.rollbackTitle", { name }) : t("update.run.title", { name })}
        lead={t("update.run.lead")}
        steps={steps}
        current={1}
        stepsLabel={t("wizard.stepsLabel")}
        doneLabel={t("wizard.done")}
        closeLabel={t("wizard.close")}
        onClose={onClose}
        back={
          rollback
            ? undefined
            : {
                label: stopping ? t("update.run.stopping") : t("update.run.stopAfter"),
                onClick: onStop,
                // Nothing left to stop once the closing scan runs.
                disabled: stopping || starting || rescanRow(recorded).state === "running",
              }
        }
        primary={{ label: t("update.run.running"), onClick: () => {}, busy: true }}
        width="wide"
        progress={
          rollback
            ? { label: t("update.run.rollingBack") }
            : { label: progressText, value: Math.min(done + gone.length, total), max: total }
        }
      >
        {goneNotices}
        {list}
      </WizardStep>
    );
  }

  const op = operation!;
  const failed = failedStep(op);
  let title: string;
  let lead: string;
  let notice = null;
  let back: WizardAction | undefined;
  let primary = close;
  if (rollback) {
    const restored = restoredStep(op);
    const item = restored ? itemName(t, restored.itemKind, restored.slug, names) : "";
    if (op.outcome === "completed") {
      title = t("update.result.rolledBackTitle", { name });
      lead = t("update.result.rolledBackLead", { item });
    } else {
      title = t("update.result.rollbackFailedTitle");
      lead = `${t("update.result.rollbackFailedLead")} ${backupPath(op.backupDir)}`;
      if (failed) notice = <Notice title={rowText(t, { key: "", state: "failed", step: failed }, names, true).label} text={failed.message} />;
    }
  } else if (op.outcome === "completed") {
    title =
      gone.length > 0
        ? t("update.result.doneExceptTitle", { name, items: gone.map((b) => itemName(t, b.kind, b.slug, names)).join(", ") })
        : t("update.result.doneTitle", { name });
    lead = t("update.result.doneLead");
  } else if (op.outcome === "failed" && rollbacks.some((b) => b.outcome === "failed")) {
    // A rollback on its own that failed: the site is not as it was, its backup stays, and the
    // rollback is offered to try again.
    const backFailed = rollbacks.find((b) => b.outcome === "failed")!;
    const restored = restoredStep(backFailed);
    title = t("update.result.rollbackFailedTitle");
    lead = `${t("update.result.rollbackFailedLead")} ${backupPath(op.backupDir)}`;
    notice = (
      <Notice
        title={t("update.result.rollbackOnItsOwnFailedTitle", {
          item: restored ? itemName(t, restored.itemKind, restored.slug, names) : "",
          backTo: restored?.versionAfter ?? "",
        })}
        text={t("update.result.rollbackOnItsOwnFailedText")}
        detail={failedStep(backFailed)?.message}
      />
    );
    if (op.rollback) {
      back = { label: t("update.result.keep"), onClick: onClose };
      primary = { label: t("update.result.rollback"), onClick: () => onRollback(op.id) };
    }
  } else if (op.outcome === "failed") {
    title = t("update.result.failedTitle");
    lead = t("update.result.failedLead");
    const what = failed
      ? failed.step === "check"
        ? t("update.result.failedOn", { item: itemName(t, failed.itemKind, failed.slug, names) })
        : t("update.result.failedStep", { step: rowText(t, { key: "", state: "failed", step: failed }, names, true).label })
      : title;
    const offered = op.rollback;
    const explain = !offered
      ? t("update.result.nothingToRollBack")
      : offered.kind === "core"
        ? `${t("update.result.rollbackCore")} ${t("update.result.rollbackCoreNever")}`
        : t("update.result.rollbackFiles", { item: itemName(t, offered.kind, offered.slug, names) });
    notice = <Notice title={what} text={explain} detail={failed?.message} />;
    if (offered) {
      back = { label: t("update.result.keep"), onClick: onClose };
      primary = { label: t("update.result.rollback"), onClick: () => onRollback(op.id) };
    }
  } else if (op.outcome === "cancelled") {
    title = t("update.result.cancelledTitle");
    lead = t("update.result.cancelledLead");
  } else {
    title = t("update.result.interruptedTitle");
    lead = t("update.result.interruptedLead");
  }

  return (
    <WizardStep
      title={title}
      lead={lead}
      steps={steps}
      current={2}
      stepsLabel={t("wizard.stepsLabel")}
      doneLabel={t("wizard.done")}
      closeLabel={t("wizard.close")}
      onClose={onClose}
      back={back}
      primary={primary}
      width="wide"
    >
      {refused}
      {notice}
      {goneNotices}
      {list}
    </WizardStep>
  );
}
