# scripts/e2e/teardown.ps1
# Encerra recursos do E2E.
#
# WHY: cloudflared quick tunnels não rotacionam URL automaticamente, mas
# deixar cloudflared rodando consome processo e o tunnel expira sozinho.
# `-Full` também derruba bridge stack (não apaga volume — dados persistem).

[CmdletBinding()]
param(
    [switch]$Full,           # também: docker compose down
    [switch]$PurgeData       # CUIDADO: down -v (apaga DB)
)

$ErrorActionPreference = 'Continue'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir '..\..')

Write-Host "==> Parando cloudflared" -ForegroundColor Cyan
$procs = Get-Process -Name 'cloudflared' -ErrorAction SilentlyContinue
if ($procs) {
    $procs | Stop-Process -Force
    Write-Host "    [OK] $($procs.Count) processo(s) cloudflared encerrado(s)" -ForegroundColor Green
} else {
    Write-Host "    nenhum cloudflared rodando"
}

$tunnelUrlFile = Join-Path $ScriptDir 'tunnel-url.txt'
if (Test-Path $tunnelUrlFile) { Remove-Item $tunnelUrlFile -Force }

if ($Full -or $PurgeData) {
    Write-Host "==> docker compose down" -ForegroundColor Cyan
    Push-Location $RepoRoot
    try {
        if ($PurgeData) {
            Write-Host "    [!] -PurgeData: removendo volume (DB apagada)" -ForegroundColor Yellow
            docker compose down -v
        } else {
            docker compose down
        }
    } finally { Pop-Location }
    if ($PurgeData) {
        $creds = Join-Path $ScriptDir 'tenant-creds.json'
        if (Test-Path $creds) { Remove-Item $creds -Force }
        Write-Host "    [OK] tenant-creds.json removido (tenant não existe mais)" -ForegroundColor Green
    }
} else {
    Write-Host "==> Bridge stack mantido up. Use -Full para derrubar, -PurgeData para apagar DB."
}

Write-Host ""
Write-Host "Teardown concluído." -ForegroundColor Green
