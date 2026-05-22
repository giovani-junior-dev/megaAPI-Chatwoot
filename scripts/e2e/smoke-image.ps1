# scripts/e2e/smoke-image.ps1
# SCR-125: CW->WA bidirectional image scenarios
# WHY: Chatwoot attachment upload requires multipart/form-data, not JSON.
# Flow: download image locally -> multipart POST to Chatwoot -> Chatwoot fires
# webhook with data_url -> bridge relays to megaAPI -> WA receives.
# Size guard tested via direct fake webhook (DEBUG_SKIP_HMAC=1 required).

[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$Phone,
    [int]$TimeoutSec = 25
)

$ErrorActionPreference = 'Stop'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot = Resolve-Path (Join-Path $ScriptDir '..\..')
$EnvFile = Join-Path $ScriptDir '.env.e2e'

$envMap = @{}
Get-Content $EnvFile | ForEach-Object {
    $line = $_.Trim()
    if ($line -eq '' -or $line.StartsWith('#')) { return }
    $eq = $line.IndexOf('=')
    if ($eq -lt 1) { return }
    $envMap[$line.Substring(0, $eq).Trim()] = $line.Substring($eq + 1).Trim()
}

$cwBase   = 'http://localhost:3000'
$cwToken  = $envMap['CHATWOOT_TOKEN']
$cwAcc    = $envMap['CHATWOOT_ACCOUNT']
$cwInbox  = [int]$envMap['CHATWOOT_INBOX']
$slug     = $envMap['TENANT_SLUG']
$bridgeBase = "http://localhost:$($envMap['BRIDGE_HOST_PORT'])"
$creds    = Get-Content (Join-Path $ScriptDir 'tenant-creds.json') -Raw | ConvertFrom-Json

$jsonHeaders = @{ 'api_access_token' = $cwToken; 'Content-Type' = 'application/json' }
$authHeaders = @{ 'api_access_token' = $cwToken }

function Cw-Post-Json($path, $body) {
    $json = $body | ConvertTo-Json -Depth 8 -Compress
    return Invoke-RestMethod -Method Post -Uri "$cwBase$path" -Headers $jsonHeaders -Body $json
}
function Cw-Get($path) {
    return Invoke-RestMethod -Method Get -Uri "$cwBase$path" -Headers $jsonHeaders
}

function Get-OrCreate-Contact($phone) {
    try {
        $r = Cw-Post-Json "/api/v1/accounts/$cwAcc/contacts" @{
            inbox_id = $cwInbox; name = "E2E $phone"
            phone_number = "+$phone"; identifier = $phone
        }
        return $r.payload.contact.id
    } catch {
        $s = Cw-Get "/api/v1/accounts/$cwAcc/contacts/search?q=$phone"
        return $s.payload[0].id
    }
}

function Create-Conv($contactId, $phone) {
    $r = Cw-Post-Json "/api/v1/accounts/$cwAcc/conversations" @{
        source_id = $phone; inbox_id = $cwInbox; contact_id = $contactId; status = 'open'
    }
    $id = $r.id; if (-not $id) { $id = $r.payload.id }
    return $id
}

function Wait-Done($cwMsgId) {
    $sql = "SELECT status, COALESCE(last_error,'') FROM messages WHERE tenant_id=(SELECT id FROM tenants WHERE slug='$slug') AND direction='out' AND external_id='cw-$cwMsgId';"
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        Push-Location $RepoRoot
        try { $row = docker compose exec -T db psql -U bridge -d bridge -tA -F '|' -c $sql 2>$null }
        finally { Pop-Location }
        $row = ($row | Out-String).Trim()
        if ($row) {
            $parts = $row -split '\|', 2
            $st = $parts[0]
            $err = if ($parts.Count -gt 1) { $parts[1] } else { '' }
            if ($st -eq 'done' -or $st -eq 'failed') { return @{status=$st; error=$err} }
        }
        Start-Sleep -Milliseconds 500
    }
    return @{status='timeout'; error=''}
}

Add-Type -AssemblyName System.Net.Http

function Cw-Post-Multipart($convId, $filePath, $caption) {
    $uri = "$cwBase/api/v1/accounts/$cwAcc/conversations/$convId/messages"
    $handler = [System.Net.Http.HttpClientHandler]::new()
    $client  = [System.Net.Http.HttpClient]::new($handler)
    $client.DefaultRequestHeaders.Add('api_access_token', $cwToken)
    try {
        $mp = [System.Net.Http.MultipartFormDataContent]::new()
        $mp.Add([System.Net.Http.StringContent]::new('outgoing'),  'message_type')
        $mp.Add([System.Net.Http.StringContent]::new('false'),     'private')
        $mp.Add([System.Net.Http.StringContent]::new($(if ($caption) { $caption } else { '' })), 'content')

        $ext  = [System.IO.Path]::GetExtension($filePath).ToLower()
        $mime = if ($ext -eq '.png') { 'image/png' } else { 'image/jpeg' }
        $name = [System.IO.Path]::GetFileName($filePath)
        $bytes   = [System.IO.File]::ReadAllBytes($filePath)
        $imgPart = [System.Net.Http.ByteArrayContent]::new($bytes)
        $imgPart.Headers.ContentType = [System.Net.Http.Headers.MediaTypeHeaderValue]::new($mime)
        $mp.Add($imgPart, 'attachments[]', $name)

        $resp    = $client.PostAsync($uri, $mp).GetAwaiter().GetResult()
        $body    = $resp.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        if (-not $resp.IsSuccessStatusCode) {
            throw "HTTP $([int]$resp.StatusCode): $body"
        }
        return ($body | ConvertFrom-Json).id
    } finally {
        $client.Dispose()
    }
}

