import { describe, expect, it } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { backoffDelayMs, idempotencyKey } from "./contract";

// The TypeScript worker must derive the same contract values as the engine.
const here = dirname(fileURLToPath(import.meta.url));
const vectors = JSON.parse(
  readFileSync(resolve(here, "../../../contracts/vectors.json"), "utf-8"),
);

describe("contract conformance", () => {
  it("idempotency keys match", () => {
    for (const c of vectors.idempotency) {
      expect(idempotencyKey(c.workflow_id, c.task_id, c.attempt)).toBe(c.key);
    }
  });

  it("backoff matches", () => {
    for (const [attempt, expected] of Object.entries(vectors.backoff_ms)) {
      expect(backoffDelayMs(Number(attempt))).toBe(expected);
    }
  });
});
