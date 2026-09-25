//! FlowForge worker contract — Rust. The idempotency key and backoff must match the
//! engine and every other language worker (see contracts/vectors.json).

const FNV64_OFFSET: u64 = 0xCBF2_9CE4_8422_2325;
const FNV64_PRIME: u64 = 0x0000_0100_0000_01B3;

const BASE_MS: u64 = 200;
const MAX_MS: u64 = 30000;

pub fn fnv1a_64(s: &str) -> u64 {
    let mut h = FNV64_OFFSET;
    for b in s.as_bytes() {
        h ^= *b as u64;
        h = h.wrapping_mul(FNV64_PRIME);
    }
    h
}

pub fn idempotency_key(workflow_id: &str, task_id: &str, attempt: u32) -> String {
    format!(
        "{:016x}",
        fnv1a_64(&format!("{workflow_id}|{task_id}|{attempt}"))
    )
}

pub fn backoff_delay_ms(attempt: u32) -> u64 {
    let mut d = BASE_MS;
    for _ in 1..attempt {
        d = (d * 2).min(MAX_MS);
    }
    d.min(MAX_MS)
}
