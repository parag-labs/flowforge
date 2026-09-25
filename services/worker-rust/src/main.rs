//! FlowForge worker — Rust. Polls the engine, executes a task, reports the result.
//! Uses only std (a tiny TCP+HTTP client) so the worker builds with no external crates
//! and no C toolchain — it only ever talks plain HTTP to the local engine.

use std::io::{Read, Write};
use std::net::TcpStream;
use std::{thread, time::Duration};

use flowforge_worker::idempotency_key;

fn env_or(key: &str, default: &str) -> String {
    std::env::var(key).unwrap_or_else(|_| default.to_string())
}

fn execute(task_id: &str) -> String {
    format!("done:{task_id}")
}

// Parse "host:port" out of an http://host:port base URL.
fn host_port(base: &str) -> String {
    base.trim_start_matches("http://")
        .trim_end_matches('/')
        .to_string()
}

// Minimal HTTP POST returning (status, body). Good enough for a localhost JSON API.
fn post(addr: &str, path: &str, json: &str) -> Option<(u16, String)> {
    let mut stream = TcpStream::connect(addr).ok()?;
    let host = addr.split(':').next().unwrap_or("localhost");
    let req = format!(
        "POST {path} HTTP/1.1\r\nHost: {host}\r\nContent-Type: application/json\r\n\
         Content-Length: {}\r\nConnection: close\r\n\r\n{json}",
        json.len()
    );
    stream.write_all(req.as_bytes()).ok()?;
    let mut raw = String::new();
    stream.read_to_string(&mut raw).ok()?;
    let status = raw
        .split_whitespace()
        .nth(1)
        .and_then(|s| s.parse::<u16>().ok())
        .unwrap_or(0);
    let body = raw
        .split_once("\r\n\r\n")
        .map(|x| x.1)
        .unwrap_or("")
        .to_string();
    Some((status, body))
}

fn field(body: &str, name: &str) -> String {
    let needle = format!("\"{name}\"");
    let Some(i) = body.find(&needle) else {
        return String::new();
    };
    let rest = &body[i + needle.len()..];
    let Some(colon) = rest.find(':') else {
        return String::new();
    };
    let after = rest[colon + 1..].trim_start();
    if let Some(stripped) = after.strip_prefix('"') {
        stripped.split('"').next().unwrap_or("").to_string()
    } else {
        after
            .split([',', '}'])
            .next()
            .unwrap_or("")
            .trim()
            .to_string()
    }
}

fn main() {
    let engine = env_or("FLOWFORGE_ENGINE", "http://localhost:8080");
    let worker = env_or("FLOWFORGE_WORKER", "worker-rust");
    let addr = host_port(&engine);
    println!("{worker} polling {engine}");

    loop {
        let claim = post(&addr, "/claim", &format!("{{\"worker\":\"{worker}\"}}"));
        let Some((200, body)) = claim else {
            thread::sleep(Duration::from_millis(500));
            continue;
        };
        let workflow_id = field(&body, "workflow_id");
        let task_id = field(&body, "task_id");
        if task_id.is_empty() {
            thread::sleep(Duration::from_millis(500));
            continue;
        }
        let attempt: u32 = field(&body, "attempt").parse().unwrap_or(1);
        let mut key = field(&body, "idempotency_key");
        if key.is_empty() {
            key = idempotency_key(&workflow_id, &task_id, attempt);
        }

        let result = execute(&task_id);
        let payload = format!(
            "{{\"workflow_id\":\"{workflow_id}\",\"task_id\":\"{task_id}\",\
             \"idempotency_key\":\"{key}\",\"result\":\"{result}\"}}"
        );
        let _ = post(&addr, "/complete", &payload);
        println!("task {workflow_id}/{task_id} attempt {attempt} -> {result}");
    }
}
