# scripts/e2e/smoke-audio.ps1
# SCR-126: CW->WA audio scenarios (mp3 -> ptt, ogg -> ptt, wav -> reject)

[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$Phone,
    [int]$TimeoutSec = 30
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

function Cw-Post-Audio($convId, $filePath, $mime, $caption) {
    $uri = "$cwBase/api/v1/accounts/$cwAcc/conversations/$convId/messages"
    $client = [System.Net.Http.HttpClient]::new()
    $client.DefaultRequestHeaders.Add('api_access_token', $cwToken)
    try {
        $mp = [System.Net.Http.MultipartFormDataContent]::new()
        $mp.Add([System.Net.Http.StringContent]::new('outgoing'), 'message_type')
        $mp.Add([System.Net.Http.StringContent]::new('false'),    'private')
        $mp.Add([System.Net.Http.StringContent]::new($(if ($caption) { $caption } else { '' })), 'content')

        $bytes = [System.IO.File]::ReadAllBytes($filePath)
        $part  = [System.Net.Http.ByteArrayContent]::new($bytes)
        $part.Headers.ContentType = [System.Net.Http.Headers.MediaTypeHeaderValue]::new($mime)
        $mp.Add($part, 'attachments[]', [System.IO.Path]::GetFileName($filePath))

        $resp = $client.PostAsync($uri, $mp).GetAwaiter().GetResult()
        $body = $resp.Content.ReadAsStringAsync().GetAwaiter().GetResult()
        if (-not $resp.IsSuccessStatusCode) {
            throw "HTTP $([int]$resp.StatusCode): $body"
        }
        return ($body | ConvertFrom-Json).id
    } finally { $client.Dispose() }
}

function Run-Audio-Scenario($label, $tmpFile, $mime, $caption, $convId, $expectFail) {
    Write-Host ""
    Write-Host "--- [$label] ---" -ForegroundColor Cyan
    try {
        $cwMsgId = Cw-Post-Audio $convId $tmpFile $mime $caption
        Write-Host "    cw_msg=$cwMsgId conv=$convId" -ForegroundColor Gray
    } catch {
        Write-Host "    [X] Chatwoot API error: $($_.Exception.Message)" -ForegroundColor Red
        return
    }
    $res = Wait-Final $cwMsgId
    if ($expectFail) {
        if ($res.status -eq 'failed') {
            Write-Host "    [OK] PASS expected fail status=failed error=$($res.error)" -ForegroundColor Green
        } else {
            Write-Host "    [X] FAIL expected fail but got status=$($res.status)" -ForegroundColor Red
        }
    } else {
        if ($res.status -eq 'done') {
            Write-Host "    [OK] PASS status=done - verify WA delivery (voice note)" -ForegroundColor Green
        } elseif ($res.status -eq 'failed') {
            Write-Host "    [X] FAIL status=failed error=$($res.error)" -ForegroundColor Red
        } else {
            Write-Host "    [?] TIMEOUT after ${TimeoutSec}s" -ForegroundColor Yellow
        }
    }
}

Write-Host ""
Write-Host "SCR-126: CW->WA audio scenarios | Phone=$Phone" -ForegroundColor Magenta

$tmpMp3 = Join-Path $env:TEMP "e2e_audio.mp3"
$tmpOgg = Join-Path $env:TEMP "e2e_audio.ogg"
$tmpWav = Join-Path $env:TEMP "e2e_audio.wav"

Write-Host "==> Downloading test audios..." -ForegroundColor Gray
# Public sample audios (W3C / archive.org)
Invoke-WebRequest -Uri "https://www.w3schools.com/html/horse.mp3"  -OutFile $tmpMp3 -UseBasicParsing
Invoke-WebRequest -Uri "https://www.w3schools.com/html/horse.ogg"  -OutFile $tmpOgg -UseBasicParsing
# Tiny wav (CC0 sample)
Invoke-WebRequest -Uri "https://www.kozco.com/tech/piano2.wav"     -OutFile $tmpWav -UseBasicParsing
Write-Host "    mp3: $([math]::Round((Get-Item $tmpMp3).Length/1KB,1))KB"
Write-Host "    ogg: $([math]::Round((Get-Item $tmpOgg).Length/1KB,1))KB"
Write-Host "    wav: $([math]::Round((Get-Item $tmpWav).Length/1KB,1))KB"

$contactId = Get-OrCreate-Contact $Phone
$convId    = Create-Conv $contactId $Phone
Write-Host "==> contact=$contactId conv=$convId" -ForegroundColor Gray

Run-Audio-Scenario "CW->WA MP3 (expect PTT voice note)"   $tmpMp3 "audio/mpeg" $null $convId $false
Run-Audio-Scenario "CW->WA OGG (expect PTT voice note)"   $tmpOgg "audio/ogg"  $null $convId $false
Run-Audio-Scenario "CW->WA WAV (expect REJECT)"           $tmpWav "audio/wav"  $null $convId $true

Remove-Item $tmpMp3, $tmpOgg, $tmpWav -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "=== Done. Verify WA on $Phone ===" -ForegroundColor Magenta
