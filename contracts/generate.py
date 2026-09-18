"""Generate contracts/vectors.json — the cross-language contract for FlowForge workers.

Pins two deterministic functions every worker must reproduce byte-for-byte:
  * idempotency_key(workflow_id, task_id, attempt)
  * backoff_delay_ms(attempt)

Run from the repo root: python contracts/generate.py
"""

from __future__ import annotations

import json
from pathlib import Path

FNV64_OFFSET = 0xCBF29CE484222325
FNV64_PRIME = 0x100000001B3
MASK64 = 0xFFFFFFFFFFFFFFFF

BASE_MS = 200
MAX_MS = 30000
MAX_ATTEMPTS = 5


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


CASES = [
    ("wf-0001", "validate", 1),
    ("wf-0001", "validate", 2),
    ("wf-0001", "charge-card", 1),
    ("wf-0001", "notify", 3),
    ("wf-abcdef", "join", 1),
    ("01J8ZK9Q", "approval", 4),
]


def build() -> dict:
    return {
        "_comment": "Cross-language contract for FlowForge workers. See contracts/README.md.",
        "params": {"base_ms": BASE_MS, "max_ms": MAX_MS, "max_attempts": MAX_ATTEMPTS},
        "idempotency": [
            {"workflow_id": w, "task_id": t, "attempt": a, "key": idempotency_key(w, t, a)}
            for (w, t, a) in CASES
        ],
        "backoff_ms": {str(a): backoff_delay_ms(a) for a in range(1, 9)},
    }


def main() -> None:
    out = Path(__file__).resolve().parent / "vectors.json"
    out.write_text(json.dumps(build(), indent=2) + "\n", encoding="utf-8")
    print(f"wrote {out}")


if __name__ == "__main__":
    main()
