using System.Text.Json;
using Xunit;

namespace FlowForge.Worker.Tests;

/// <summary>The C# worker must derive the same contract values as the engine.</summary>
public class ConformanceTests
{
    private static readonly JsonElement Vectors = Load();

    private static JsonElement Load()
    {
        var dir = AppContext.BaseDirectory;
        while (dir is not null && !File.Exists(Path.Combine(dir, "contracts", "vectors.json")))
        {
            dir = Directory.GetParent(dir)?.FullName;
        }

        if (dir is null)
        {
            throw new FileNotFoundException("could not locate contracts/vectors.json");
        }

        return JsonDocument.Parse(File.ReadAllText(Path.Combine(dir, "contracts", "vectors.json")))
            .RootElement.Clone();
    }

    [Fact]
    public void IdempotencyKeysMatchContract()
    {
        foreach (var c in Vectors.GetProperty("idempotency").EnumerateArray())
        {
            var got = Contract.IdempotencyKey(
                c.GetProperty("workflow_id").GetString()!,
                c.GetProperty("task_id").GetString()!,
                c.GetProperty("attempt").GetInt32());
            Assert.Equal(c.GetProperty("key").GetString(), got);
        }
    }

    [Fact]
    public void BackoffMatchesContract()
    {
        foreach (var p in Vectors.GetProperty("backoff_ms").EnumerateObject())
        {
            Assert.Equal(p.Value.GetInt32(), Contract.BackoffDelayMs(int.Parse(p.Name)));
        }
    }
}
