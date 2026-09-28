package com.flowforge.worker;

import static org.junit.jupiter.api.Assertions.assertEquals;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import org.json.JSONObject;
import org.junit.jupiter.api.Test;

/** The Java worker must derive the same contract values as the engine. */
class ConformanceTest {

    private static JSONObject contract() throws IOException {
        Path dir = Paths.get("").toAbsolutePath();
        while (dir != null && !Files.exists(dir.resolve("contracts/vectors.json"))) {
            dir = dir.getParent();
        }
        if (dir == null) {
            throw new IOException("could not locate contracts/vectors.json");
        }
        return new JSONObject(Files.readString(dir.resolve("contracts/vectors.json")));
    }

    @Test
    void idempotencyKeysMatchContract() throws IOException {
        var arr = contract().getJSONArray("idempotency");
        for (int i = 0; i < arr.length(); i++) {
            JSONObject c = arr.getJSONObject(i);
            String got = Contract.idempotencyKey(
                    c.getString("workflow_id"), c.getString("task_id"), c.getInt("attempt"));
            assertEquals(c.getString("key"), got);
        }
    }

    @Test
    void backoffMatchesContract() throws IOException {
        JSONObject b = contract().getJSONObject("backoff_ms");
        for (String attempt : b.keySet()) {
            assertEquals(b.getInt(attempt), Contract.backoffDelayMs(Integer.parseInt(attempt)));
        }
    }
}
