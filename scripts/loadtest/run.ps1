# run.ps1 - drive k6 against POST /v1/wa/{slug} and diff DB row count.
#
# Why: SCR-72 client-side load baseline. k6 measures p99 ACK latency; the
# wrapper records start timestamp, runs k6 in docker, then counts inserted
# `messages` rows attributed to RUN_TAG via payload->>'key'->>'id' LIKE.
#
# Required env / params:
#   -Bearer         tenant webhook bearer (REQUIRED)
#   -Slug           tenant slug (default: loadtest)
#   -Rate           target rps (default: 1000)
#   -Duration       k6 duration string (default: 5m)
#   -BaseUrl        bridge base url from the docker host (default: http://localhost:8090)
#   -ComposeNet     compose network name (default: chatwoot-megaapi-bridge_default)
#   -DbService      compose service name for postgres (default: db)
#   -PreVUs/MaxVUs  k6 VU pool tuning (defaults 200/500)

param(
    [Parameter(Mandatory = $true)][string]$Bearer,
    [string]$Slug = 'loadtest',
    [int]$Rate = 1000,
    [string]$Duration = '5m',
    [string]$BaseUrl = 'http://host.docker.internal:8090',
    [string]$ComposeNet = 'chatwoot-megaapi-bridge_default',
    [string]$DbService = 'chatwoot-megaapi-bridge-db-1',
    [int]$PreVUs = 200,
    [int]$MaxVUs = 500
)

$ErrorActionPreference = 'Stop'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RunTag = "run-$(Get-Date -Format yyyyMMddHHmmss)"
$StartTs = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ss.fffZ')

Write-Host "=== SCR-72 load test ==="
Write-Host "RUN_TAG     : $RunTag"
Write-Host "Start (UTC) : $StartTs"
Write-Host "Target      : $BaseUrl/v1/wa/$Slug @ $Rate rps for $Duration"
Write-Host ""

# Pull image up-front so the timer doesn't include the download.
docker pull grafana/k6:latest | Out-Null

$dockerArgs = @(
    'run', '--rm',
    '--network', $ComposeNet,
    '-v', "${ScriptDir}:/scripts",
    '-e', "BASE_URL=$BaseUrl",
    '-e', "SLUG=$Slug",
    '-e', "BEARER=$Bearer",
    '-e', "RATE=$Rate",
    '-e', "DURATION=$Duration",
    '-e', "PRE_VUS=$PreVUs",
    '-e', "MAX_VUS=$MaxVUs",
    '-e', "RUN_TAG=$RunTag",
    'grafana/k6:latest', 'run', '/scripts/wa-webhook.js'
)

# Add host-gateway only when targeting host.docker.internal so the container
# can reach the bridge published port from inside the compose network.
if ($BaseUrl -match 'host\.docker\.internal') {
    $dockerArgs = @('run', '--rm', '--add-host', 'host.docker.internal:host-gateway') + $dockerArgs[1..($dockerArgs.Length - 1)]
}

$k6Output = & docker @dockerArgs 2>&1
$k6Exit = $LASTEXITCODE
$k6Output | ForEach-Object { Write-Host $_ }

Write-Host ""
Write-Host "=== Post-run DB count ==="

# Count messages inserted during this run by RUN_TAG prefix; the wa payload
# `key.id` is shaped as `${RUN_TAG}-${vu}-${iter}` (see wa-webhook.js).
$sql = @"
SELECT count(*) FROM messages
 WHERE direction = 'in'
   AND external_id LIKE '${RunTag}-%';
"@

$dbCount = docker exec $DbService psql -U bridge -d bridge -tA -c $sql
$dbCount = ($dbCount | Out-String).Trim()
Write-Host "DB inserted rows (RUN_TAG=$RunTag): $dbCount"

# k6 iteration count from summary.json
$summaryPath = Join-Path $ScriptDir 'summary.json'
if (Test-Path $summaryPath) {
    $summary = Get-Content $summaryPath -Raw | ConvertFrom-Json
    $iters = [int]$summary.metrics.iterations.values.count
    $p99 = $summary.metrics.http_req_duration.values.'p(99)'
    $p95 = $summary.metrics.http_req_duration.values.'p(95)'
    $p50 = $summary.metrics.http_req_duration.values.'p(50)'
    $failRate = $summary.metrics.http_req_failed.values.rate
    Write-Host "k6 iterations            : $iters"
    Write-Host "Delta (iters - db_rows)  : $($iters - [int]$dbCount)"
    Write-Host "p50 / p95 / p99 (ms)     : $p50 / $p95 / $p99"
    Write-Host "http_req_failed rate     : $failRate"
} else {
    Write-Host "WARN: summary.json not produced - check k6 exit/logs above."
}

if ($k6Exit -ne 0) {
    Write-Host "k6 exit code $k6Exit - threshold breach treated as fail"
}

exit $k6Exit
