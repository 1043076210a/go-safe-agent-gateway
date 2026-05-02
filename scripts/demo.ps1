param(
    [string]$BaseUrl = "http://127.0.0.1:8080",
    [string]$ApiKey = $env:GATEWAY_API_KEY,
    [string]$OutputDir = "demo-output"
)

$ErrorActionPreference = "Stop"

function New-Headers {
    $headers = @{ "Content-Type" = "application/json" }
    if ($ApiKey -and $ApiKey.Trim() -ne "") {
        $headers["Authorization"] = "Bearer $ApiKey"
    }
    return $headers
}

function Save-Json {
    param(
        [string]$Name,
        [object]$Value
    )
    $path = Join-Path $OutputDir "$Name.json"
    $Value | ConvertTo-Json -Depth 20 | Set-Content -Path $path -Encoding UTF8
    return $path
}

function Invoke-DemoPost {
    param(
        [string]$Path,
        [object]$Body
    )
    $json = $Body | ConvertTo-Json -Depth 20
    return Invoke-RestMethod -Uri "$BaseUrl$Path" -Method Post -Headers (New-Headers) -Body $json
}

New-Item -ItemType Directory -Force -Path $OutputDir | Out-Null
$transcript = Join-Path $OutputDir "demo-transcript.txt"
Start-Transcript -Path $transcript -Force | Out-Null

try {
    Write-Host "== Go Safe Agent Gateway Demo =="
    Write-Host "Base URL: $BaseUrl"

    Write-Host "`n[1/7] Health check"
    $health = Invoke-RestMethod -Uri "$BaseUrl/health"
    Save-Json "01-health" $health | Out-Null
    $health | ConvertTo-Json -Depth 10

    Write-Host "`n[2/7] List registered tools"
    $tools = Invoke-RestMethod -Uri "$BaseUrl/v1/tools" -Headers (New-Headers)
    Save-Json "02-tools" $tools | Out-Null
    $tools | ConvertTo-Json -Depth 10

    Write-Host "`n[3/7] Index demo document into Qdrant"
    $doc = @{
        title = "Gateway Demo Guide"
        source_type = "manual"
        source_path = "docs/demo-gateway-guide.md"
        content = "# Safe Tool Gateway`nThe gateway forces every LLM tool call through registry lookup, JSON Schema validation, policy checks, executor timeout, audit logging, and metrics.`n`n# RAG Search`nKnowledge base search chunks documents, embeds text, writes vectors to Qdrant, and returns citation metadata."
    }
    $index = Invoke-DemoPost "/v1/documents" $doc
    Save-Json "03-index-document" $index | Out-Null
    $index | ConvertTo-Json -Depth 10

    Write-Host "`n[4/7] Run Qdrant-backed knowledge-base search"
    $searchBody = @{
        user_id = "demo-user"
        tool_name = "search_knowledge_base"
        input = @{
            query = "policy schema timeout audit metrics"
            top_k = 3
        }
    }
    $search = Invoke-DemoPost "/v1/tools/execute" $searchBody
    Save-Json "04-search-knowledge-base" $search | Out-Null
    $search | ConvertTo-Json -Depth 20

    Write-Host "`n[5/7] Run calculator expression parser"
    $calcBody = @{
        user_id = "demo-user"
        tool_name = "calculator"
        input = @{
            expression = "2 + 3 * (4 - 1)"
        }
    }
    $calc = Invoke-DemoPost "/v1/tools/execute" $calcBody
    Save-Json "05-calculator" $calc | Out-Null
    $calc | ConvertTo-Json -Depth 10

    Write-Host "`n[6/7] Submit async query_logs task"
    $asyncBody = @{
        user_id = "demo-user"
        tool_name = "query_logs"
        async = $true
        input = @{
            kind = "tool_calls"
            limit = 10
        }
    }
    $task = Invoke-DemoPost "/v1/tools/execute" $asyncBody
    Save-Json "06-async-submit" $task | Out-Null
    Start-Sleep -Seconds 1
    $taskID = $task.data.task_id
    $taskStatus = Invoke-RestMethod -Uri "$BaseUrl/v1/async-tasks/$taskID" -Headers (New-Headers)
    Save-Json "07-async-status" $taskStatus | Out-Null
    $taskStatus | ConvertTo-Json -Depth 20

    Write-Host "`n[7/7] Fetch metrics sample"
    $webClient = New-Object System.Net.WebClient
    $metrics = $webClient.DownloadString("$BaseUrl/metrics")
    $webClient.Dispose()
    $metricsPath = Join-Path $OutputDir "08-metrics.txt"
    $metrics.Split("`n") |
        Where-Object { $_ -match "agent_requests_total|tool_calls_total|tool_call_duration_seconds|policy_reject_total" } |
        Select-Object -First 40 |
        Set-Content -Path $metricsPath -Encoding UTF8
    Get-Content $metricsPath

    $reportPath = Join-Path $OutputDir "demo-report.html"
    $firstChunk = $null
    if ($search.data.data.chunks.Count -gt 0) {
        $firstChunk = $search.data.data.chunks[0]
    }
    $toolNames = ($tools.data | ForEach-Object { $_.name }) -join ", "
    $html = @"
<!doctype html>
<html>
<head>
  <meta charset="utf-8">
  <title>Go Safe Agent Gateway Demo</title>
  <style>
    body { font-family: Segoe UI, Arial, sans-serif; margin: 32px; color: #18212f; background: #f6f8fb; }
    h1 { margin-bottom: 4px; }
    .grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 16px; }
    .card { background: white; border: 1px solid #d9e1ec; border-radius: 8px; padding: 16px; box-shadow: 0 1px 2px rgba(0,0,0,.04); }
    .wide { grid-column: 1 / -1; }
    code, pre { background: #eef3f8; border-radius: 6px; padding: 2px 5px; }
    pre { padding: 12px; overflow: auto; max-height: 320px; }
    .ok { color: #0b7a3b; font-weight: 600; }
  </style>
</head>
<body>
  <h1>Go Safe Agent Gateway Demo</h1>
  <p>Generated from <code>scripts/demo.ps1</code>.</p>
  <div class="grid">
    <div class="card"><h2>Health</h2><p class="ok">$($health.message)</p></div>
    <div class="card"><h2>Tools</h2><p>$toolNames</p></div>
    <div class="card"><h2>RAG Index</h2><p>Document: <code>$($index.data.document_id)</code></p><p>Chunks: $($index.data.chunks)</p></div>
    <div class="card"><h2>Calculator</h2><p><code>2 + 3 * (4 - 1)</code> = <strong>$($calc.data.data.result)</strong></p></div>
    <div class="card wide"><h2>Qdrant Search Result</h2><p>Title: <strong>$($firstChunk.document_title)</strong></p><p>Score: $($firstChunk.score)</p><p>Source: <code>$($firstChunk.source_path)</code></p><pre>$($firstChunk.content)</pre></div>
    <div class="card wide"><h2>Async Task</h2><p>Task ID: <code>$taskID</code></p><p>Status: <strong>$($taskStatus.data.status)</strong></p></div>
  </div>
</body>
</html>
"@
    $html | Set-Content -Path $reportPath -Encoding UTF8
    Write-Host "`nDemo report: $reportPath"
    Write-Host "Transcript: $transcript"
}
finally {
    Stop-Transcript | Out-Null
}
