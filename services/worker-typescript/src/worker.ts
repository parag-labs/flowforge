// FlowForge worker — TypeScript. Polls the engine, executes a task, reports the result.
import { idempotencyKey } from "./contract";

const engine = process.env.FLOWFORGE_ENGINE ?? "http://localhost:8080";
const worker = process.env.FLOWFORGE_WORKER ?? "worker-typescript";

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

function execute(taskId: string): string {
  return `done:${taskId}`;
}

async function post(path: string, body: unknown): Promise<Response | null> {
  try {
    return await fetch(`${engine}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
  } catch {
    return null;
  }
}

async function run(): Promise<void> {
  console.log(`${worker} polling ${engine}`);
  for (;;) {
    const resp = await post("/claim", { worker });
    if (!resp || resp.status !== 200) {
      await sleep(500);
      continue;
    }
    const task = await resp.json();
    if (!task || !task.task_id) {
      await sleep(500);
      continue;
    }
    const key: string =
      task.idempotency_key ?? idempotencyKey(task.workflow_id, task.task_id, task.attempt);
    const result = execute(task.task_id);
    await post("/complete", {
      workflow_id: task.workflow_id,
      task_id: task.task_id,
      idempotency_key: key,
      result,
    });
    console.log(`task ${task.workflow_id}/${task.task_id} attempt ${task.attempt} -> ${result}`);
  }
}

run();