function Run-Image-Scenario($label, $tmpFile, $caption, $convId) {
    Write-Host ""
    Write-Host "--- [$label] ---" -ForegroundColor Cyan
    try {
        $cwMsgId = Cw-Post-Multipart $convId $tmpFile $caption
        Write-Host "    cw_msg=$cwMsgId conv=$convId" -ForegroundColor Gray
    } catch {
        Write-Host "    [X] Chatwoot API error: $($_.Exception.Message)" -ForegroundColor Red
        return
    }
    $res = Wait-Done $cwMsgId
    if ($res.status -eq 'done') {
        Write-Host "    [OK] PASS status=done - verify WA delivery manually" -ForegroundColor Green
    } elseif ($res.status -eq 'failed') {
        Write-Host "    [X] FAIL status=failed error=$($res.error)" -ForegroundColor Red
    } else {
        Write-Host "    [?] TIMEOUT no row after ${TimeoutSec}s" -ForegroundColor Yellow
    }
}

# ----- Download test images to temp -----
Write-Host ""
Write-Host "SCR-125: CW->WA image scenarios | Phone=$Phone" -ForegroundColor Magenta

$tmpJpg = Join-Path $env:TEMP "e2e_test.jpg"
$tmpPng = Join-Path $env:TEMP "e2e_test.png"

Write-Host "==> Downloading test images..." -ForegroundColor Gray
Invoke-WebRequest -Uri "https://www.w3schools.com/css/img_5terre.jpg" -OutFile $tmpJpg -UseBasicParsing
Invoke-WebRequest -Uri "https://www.gstatic.com/webp/gallery3/1.png" -OutFile $tmpPng -UseBasicParsing
Write-Host "    JPG: $([math]::Round((Get-Item $tmpJpg).Length/1KB,1))KB"
Write-Host "    PNG: $([math]::Round((Get-Item $tmpPng).Length/1KB,1))KB"

$contactId = Get-OrCreate-Contact $Phone
$convId = Create-Conv $contactId $Phone
Write-Host "==> Using contact_id=$contactId conv_id=$convId" -ForegroundColor Gray

# Scenario 1: JPG no caption
Run-Image-Scenario "CW->WA JPG no caption" $tmpJpg $null $convId

# Scenario 2: JPG with caption
Run-Image-Scenario "CW->WA JPG with caption" $tmpJpg "Veja esta imagem de teste" $convId

# Scenario 3: PNG
Run-Image-Scenario "CW->WA PNG" $tmpPng $null $convId

# ----- Scenario 4: >5MB size guard via direct fake webhook -----
Write-Host ""
Write-Host "--- [CW->WA >5MB size guard (direct webhook)] ---" -ForegroundColor Cyan
# Craft fake CW webhook payload with a >5MB URL
# Bridge will HEAD the URL, see Content-Length >5MB, return notRetriable
$bigUrl  = "https://upload.wikimedia.org/wikipedia/commons/9/9d/NASA_Mars_Rover.jpg"
$fakePayload = @{
    event        = 'message_created'
    message_type = 'outgoing'
    private      = $false
    id           = 9999
    content      = ''
    conversation = @{
        id            = 9999
        contact_inbox = @{ source_id = $Phone }
    }
    sender       = @{ name = 'E2E'; phone_number = "+$Phone" }
    attachments  = @(@{ file_type = 'image'; data_url = $bigUrl })
} | ConvertTo-Json -Depth 8 -Compress

try {
    $r = Invoke-RestMethod -Method Post `
        -Uri "$bridgeBase/v1/cw/$slug" `
        -ContentType 'application/json' `
        -Body $fakePayload `
        -Headers @{ 'X-Chatwoot-Signature' = 'sha256=skip' }
    Write-Host "    Bridge queued (status=$($r.status))" -ForegroundColor Gray
    Start-Sleep -Seconds 8
    $sql = "SELECT status, last_error FROM messages WHERE external_id='cw-9999' AND tenant_id=(SELECT id FROM tenants WHERE slug='$slug');"
    Push-Location $RepoRoot
    try { $row = docker compose exec -T db psql -U bridge -d bridge -tA -F '|' -c $sql 2>$null }
    finally { Pop-Location }
    $row = ($row | Out-String).Trim()
    if ($row -match 'failed') {
        $errPart = ($row -split [regex]::Escape('|'))[1]
        Write-Host "    [OK] PASS size guard fired status=failed error=$errPart" -ForegroundColor Green
    } elseif ($row -match 'done') {
        Write-Host "    WARN message went done (size guard did not fire or no Content-Length)" -ForegroundColor Yellow
    } else {
        Write-Host "    ROW: $row" -ForegroundColor Yellow
    }
} catch {
    Write-Host "    [X] Error: $($_.Exception.Message)" -ForegroundColor Red
}

# Cleanup
Remove-Item $tmpJpg, $tmpPng -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "=== Done. Verify WA on $Phone + ngrok http://localhost:4040 ===" -ForegroundColor Magenta
