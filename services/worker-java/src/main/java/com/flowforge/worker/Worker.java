package com.flowforge.worker;

import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * FlowForge worker — Java. Polls the engine, executes a task, reports the result.
 * Uses only the JDK HTTP client and small regex extraction so the worker has no
 * runtime dependencies; JSON parsing for tests uses org.json (test scope only).
 */
public final class Worker {

    private static final Pattern FIELD = Pattern.compile("\"%s\"\\s*:\\s*\"([^\"]*)\"");
    private static final Pattern ATTEMPT = Pattern.compile("\"attempt\"\\s*:\\s*(\\d+)");

    public static void main(String[] args) throws Exception {
        String engine = envOr("FLOWFORGE_ENGINE", "http://localhost:8080");
        String worker = envOr("FLOWFORGE_WORKER", "worker-java");
        HttpClient http = HttpClient.newHttpClient();
        System.out.println(worker + " polling " + engine);

        while (true) {
            HttpResponse<String> claim = post(http, engine + "/claim", "{\"worker\":\"" + worker + "\"}");
            if (claim.statusCode() != 200 || claim.body().isBlank()) {
                Thread.sleep(500);
                continue;
            }
            String body = claim.body();
            String workflowId = field(body, "workflow_id");
            String taskId = field(body, "task_id");
            String key = field(body, "idempotency_key");
            Matcher am = ATTEMPT.matcher(body);
            int attempt = am.find() ? Integer.parseInt(am.group(1)) : 1;
            if (key.isEmpty()) {
                key = Contract.idempotencyKey(workflowId, taskId, attempt);
            }

            String result = execute(taskId);
            String payload = String.format(
                    "{\"workflow_id\":\"%s\",\"task_id\":\"%s\",\"idempotency_key\":\"%s\",\"result\":\"%s\"}",
                    workflowId, taskId, key, result);
            post(http, engine + "/complete", payload);
            System.out.printf("task %s/%s attempt %d -> %s%n", workflowId, taskId, attempt, result);
        }
    }

    private static String execute(String taskId) {
        return "done:" + taskId;
    }

    private static HttpResponse<String> post(HttpClient http, String url, String json) throws Exception {
        HttpRequest req = HttpRequest.newBuilder(URI.create(url))
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(json))
                .build();
        return http.send(req, HttpResponse.BodyHandlers.ofString());
    }

    private static String field(String body, String name) {
        Matcher m = Pattern.compile(String.format(FIELD.pattern(), name)).matcher(body);
        return m.find() ? m.group(1) : "";
    }

    private static String envOr(String key, String def) {
        String v = System.getenv(key);
        return (v == null || v.isEmpty()) ? def : v;
    }

    private Worker() {
    }
}
