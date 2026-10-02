// Thin client for the FlowForge engine API. In dev, Vite proxies /api to the engine.
export type TaskState = "PENDING" | "LEASED" | "COMPLETED" | "DEAD_LETTER";

export interface Task {
  WorkflowID: string;
  TaskID: string;
  State: TaskState;
  Attempt: number;
  MaxRetries: number;
}

const BASE = "/api";

export async function fetchTasks(): Promise<Task[]> {
  const resp = await fetch(`${BASE}/tasks`);
  if (!resp.ok) throw new Error(`tasks: ${resp.status}`);
  return (await resp.json()) ?? [];
}

export async function submitWorkflow(
  workflowId: string,
  tasks: string[],
  maxRetries = 3,
): Promise<void> {
  await fetch(`${BASE}/workflows`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ workflow_id: workflowId, tasks, max_retries: maxRetries }),
  });
}

export async function health(): Promise<boolean> {
  try {
    const resp = await fetch(`${BASE}/healthz`);
    return resp.ok;
  } catch {
    return false;
  }
}
