import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import type {
  ExportResult,
  ExportSummary,
  ImportPreview,
  ImportResult,
  Node,
  Problem,
  SSHConfigView,
} from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { ExportDialog } from "../ExportDialog/ExportDialog";
import { ImportWindow } from "../ImportWindow/ImportWindow";
import { SSHConfigDialog } from "../SSHConfigDialog/SSHConfigDialog";
import styles from "./DataFlow.module.css";

/** The Go service of the Data tab (app.Data). */
export interface DataApi {
  ExportSummary(nodeId: string): Promise<ExportSummary>;
  Export(nodeId: string, history: boolean): Promise<ExportResult>;
  ChooseImport(kind: string): Promise<ImportPreview>;
  PreviewImport(intoId: string): Promise<ImportPreview>;
  ApplyImport(trustHostIds: string[]): Promise<ImportResult>;
  CancelImport(): Promise<void>;
  SSHConfig(): Promise<SSHConfigView>;
  ImportSSHConfig(aliases: string[], intoId: string): Promise<ImportResult>;
}

/** What the user asked for: an export (of everything, "", or a node), or an import, of a file or
 of what an add-on wrote (its preview made already). */
export type DataTask =
  | { kind: "export"; nodeId: string }
  | { kind: "ssh" }
  | { kind: "import"; source: "vigie" | "csv" }
  | { kind: "preview"; preview: ImportPreview };

/** What the task did, for the row of the Data tab. */
export interface DataNote {
  row: "export" | "import" | "compact";
  text: string;
}

export interface DataFlowProps {
  api: DataApi;
  task: DataTask;
  nodes: Node[];
  /** Done or cancelled, with what it did when it did something; changed: the site manager has new nodes. */
  onClose: (note?: DataNote, changed?: boolean) => void;
  /** The preview of an import is shown: it takes the whole window, the rest of it goes away. */
  onWindow?: () => void;
}

/** An export or an import, from its first dialog to its end, over the window. */
export function DataFlow({ api, task, nodes, onClose, onWindow }: DataFlowProps) {
  return (
    <div className={task.kind === "import" || task.kind === "preview" ? styles.window : styles.layer}>
      {task.kind === "export" && <Export api={api} initial={task.nodeId} nodes={nodes} onClose={onClose} />}
      {task.kind === "ssh" && <FromSSHConfig api={api} nodes={nodes} onClose={onClose} />}
      {task.kind === "import" && <Import api={api} source={task.source} nodes={nodes} onClose={onClose} onWindow={onWindow} />}
      {task.kind === "preview" && <Import api={api} ready={task.preview} nodes={nodes} onClose={onClose} onWindow={onWindow} />}
    </div>
  );
}

function Export({ api, initial, nodes, onClose }: { api: DataApi; initial: string; nodes: Node[]; onClose: DataFlowProps["onClose"] }) {
  const { t } = useTranslation();
  const [nodeId, setNodeId] = useState(initial);
  const [history, setHistory] = useState(false);
  const [summary, setSummary] = useState<ExportSummary | null>(null);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<Problem | null>(null);

  useEffect(() => {
    let current = true;
    void api.ExportSummary(nodeId).then((s) => {
      if (!current) return;
      if (s.problem) setProblem(s.problem);
      else setSummary(s);
    });
    return () => {
      current = false;
    };
  }, [api, nodeId]);

  const run = async () => {
    setBusy(true);
    setProblem(null);
    const result = await api.Export(nodeId, history);
    setBusy(false);
    if (result.problem) return setProblem(result.problem);
    // The file dialog closed without a file: the export dialog stays.
    if (result.file) onClose({ row: "export", text: t("data.exported", { file: result.file }) });
  };

  return (
    <ExportDialog
      nodes={nodes}
      nodeId={nodeId}
      onNode={(id) => {
        setSummary(null);
        setNodeId(id);
      }}
      history={history}
      onHistory={setHistory}
      summary={summary}
      onExport={() => void run()}
      onClose={() => onClose()}
      busy={busy}
      problem={problem}
    />
  );
}

function FromSSHConfig({ api, nodes, onClose }: { api: DataApi; nodes: Node[]; onClose: DataFlowProps["onClose"] }) {
  const { t } = useTranslation();
  const [config, setConfig] = useState<SSHConfigView | null>(null);
  const [chosen, setChosen] = useState(new Set<string>());
  const [into, setInto] = useState("");
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<Problem | null>(null);

  useEffect(() => {
    void api.SSHConfig().then((c) => {
      setConfig(c);
      setProblem(c.problem);
    });
  }, [api]);

  const run = async () => {
    setBusy(true);
    setProblem(null);
    const result = await api.ImportSSHConfig([...chosen], into);
    setBusy(false);
    if (result.problem) return setProblem(result.problem);
    onClose({ row: "import", text: t("data.imported", { count: result.added }) }, result.added > 0);
  };

  return (
    <SSHConfigDialog
      config={config}
      nodes={nodes}
      into={into}
      onInto={setInto}
      chosen={chosen}
      onToggle={(alias, on) => {
        const next = new Set(chosen);
        if (on) next.add(alias);
        else next.delete(alias);
        setChosen(next);
      }}
      onImport={() => void run()}
      onClose={() => onClose()}
      busy={busy}
      problem={problem}
    />
  );
}

function Import({
  api,
  source,
  ready,
  nodes,
  onClose,
  onWindow,
}: {
  api: DataApi;
  /** A file to choose, or a preview made already. */
  source?: "vigie" | "csv";
  ready?: ImportPreview;
  nodes: Node[];
  onClose: DataFlowProps["onClose"];
  onWindow?: () => void;
}) {
  const { t } = useTranslation();
  const [preview, setPreview] = useState<ImportPreview | null>(null);
  const [keep, setKeep] = useState(new Set<string>());
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<Problem | null>(null);
  // Asked once, even when React runs the effect twice in development.
  const asked = useRef(false);

  useEffect(() => {
    if (asked.current) return;
    asked.current = true;
    void (ready ? Promise.resolve(ready) : api.ChooseImport(source ?? "vigie")).then((p) => {
      if (p.cancelled) return onClose();
      // A file Vigie cannot read: said under the row of the Data tab.
      if (p.problem) return onClose({ row: "import", text: t(p.problem.code, { message: p.problem.detail }) });
      setPreview(p);
      onWindow?.();
    });
  }, [api, source, ready, onClose, onWindow, t]);

  const close = () => {
    void api.CancelImport();
    onClose();
  };

  const into = async (folderId: string) => {
    setBusy(true);
    const p = await api.PreviewImport(folderId);
    setBusy(false);
    if (p.problem) return setProblem(p.problem);
    setProblem(null);
    // The hosts added may have new IDs: the keys are checked again.
    setKeep(new Set());
    setPreview(p);
  };

  const run = async () => {
    setBusy(true);
    setProblem(null);
    const result = await api.ApplyImport([...keep]);
    setBusy(false);
    if (result.problem) return setProblem(result.problem);
    onClose({ row: "import", text: t("data.imported", { count: result.added }) }, true);
  };

  // While the file dialog is open, the window stays as it was.
  if (!preview) return null;
  return (
    <ImportWindow
      preview={preview}
      nodes={nodes}
      onInto={(id) => void into(id)}
      keep={keep}
      onKeep={(id, on) => {
        const next = new Set(keep);
        if (on) next.add(id);
        else next.delete(id);
        setKeep(next);
      }}
      onConfirm={() => void run()}
      onClose={close}
      busy={busy}
      problem={problem}
    />
  );
}
