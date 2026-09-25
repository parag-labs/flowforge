//! The Rust worker must derive the same contract values as the engine.

use flowforge_worker::{backoff_delay_ms, idempotency_key};
use std::fs;
use std::path::Path;

fn contract() -> serde_json::Value {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("..")
        .join("..")
        .join("contracts")
        .join("vectors.json");
    serde_json::from_str(&fs::read_to_string(path).expect("read vectors")).expect("parse")
}

#[test]
fn idempotency_keys_match_contract() {
    let c = contract();
    for case in c["idempotency"].as_array().unwrap() {
        let got = idempotency_key(
            case["workflow_id"].as_str().unwrap(),
            case["task_id"].as_str().unwrap(),
            case["attempt"].as_u64().unwrap() as u32,
        );
        assert_eq!(got, case["key"].as_str().unwrap());
    }
}

#[test]
fn backoff_matches_contract() {
    let c = contract();
    for (attempt, expected) in c["backoff_ms"].as_object().unwrap() {
        let got = backoff_delay_ms(attempt.parse().unwrap());
        assert_eq!(got, expected.as_u64().unwrap());
    }
}
