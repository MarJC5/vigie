import { useEffect, useRef, useState } from "react";

import type {
  OperationView,
  Problem,
  SelectionView,
  SitePreviewView,
  UpdatesState,
} from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { plannedItems, selectionOf, selectionOfItems } from "../../lib/updates";
import { UpdateWindow } from "../UpdateWindow/UpdateWindow";

/** The Go service of the updates (app.Updates). */
export interface UpdatesApi {
  Preview(nodeId: string): Promise<SitePreviewView[] | null>;
  Update(siteId: string, sel: SelectionView): Promise<Problem | null>;
  UpdateAll(nodeId: string): Promise<Problem | null>;
  Rollback(siteId: string, operationId: string): Promise<Problem | null>;
  CancelAll(): Promise<void>;
  History(siteId: string): Promise<OperationView[] | null>;
}

export interface UpdateFlowProps {
  api: UpdatesApi;
  site: { id: string; name: string };
  /** The keys of the items chosen in the Updates tab. */
  keys: Set<string>;
  /** The titles of the plugins and themes, by key. */
  names: Map<string, string>;
  /** The plugins wordpress.org knows, for their changelogs. */
  known?: Set<string>;
  /** Opens a changelog in the browser. */
  onOpen?: (href: string) => void;
  state: UpdatesState;
  /** Opens on the run of the site already going, instead of a new preview. */
  follow?: boolean;
  onClose: () => void;
}

/** A run the window follows. */
interface Following {
  kind: "update" | "rollback";
  /** The operations of the site before the run: its own is the first not among them. Null while
   they are read. */
  before: Set<string> | null;
  /** Vigie's count of operations ended when the run started. */
  completed: number;
}

const notStarted: Problem = { code: "update.run.notStarted", detail: "" };

/** The update of one site, from its preview to how it ended, following Vigie's state. */
export function UpdateFlow({ api, site, keys, names, known, onOpen, state, follow = false, onClose }: UpdateFlowProps) {
  const running = (state.running ?? []).find((op) => op.siteId === site.id);
  const busy = (state.sites ?? []).includes(site.id);

  const [preview, setPreview] = useState<SitePreviewView | null>(null);
  const [problem, setProblem] = useState<Problem | null>(null);
  const [following, setFollowing] = useState<Following | null>(() =>
    follow ? { kind: running?.kind === "rollback" ? "rollback" : "update", before: null, completed: state.completed } : null,
  );
  const [ended, setEnded] = useState<OperationView | null>(null);
  const [stopping, setStopping] = useState(false);
  const closed = useRef(false);
  // The site was seen in the run: when it leaves the runs, the run is over.
  const seen = useRef(follow && busy);
  // The operation running when the window opened on it.
  const runningAtOpen = useRef(follow ? running?.id : undefined);
  const ending = useRef(false);

  useEffect(() => {
    closed.current = false;
    if (follow) {
      void api.History(site.id).then((ops) => {
        if (closed.current) return;
        const before = new Set((ops ?? []).map((op) => op.id).filter((id) => id !== runningAtOpen.current));
        setFollowing((f) => f && { ...f, before });
      });
    } else {
      api
        .Preview(site.id)
        .then((list) => {
          if (!closed.current) setPreview(list?.[0] ?? null);
        })
        .catch((err: unknown) => {
          if (!closed.current) setProblem({ ...notStarted, detail: String(err) });
        });
    }
    return () => {
      closed.current = true;
    };
  }, [api, site.id, follow]);

  // The run ends when the site leaves the runs; its operation is the first new one in the history.
  useEffect(() => {
    if (!following?.before || ending.current) return;
    if (busy || running) {
      seen.current = true;
      return;
    }
    if (!seen.current && state.completed === following.completed) return;
    ending.current = true;
    const before = following.before;
    void api.History(site.id).then((ops) => {
      ending.current = false;
      seen.current = false;
      if (closed.current) return;
      setFollowing(null);
      setStopping(false);
      const op = (ops ?? []).find((o) => !before.has(o.id));
      if (op) setEnded(op);
      else setProblem(notStarted);
    });
  }, [following, busy, running, state.completed, api, site.id]);

  const planned = preview ? plannedItems(preview, selectionOf(keys)) : [];

  const start = async (kind: Following["kind"], call: () => Promise<Problem | null>) => {
    setProblem(null);
    const before = new Set(((await api.History(site.id)) ?? []).map((op) => op.id));
    const completed = state.completed;
    const refused = await call();
    if (refused) return setProblem(refused);
    seen.current = false;
    setEnded(null);
    setFollowing({ kind, before, completed });
  };

  return (
    <UpdateWindow
      name={site.name}
      preview={preview}
      planned={planned}
      names={names}
      known={known}
      onOpen={onOpen}
      operation={following ? (running ?? null) : ended}
      starting={!!following && !running}
      stopping={stopping}
      problem={problem}
      rollingBack={following?.kind === "rollback"}
      onClose={onClose}
      onConfirm={() => void start("update", () => api.Update(site.id, selectionOfItems(planned)))}
      onStop={() => {
        setStopping(true);
        void api.CancelAll();
      }}
      onRollback={(opId) => void start("rollback", () => api.Rollback(site.id, opId))}
    />
  );
}
