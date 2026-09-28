using System.Net.Http.Json;
using System.Text.Json;

namespace FlowForge.Worker;

/// <summary>FlowForge worker — C#. Polls the engine, executes a task, reports the result.</summary>
public static class Program
{
    public static async Task Main()
    {
        var engine = Environment.GetEnvironmentVariable("FLOWFORGE_ENGINE") ?? "http://localhost:8080";
        var worker = Environment.GetEnvironmentVariable("FLOWFORGE_WORKER") ?? "worker-csharp";
        using var http = new HttpClient();
        Console.WriteLine($"{worker} polling {engine}");

        while (true)
        {
            var claim = await http.PostAsJsonAsync($"{engine}/claim", new { worker });
            if (claim.StatusCode != System.Net.HttpStatusCode.OK)
            {
                await Task.Delay(500);
                continue;
            }

            var task = await claim.Content.ReadFromJsonAsync<JsonElement>();
            if (task.ValueKind != JsonValueKind.Object)
            {
                await Task.Delay(500);
                continue;
            }

            var workflowId = task.GetProperty("workflow_id").GetString()!;
            var taskId = task.GetProperty("task_id").GetString()!;
            var attempt = task.GetProperty("attempt").GetInt32();
            var key = task.TryGetProperty("idempotency_key", out var k)
                ? k.GetString()!
                : Contract.IdempotencyKey(workflowId, taskId, attempt);

            var result = Execute(taskId);
            await http.PostAsJsonAsync($"{engine}/complete", new
            {
                workflow_id = workflowId,
                task_id = taskId,
                idempotency_key = key,
                result,
            });
            Console.WriteLine($"task {workflowId}/{taskId} attempt {attempt} -> {result}");
        }
    }

    private static string Execute(string taskId) => $"done:{taskId}";
}
