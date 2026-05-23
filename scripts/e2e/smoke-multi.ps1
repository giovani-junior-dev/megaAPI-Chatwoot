# scripts/e2e/smoke-multi.ps1
# SCR-131: CW->WA multi-attachment scenarios (caption-on-first, mixed types, order preservation)

[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$Phone,
    [int]$TimeoutSec = 60
)

$ErrorActionPreference = 'Stop'
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$RepoRoot  = Resolve-Path (Join-Path $ScriptDir '..\..')
$EnvFile   = Join-Path $ScriptDir '.env.e2e'

$envMap = @{}
Get-Content $EnvFile | ForEach-Object {
    $line = $_.Trim()
    if ($line -eq '' -or $line.StartsWith('#')) { return }
    $eq = $line.IndexOf('=')
    if ($eq -lt 1) { return }
    $envMap[$line.Substring(0, $eq).Trim()] = $line.Substring($eq + 1).Trim()
}

$cwBase  = 'http://localhost:3000'
$cwToken = $envMap['CHATWOOT_TOKEN']
$cwAcc   = $envMap['CHATWOOT_ACCOUNT']
$cwInbox = [int]$envMap['CHATWOOT_INBOX']
$slug    = $envMap['TENANT_SLUG']

$jsonHeaders = @{ 'api_access_token' = $cwToken; 'Content-Type' = 'application/json' }

function Cw-Post-Json($path, $body) {
    $json = $body | ConvertTo-Json -Depth 8 -Compress
    return Invoke-RestMethod -Method Post -Uri "$cwBase$path" -Headers $jsonHeaders -Body $json
}
function Cw-Get($path) { return Invoke-RestMethod -Method Get -Uri "$cwBase$path" -Headers $jsonHeaders }

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

