"""The Python worker must derive the same contract values as the engine and every
other worker."""

from __future__ import annotations

import json
from pathlib import Path

from worker import backoff_delay_ms, idempotency_key

VECTORS = json.loads(
    (Path(__file__).resolve().parents[2] / "contracts" / "vectors.json").read_text(encoding="utf-8")
)


def test_idempotency_keys_match_contract():
    for case in VECTORS["idempotency"]:
        assert idempotency_key(case["workflow_id"], case["task_id"], case["attempt"]) == case["key"]


def test_backoff_matches_contract():
    for attempt_str, expected in VECTORS["backoff_ms"].items():
        assert backoff_delay_ms(int(attempt_str)) == expected
