import { useEffect, useRef, useState } from "react";

import type { AddonsView, FileChoice, ImportPreview, Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { defaultsOf } from "../../lib/addons";
import { AddonPage } from "../AddonPage/AddonPage";
import { AddonsPage, type AddonsTab } from "../AddonsPage/AddonsPage";

/** A call that can be stopped, as the Wails bindings return. */
export interface Cancellable<T> extends Promise<T> {
  cancel?: () => void;
}

/** The Go service of the Add-ons page (app.Addons). */
export interface AddonsApi {
  List(): Promise<AddonsView>;
  ChooseFile(): Promise<FileChoice>;
  Run(name: string, values: Record<string, string>): Cancellable<ImportPreview>;
}

export interface AddonsFlowProps {
  api: AddonsApi;
  /** Opens an address in the browser. */
  onLink: (href: string) => void;
  /** A source wrote an export: its import preview opens. */
  onPreview: (preview: ImportPreview) => void;
}

/** The Add-ons page and the page of one add-on, with the run of a source. */
export function AddonsFlow({ api, onLink, onPreview }: AddonsFlowProps) {
  const [view, setView] = useState<AddonsView | null>(null);
  const [tab, setTab] = useState<AddonsTab>("library");
  const [open, setOpen] = useState<string>();
  const [values, setValues] = useState<Record<string, string>>({});
  const [running, setRunning] = useState(false);
  const [failure, setFailure] = useState<{ problem: Problem; report: string } | null>(null);
  const run = useRef<Cancellable<ImportPreview> | null>(null);

  useEffect(() => {
    void api.List().then(setView);
    // Leaving the page stops a run still going.
    return () => run.current?.cancel?.();
  }, [api]);

  const addon = open ? view?.installed?.find((a) => a.name === open) ?? view?.library?.find((a) => a.name === open) : undefined;

  if (addon) {
    return (
      <AddonPage
        addon={addon}
        values={values}
        onValue={(id, value) => setValues((v) => ({ ...v, [id]: value }))}
        onChooseFile={(id) =>
          void api.ChooseFile().then((choice) => {
            if (choice.file) setValues((v) => ({ ...v, [id]: choice.file }));
          })
        }
        onRun={() => {
          setRunning(true);
          setFailure(null);
          const call = api.Run(addon.name, values);
          run.current = call;
          void call
            .then((preview) => {
              if (preview.problem) setFailure({ problem: preview.problem, report: preview.report });
              else onPreview(preview);
            })
            .catch(() => undefined)
            .finally(() => {
              run.current = null;
              setRunning(false);
            });
        }}
        running={running}
        failure={failure}
        onBack={() => {
          run.current?.cancel?.();
          setOpen(undefined);
          setFailure(null);
        }}
        onLink={onLink}
      />
    );
  }
  return (
    <AddonsPage
      view={view}
      tab={tab}
      onTab={setTab}
      onOpen={(name) => {
        const a = view?.installed?.find((x) => x.name === name);
        setValues(a ? defaultsOf(a) : {});
        setFailure(null);
        setOpen(name);
      }}
      onLink={onLink}
    />
  );
}
