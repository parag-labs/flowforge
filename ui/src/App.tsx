import { useCallback, useEffect, useMemo, useState } from "react";
import { fetchTasks, health, submitWorkflow, type Task, type TaskState } from "./api";

const STATE_META: Record<TaskState, { label: string; cls: string }> = {
  PENDING: { label: "Pending", cls: "s-pending" },
  LEASED: { label: "Running", cls: "s-leased" },
  COMPLETED: { label: "Completed", cls: "s-done" },
  DEAD_LETTER: { label: "Dead-letter", cls: "s-dead" },
};

const DEMO_TASKS = ["validate", "charge-card", "reserve-stock", "ship", "notify"];

export function App() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [online, setOnline] = useState(false);
  const [wfName, setWfName] = useState("wf-demo");

  const refresh = useCallback(async () => {
    setOnline(await health());
    try {
      setTasks(await fetchTasks());
    } catch {
      setTasks([]);
    }
  }, []);

  useEffect(() => {
    refresh();
    const t = setInterval(refresh, 1000);
    return () => clearInterval(t);
  }, [refresh]);

  const byWorkflow = useMemo(() => {
    const m = new Map<string, Task[]>();
    for (const t of tasks) {
      const arr = m.get(t.WorkflowID) ?? [];
      arr.push(t);
      m.set(t.WorkflowID, arr);
    }
    for (const arr of m.values()) arr.sort((a, b) => a.TaskID.localeCompare(b.TaskID));
    return [...m.entries()];
  }, [tasks]);

  const totals = useMemo(() => {
    const c: Record<TaskState, number> = { PENDING: 0, LEASED: 0, COMPLETED: 0, DEAD_LETTER: 0 };
    for (const t of tasks) c[t.State]++;
    return c;
  }, [tasks]);

  async function launch() {
    const id = `${wfName}-${Math.random().toString(36).slice(2, 6)}`;
    await submitWorkflow(id, DEMO_TASKS, 3);
    refresh();
  }

  return (
    <div className="app">
      <header>
        <div className="brand">
          <span className="pulse" /> FlowForge
          <span className={online ? "badge on" : "badge off"}>
            {online ? "engine online" : "engine offline"}
          </span>
        </div>
        <h1>Operator console</h1>
        <p className="lede">
          Live view of every workflow the engine is running — task states, retry
          attempts, and the dead-letter queue. Polls the engine once a second.
        </p>
      </header>

      <div className="stats">
        {(Object.keys(STATE_META) as TaskState[]).map((s) => (
          <div className={`stat ${STATE_META[s].cls}`} key={s}>
            <div className="stat-num">{totals[s]}</div>
            <div className="stat-label">{STATE_META[s].label}</div>
          </div>
        ))}
      </div>

      <div className="controls">
        <input value={wfName} onChange={(e) => setWfName(e.target.value)} aria-label="workflow name" />
        <button onClick={launch} disabled={!online}>
          ▶ Launch demo workflow
        </button>
        <span className="hint">
          Start the engine and one or more workers, then launch — watch tasks flow to
          completed. Kill a worker mid-run to see leases recover.
        </span>
      </div>

      {byWorkflow.length === 0 && (
        <div className="empty">No workflows yet. Launch one above.</div>
      )}

      <div className="workflows">
        {byWorkflow.map(([wf, ts]) => {
          const done = ts.filter((t) => t.State === "COMPLETED").length;
          return (
            <div className="wf" key={wf}>
              <div className="wf-head">
                <span className="wf-id">{wf}</span>
                <span className="wf-progress">
                  {done}/{ts.length} complete
                </span>
              </div>
              <div className="timeline">
                {ts.map((t) => (
                  <div className={`node ${STATE_META[t.State].cls}`} key={t.TaskID} title={t.TaskID}>
                    <span className="node-name">{t.TaskID}</span>
                    <span className="node-state">{STATE_META[t.State].label}</span>
                    {t.Attempt > 1 && <span className="node-attempt">try {t.Attempt}</span>}
                  </div>
                ))}
              </div>
            </div>
          );
        })}
      </div>

      <footer>
        <span>Engine + interchangeable workers in Go · Python · Rust · C# · Java · TypeScript.</span>
        <a href="https://github.com/parag-labs/flowforge">Source →</a>
      </footer>
    </div>
  );
}
