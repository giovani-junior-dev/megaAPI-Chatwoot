# scripts/e2e/smoke.ps1
# Smoke test: envia uma mensagem outbound via Chatwoot REST e confirma que
# bridge processou (row em `messages` com direction='out' e status='done').
#
# WHY: valida ponta-a-ponta o lado Chatwoot→megaAPI sem precisar abrir UI.
# Não substitui o teste manual de WhatsApp real, mas é o "ping" rápido que
# detecta regressão de HMAC/auth/serialização antes do user perder tempo
# digitando no celular.
#
# Fluxo:
#   1. POST /api/v1/accounts/{acc}/contacts (cria/encontra contato)
#   2. POST /api/v1/accounts/{acc}/conversations (cria conversation no inbox)
#   3. POST /api/v1/accounts/{acc}/conversations/{conv}/messages (envia "out")
#   4. Polling em `messages` até direction='out' AND status='done' (10s)

[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$Phone,   # E.164 sem '+', ex: 5511999999999
    [Parameter(Mandatory=$true)][string]$Text,
    [int]$TimeoutSec = 15
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

$cwBase = 'http://localhost:3000'  # host-side acesso ao Chatwoot
$cwToken = $envMap['CHATWOOT_TOKEN']
$cwAcc = $envMap['CHATWOOT_ACCOUNT']
$cwInbox = [int]$envMap['CHATWOOT_INBOX']
$slug = $envMap['TENANT_SLUG']

$headers = @{
    'api_access_token' = $cwToken
    'Content-Type'     = 'application/json'
}

function Cw-Post($path, $body) {
    $json = $body | ConvertTo-Json -Depth 8 -Compress
    return Invoke-RestMethod -Method Post -Uri "$cwBase$path" -Headers $headers -Body $json
}

function Cw-Get($path) {
    return Invoke-RestMethod -Method Get -Uri "$cwBase$path" -Headers $headers
}

Write-Host "==> [1/4] Cria/encontra contato $Phone no inbox $cwInbox" -ForegroundColor Cyan
try {
    $contactBody = @{
        inbox_id     = $cwInbox
        name         = "E2E $Phone"
        phone_number = "+$Phone"
        identifier   = $Phone
    }
    $contactResp = Cw-Post "/api/v1/accounts/$cwAcc/contacts" $contactBody
    $contactId = $contactResp.payload.contact.id
    Write-Host "    contact_id=$contactId"
} catch {
    # Já existe (HTTP 422) — buscar via search
    Write-Host "    Contato pode já existir, buscando..." -ForegroundColor Yellow
    $search = Cw-Get "/api/v1/accounts/$cwAcc/contacts/search?q=$Phone"
    if ($search.payload.Count -lt 1) {
        Write-Host "[X] Falhou ao criar e não achou na busca: $($_.Exception.Message)" -ForegroundColor Red
        exit 1
    }
    $contactId = $search.payload[0].id
    Write-Host "    contact_id=$contactId (existente)"
}

Write-Host "==> [2/4] Cria conversation" -ForegroundColor Cyan
$convBody = @{
    source_id  = $Phone
    inbox_id   = $cwInbox
    contact_id = $contactId
    status     = 'open'
}
try {
    $convResp = Cw-Post "/api/v1/accounts/$cwAcc/conversations" $convBody
    $convId = $convResp.id
    if (-not $convId) { $convId = $convResp.payload.id }
    Write-Host "    conversation_id=$convId"
} catch {
    Write-Host "[X] Falha ao criar conversation: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "    Resposta: $($_.ErrorDetails.Message)"
    exit 1
}

Write-Host "==> [3/4] Envia mensagem outbound" -ForegroundColor Cyan
$msgBody = @{
    content      = $Text
    message_type = 'outgoing'
    private      = $false
}
try {
    $msgResp = Cw-Post "/api/v1/accounts/$cwAcc/conversations/$convId/messages" $msgBody
    $cwMsgId = $msgResp.id
    Write-Host "    cw_message_id=$cwMsgId — Chatwoot deve disparar webhook agora"
} catch {
    Write-Host "[X] Falha ao enviar message: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host "==> [4/4] Aguardando processamento no bridge (timeout ${TimeoutSec}s)" -ForegroundColor Cyan
$deadline = (Get-Date).AddSeconds($TimeoutSec)
$sql = "SELECT status, COALESCE(last_error, '') FROM messages WHERE tenant_id = (SELECT id FROM tenants WHERE slug='$slug') AND direction='out' AND external_id='$cwMsgId';"

$status = $null
$lastErr = ''
while ((Get-Date) -lt $deadline) {
    Push-Location $RepoRoot
    try {
        $row = docker compose exec -T db psql -U bridge -d bridge -tA -F '|' -c $sql 2>$null
        $row = ($row | Out-String).Trim()
    } finally { Pop-Location }
    if ($row) {
        $parts = $row -split '\|', 2
        $status = $parts[0]
        if ($parts.Count -gt 1) { $lastErr = $parts[1] }
        if ($status -eq 'done' -or $status -eq 'failed') { break }
    }
    Start-Sleep -Milliseconds 500
}

Write-Host ""
if ($status -eq 'done') {
    Write-Host "[OK] SMOKE PASS — message $cwMsgId entregue (status=done)" -ForegroundColor Green
    exit 0
} elseif ($status -eq 'failed') {
    Write-Host "[X] SMOKE FAIL — status=failed last_error='$lastErr'" -ForegroundColor Red
    exit 2
} elseif ($status -eq 'pending') {
    Write-Host "[X] SMOKE TIMEOUT — message ficou em 'pending' por ${TimeoutSec}s" -ForegroundColor Red
    Write-Host "    Provável: worker travado ou megaAPI lenta/inalcançável"
    exit 3
} else {
    Write-Host "[X] SMOKE FAIL — nenhuma row encontrada (Chatwoot webhook não chegou no bridge)" -ForegroundColor Red
    Write-Host "    Verifique: URL do webhook + HMAC secret no Inbox (Settings)"
    exit 4
}
