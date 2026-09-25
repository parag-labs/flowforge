"""FlowForge worker — Python.

Polls the engine for a task, runs the effect, reports the result. The worker also
implements the shared contract (idempotency key + backoff) itself, so it agrees with
the engine and every other language worker. That's what makes the workers
interchangeable: a Python worker and a Go worker place the same effect under the same
key, so a duplicate never applies twice.
"""

from __future__ import annotations

import os
import time
import urllib.request
import json

FNV64_OFFSET = 0xCBF29CE484222325
FNV64_PRIME = 0x100000001B3
MASK64 = 0xFFFFFFFFFFFFFFFF

BASE_MS = 200
MAX_MS = 30000


def fnv1a_64(data: str) -> int:
    h = FNV64_OFFSET
    for byte in data.encode("utf-8"):
        h ^= byte
        h = (h * FNV64_PRIME) & MASK64
    return h


def idempotency_key(workflow_id: str, task_id: str, attempt: int) -> str:
    return f"{fnv1a_64(f'{workflow_id}|{task_id}|{attempt}'):016x}"


def backoff_delay_ms(attempt: int) -> int:
    return min(BASE_MS * (2 ** (attempt - 1)), MAX_MS)


def _post(url: str, payload: dict) -> tuple[int, dict]:
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(url, data=data, headers={"Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req) as resp:
            body = resp.read()
            return resp.status, (json.loads(body) if body else {})
    except urllib.error.HTTPError as e:  # noqa: PERF203
        return e.code, {}
    except urllib.error.URLError:
        return 0, {}


def execute(task_id: str) -> str:
    """Run the task's effect. Reference worker succeeds deterministically."""
    return f"done:{task_id}"


def run() -> None:
    engine = os.environ.get("FLOWFORGE_ENGINE", "http://localhost:8080")
    worker = os.environ.get("FLOWFORGE_WORKER", "worker-python")
    print(f"{worker} polling {engine}", flush=True)

    while True:
        status, task = _post(f"{engine}/claim", {"worker": worker})
        if status != 200 or not task:
            time.sleep(0.5)
            continue
        try:
            result = execute(task["task_id"])
        except Exception:  # noqa: BLE001
            _post(f"{engine}/fail", {"workflow_id": task["workflow_id"], "task_id": task["task_id"]})
            continue
        _post(
            f"{engine}/complete",
            {
                "workflow_id": task["workflow_id"],
                "task_id": task["task_id"],
                "idempotency_key": task["idempotency_key"],
                "result": result,
            },
        )
        print(f"task {task['workflow_id']}/{task['task_id']} attempt {task['attempt']} -> {result}", flush=True)


if __name__ == "__main__":
    run()
