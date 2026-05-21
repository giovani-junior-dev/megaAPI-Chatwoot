# scripts/e2e/watch.ps1
# Live monitor: bridge logs + messages table polling.
#
# WHY: durante E2E manual user precisa ver simultaneamente (a) o que bridge
# está logando (HMAC mismatch, megaAPI 4xx, retries) e (b) o que está
# entrando na tabela `messages` (direction, status, last_error). Esta view
# diferencia "request chegou e foi descartada" vs "request nunca chegou".

[CmdletBinding()]
param(
    [int]$PollSec = 2,
    [int]$Limit = 15
)

$ErrorActionPreference = 'Stop'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir '..\..')
$EnvFile = Join-Path $ScriptDir '.env.e2e'

if (-not (Test-Path $EnvFile)) {
    Write-Host "[X] .env.e2e ausente — rode setup.ps1 primeiro" -ForegroundColor Red
    exit 1
}

$envMap = @{}
Get-Content $EnvFile | ForEach-Object {
    $line = $_.Trim()
    if ($line -eq '' -or $line.StartsWith('#')) { return }
    $eq = $line.IndexOf('=')
    if ($eq -lt 1) { return }
    $envMap[$line.Substring(0, $eq).Trim()] = $line.Substring($eq + 1).Trim()
}
$slug = $envMap['TENANT_SLUG']

Write-Host "==> Streaming bridge logs em janela nova" -ForegroundColor Cyan
$logCmd = "cd '$RepoRoot'; docker compose logs -f --tail 30 bridge"
Start-Process powershell -ArgumentList @('-NoExit','-Command',$logCmd) | Out-Null

Write-Host "==> Polling messages table (Ctrl+C para sair)" -ForegroundColor Cyan
Write-Host "    slug=$slug | refresh=${PollSec}s | limit=$Limit"
Write-Host ""

$sql = @"
SELECT
  substring(id::text, 1, 8) AS id,
  direction AS dir,
  status,
  attempts AS att,
  substring(external_id, 1, 30) AS external_id,
  to_char(created_at, 'HH24:MI:SS') AS at,
  COALESCE(substring(last_error, 1, 50), '') AS last_error
FROM messages
WHERE tenant_id = (SELECT id FROM tenants WHERE slug = '$slug')
ORDER BY created_at DESC
LIMIT $Limit;
"@

while ($true) {
    Clear-Host
    Write-Host "==> messages (tenant=$slug) — $(Get-Date -Format 'HH:mm:ss')" -ForegroundColor Cyan
    Push-Location $RepoRoot
    try {
        docker compose exec -T db psql -U bridge -d bridge -c $sql 2>&1 | ForEach-Object { Write-Host $_ }
    } finally {
        Pop-Location
    }
    Start-Sleep -Seconds $PollSec
}