function Wait-Final($cwMsgId) {
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

function Cw-Post-MultiAttach($convId, $files, $caption) {
    $uri = "$cwBase/api/v1/accounts/$cwAcc/conversations/$convId/messages"
    $client = [System.Net.Http.HttpClient]::new()
    $client.Timeout = [TimeSpan]::FromSeconds(180)
    $client.DefaultRequestHeaders.Add('api_access_token', $cwToken)
    try {
        $mp = [System.Net.Http.MultipartFormDataContent]::new()
        $mp.Add([System.Net.Http.StringContent]::new('outgoing'), 'message_type')
        $mp.Add([System.Net.Http.StringContent]::new('false'),    'private')
        $mp.Add([System.Net.Http.StringContent]::new($(if ($caption) { $caption } else { '' })), 'content')
        foreach ($f in $files) {
            $bytes = [System.IO.File]::ReadAllBytes($f.Path)
            $part = [System.Net.Http.ByteArrayContent]::new($bytes)
            $part.Headers.ContentType = [System.Net.Http.Headers.MediaTypeHeaderValue]::new($f.Mime)
            $mp.Add($part, 'attachments[]', [System.IO.Path]::GetFileName($f.Path))
        }
        $resp = $client.PostAsync($uri, $mp).GetAwaiter().GetResult()
        $body = $resp.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        if (-not $resp.IsSuccessStatusCode) { throw "HTTP $([int]$resp.StatusCode): $body" }
        return ($body | ConvertFrom-Json).id
    } finally { $client.Dispose() }
}

function Run-Multi($label, $files, $caption, $convId) {
    Write-Host ""
    Write-Host "--- [$label] ($($files.Count) files, caption='$caption') ---" -ForegroundColor Cyan
    try {
        $cwMsgId = Cw-Post-MultiAttach $convId $files $caption
        Write-Host "    cw_msg=$cwMsgId" -ForegroundColor Gray
    } catch {
        Write-Host "    [X] Chatwoot API error: $($_.Exception.Message)" -ForegroundColor Red
        return
    }
    $res = Wait-Final $cwMsgId
    $st = $res.status
    if ($st -eq 'done') {
        $n = $files.Count
        Write-Host "    [OK] PASS status=done - verify WA $n msgs caption ONLY on first" -ForegroundColor Green
    } elseif ($st -eq 'failed') {
        $errstr = $res.error
        Write-Host "    [X] FAIL status=failed error=$errstr" -ForegroundColor Red
    } else {
        Write-Host "    [?] TIMEOUT" -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "SCR-131: CW->WA multi-attachment scenarios | Phone=$Phone" -ForegroundColor Magenta

# Prepare test files
$tmpJpg1 = Join-Path $env:TEMP "multi_a.jpg"
$tmpJpg2 = Join-Path $env:TEMP "multi_b.jpg"
$tmpJpg3 = Join-Path $env:TEMP "multi_c.jpg"
$tmpPng  = Join-Path $env:TEMP "multi_d.png"
$tmpPdf  = Join-Path $env:TEMP "multi_relatorio.pdf"
$tmpZip  = Join-Path $env:TEMP "multi_pacote.zip"

Write-Host "==> Downloading test files..." -ForegroundColor Gray
Invoke-WebRequest -Uri "https://www.w3schools.com/css/img_5terre.jpg" -OutFile $tmpJpg1 -UseBasicParsing
Invoke-WebRequest -Uri "https://www.w3schools.com/css/img_forest.jpg" -OutFile $tmpJpg2 -UseBasicParsing
Invoke-WebRequest -Uri "https://www.w3schools.com/css/img_lights.jpg" -OutFile $tmpJpg3 -UseBasicParsing
Invoke-WebRequest -Uri "https://www.gstatic.com/webp/gallery3/1.png"   -OutFile $tmpPng  -UseBasicParsing
Invoke-WebRequest -Uri "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf" -OutFile $tmpPdf -UseBasicParsing
$zipSrc = Join-Path $env:TEMP "multi_zip_src_$(Get-Random)"
New-Item -ItemType Directory -Force -Path $zipSrc | Out-Null
"hello multi" | Set-Content -Path (Join-Path $zipSrc "test.txt") -Encoding ASCII
if (Test-Path $tmpZip) { Remove-Item $tmpZip -Force }
Compress-Archive -Path (Join-Path $zipSrc "*") -DestinationPath $tmpZip -Force
Remove-Item $zipSrc -Recurse -Force

$contactId = Get-OrCreate-Contact $Phone
$convId    = Create-Conv $contactId $Phone
Write-Host "==> contact=$contactId conv=$convId" -ForegroundColor Gray

# Scenario 1: 3 images + caption (caption-on-first)
Run-Multi "3 imagens + caption" @(
    @{Path=$tmpJpg1; Mime='image/jpeg'},
    @{Path=$tmpJpg2; Mime='image/jpeg'},
    @{Path=$tmpJpg3; Mime='image/jpeg'}
) "Legenda comum nas 3 imagens (deve aparecer só na 1a no WA)" $convId

# Scenario 2: 2 images + 1 document (mixed, order preserved)
Run-Multi "2 imagens + 1 PDF mixed ordem" @(
    @{Path=$tmpJpg1; Mime='image/jpeg'},
    @{Path=$tmpJpg2; Mime='image/jpeg'},
    @{Path=$tmpPdf;  Mime='application/pdf'}
) "" $convId

# Scenario 3: image + png + zip (3 different kinds)
Run-Multi "image + png + zip (mixed kinds)" @(
    @{Path=$tmpJpg1; Mime='image/jpeg'},
    @{Path=$tmpPng;  Mime='image/png'},
    @{Path=$tmpZip;  Mime='application/zip'}
) "" $convId

Remove-Item $tmpJpg1, $tmpJpg2, $tmpJpg3, $tmpPng, $tmpPdf, $tmpZip -Force -EA 0

Write-Host ""
Write-Host "=== Done. Verify WA - separate msgs per file caption only on FIRST scenario 1 ===" -ForegroundColor Magenta
