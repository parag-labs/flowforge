namespace FlowForge.Worker;

/// <summary>
/// FlowForge worker contract — C#. Idempotency key and backoff must match the engine
/// and every other language worker (see contracts/vectors.json).
/// </summary>
public static class Contract
{
    private const ulong Fnv64Offset = 0xCBF29CE484222325;
    private const ulong Fnv64Prime = 0x100000001B3;
    private const int BaseMs = 200;
    private const int MaxMs = 30000;

    public static ulong Fnv1a64(string data)
    {
        ulong h = Fnv64Offset;
        foreach (var b in System.Text.Encoding.UTF8.GetBytes(data))
        {
            h ^= b;
            h *= Fnv64Prime;
        }

        return h;
    }

    public static string IdempotencyKey(string workflowId, string taskId, int attempt) =>
        Fnv1a64($"{workflowId}|{taskId}|{attempt}").ToString("x16");

    public static int BackoffDelayMs(int attempt)
    {
        var d = BaseMs;
        for (var i = 1; i < attempt; i++)
        {
            d = Math.Min(d * 2, MaxMs);
        }

        return Math.Min(d, MaxMs);
    }
}
