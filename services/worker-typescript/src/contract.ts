// FlowForge worker contract — TypeScript. Idempotency key and backoff must match the
// engine and every other language worker (see contracts/vectors.json).

const FNV64_OFFSET = 0xcbf29ce484222325n;
const FNV64_PRIME = 0x100000001b3n;
const MASK64 = 0xffffffffffffffffn;

const BASE_MS = 200;
const MAX_MS = 30000;

export function fnv1a64(s: string): bigint {
  let h = FNV64_OFFSET;
  for (const b of new TextEncoder().encode(s)) {
    h ^= BigInt(b);
    h = (h * FNV64_PRIME) & MASK64;
  }
  return h;
}

export function idempotencyKey(workflowId: string, taskId: string, attempt: number): string {
  return fnv1a64(`${workflowId}|${taskId}|${attempt}`).toString(16).padStart(16, "0");
}

export function backoffDelayMs(attempt: number): number {
  let d = BASE_MS;
  for (let i = 1; i < attempt; i++) d = Math.min(d * 2, MAX_MS);
  return Math.min(d, MAX_MS);
}
