package com.flowforge.worker;

import java.nio.charset.StandardCharsets;

/**
 * FlowForge worker contract — Java. Idempotency key and backoff must match the engine
 * and every other language worker (see contracts/vectors.json).
 */
public final class Contract {

    private static final long FNV64_OFFSET = 0xCBF29CE484222325L;
    private static final long FNV64_PRIME = 0x100000001B3L;
    private static final int BASE_MS = 200;
    private static final int MAX_MS = 30000;

    private Contract() {
    }

    /** FNV-1a 64-bit; bits held in a signed long. */
    public static long fnv1a64(String data) {
        long h = FNV64_OFFSET;
        for (byte b : data.getBytes(StandardCharsets.UTF_8)) {
            h ^= (b & 0xFF);
            h *= FNV64_PRIME; // wraps mod 2^64
        }
        return h;
    }

    public static String idempotencyKey(String workflowId, String taskId, int attempt) {
        return String.format("%016x", fnv1a64(workflowId + "|" + taskId + "|" + attempt));
    }

    public static int backoffDelayMs(int attempt) {
        int d = BASE_MS;
        for (int i = 1; i < attempt; i++) {
            d = Math.min(d * 2, MAX_MS);
        }
        return Math.min(d, MAX_MS);
    }
}
