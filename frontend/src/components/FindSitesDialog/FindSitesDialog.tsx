import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import type {
  Choice,
  DiscoverResult,
  DiscoveryProgress,
  Install,
  Problem,
} from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { installItem } from "../../lib/hosts";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { InstallList } from "../InstallList/InstallList";
import { Notice } from "../Notice/Notice";
import { Progress } from "../Progress/Progress";

/** The Go service the dialog calls (app.HostWizard); stories give a fake. */
export interface FindSitesApi {
  DiscoverHost(hostId: string): Promise<DiscoverResult>;
  AddSites(hostId: string, choices: Choice[] | null): Promise<Problem | null>;
  /** Follows the checks of DiscoverHost (the "vigie:discovery" event); returns the unsubscribe. */
  onDiscovery?(listener: (p: DiscoveryProgress) => void): () => void;
}

export interface FindSitesDialogProps {
  api: FindSitesApi;
  hostId: string;
  hostName: string;
  onClose: () => void;
  onAdded: () => void;
}

/** Searches a stored host's account again and adds the new installs chosen. */
export function FindSitesDialog({ api, hostId, hostName, onClose, onAdded }: FindSitesDialogProps) {
  const { t } = useTranslation();
  const [installs, setInstalls] = useState<Array<Install & { checked: boolean }> | null>(null);
  const [checking, setChecking] = useState<DiscoveryProgress | null>(null);
  const [problem, setProblem] = useState<Problem | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    let current = true;
    const unsubscribe = api.onDiscovery?.((p) => current && setChecking(p));
    void api
      .DiscoverHost(hostId)
      .then((result) => {
        if (!current) return;
        if (result.problem) setProblem(result.problem);
        setInstalls((result.installs ?? []).map((i) => ({ ...i, checked: i.state === "new" })));
      })
      .catch((err: unknown) => current && setProblem({ code: "problem.error", detail: String(err) }))
      .finally(() => unsubscribe?.());
    return () => {
      current = false;
      unsubscribe?.();
    };
  }, [api, hostId]);

  const chosen = (installs ?? []).filter((i) => i.checked && i.state === "new");
  const offered = (installs ?? []).some((i) => i.state === "new");
  const say = (p: Problem) => t(p.code, { message: p.detail });

  const add = async () => {
    setBusy(true);
    setProblem(null);
    try {
      const refused = await api.AddSites(
        hostId,
        chosen.map((i) => ({ path: i.path, name: i.name.trim(), url: i.url, multisite: i.multisite })),
      );
      if (refused) return setProblem(refused);
      onAdded();
    } catch (err) {
      setProblem({ code: "problem.error", detail: String(err) });
    } finally {
      setBusy(false);
    }
  };

  return (
    <Dialog
      wide
      title={t("edit.find.title")}
      lead={t("edit.find.lead", { name: hostName })}
      onClose={onClose}
      actions={
        <>
          <Button variant="outline" onClick={onClose}>
            {offered ? t("edit.cancel") : t("edit.find.close")}
          </Button>
          {offered && (
            <Button variant="primary" disabled={chosen.length === 0 || busy} onClick={() => void add()}>
              {t("edit.find.add", { count: chosen.length })}
            </Button>
          )}
        </>
      }
    >
      {installs === null && !problem &&
        (checking && checking.total > 0 ? (
          <Progress
            label={t("wizard.progress.checking", { checked: checking.checked, total: checking.total })}
            value={checking.checked}
            max={checking.total}
          />
        ) : (
          <Progress label={t("edit.find.searching")} />
        ))}
      {installs !== null && installs.length > 0 && (
        <InstallList
          label={t("wizard.found")}
          columns={{ add: t("wizard.col.add"), name: t("wizard.col.name"), path: t("wizard.col.path"), state: t("wizard.col.state") }}
          addLabel={t("wizard.col.add")}
          items={installs.map((i) => installItem(i, t))}
          onToggle={(path) => setInstalls((list) => list && list.map((i) => (i.path === path ? { ...i, checked: !i.checked } : i)))}
          onRename={(path, name) => setInstalls((list) => list && list.map((i) => (i.path === path ? { ...i, name } : i)))}
        />
      )}
      {installs !== null && !offered && !problem && <p style={{ margin: 0 }}>{t("edit.find.none")}</p>}
      {problem && (
        <Notice
          title={t("edit.find.title")}
          text={say(problem)}
          detail={problem.detail && problem.detail !== say(problem) ? problem.detail : undefined}
        />
      )}
    </Dialog>
  );
}
