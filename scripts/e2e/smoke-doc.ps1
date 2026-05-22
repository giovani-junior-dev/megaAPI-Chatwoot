# scripts/e2e/smoke-doc.ps1
# SCR-128: CW->WA document scenarios (PDF, DOCX, XLSX, ZIP)

[CmdletBinding()]
param(
    [Parameter(Mandatory=$true)][string]$Phone,
    [int]$TimeoutSec = 40
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

function Cw-Post-Doc($convId, $filePath, $mime, $caption) {
    $uri = "$cwBase/api/v1/accounts/$cwAcc/conversations/$convId/messages"
    $client = [System.Net.Http.HttpClient]::new()
    $client.Timeout = [TimeSpan]::FromSeconds(120)
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
        if (-not $resp.IsSuccessStatusCode) { throw "HTTP $([int]$resp.StatusCode): $body" }
        return ($body | ConvertFrom-Json).id
    } finally { $client.Dispose() }
}

function Run-Doc-Scenario($label, $tmpFile, $mime, $convId) {
    Write-Host ""
    Write-Host "--- [$label] ---" -ForegroundColor Cyan
    try {
        $cwMsgId = Cw-Post-Doc $convId $tmpFile $mime $null
        Write-Host "    cw_msg=$cwMsgId conv=$convId" -ForegroundColor Gray
    } catch {
        Write-Host "    [X] Chatwoot API error: $($_.Exception.Message)" -ForegroundColor Red
        return
    }
    $res = Wait-Final $cwMsgId
    if ($res.status -eq 'done') {
        Write-Host "    [OK] PASS status=done - verify WA delivery + filename" -ForegroundColor Green
    } elseif ($res.status -eq 'failed') {
        Write-Host "    [X] FAIL status=failed error=$($res.error)" -ForegroundColor Red
    } else {
        Write-Host "    [?] TIMEOUT after ${TimeoutSec}s" -ForegroundColor Yellow
    }
}

Write-Host ""
Write-Host "SCR-128: CW->WA document scenarios | Phone=$Phone" -ForegroundColor Magenta

$tmpPdf  = Join-Path $env:TEMP "e2e_relatorio.pdf"
$tmpDocx = Join-Path $env:TEMP "e2e_contrato.docx"
$tmpXlsx = Join-Path $env:TEMP "e2e_planilha.xlsx"
$tmpZip  = Join-Path $env:TEMP "e2e_pacote.zip"

Write-Host "==> Preparing test documents..." -ForegroundColor Gray
# PDF: public dummy
Invoke-WebRequest -Uri "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf" -OutFile $tmpPdf -UseBasicParsing
# DOCX/XLSX: build minimal valid Open XML files via ZIP structure
function Build-MinimalDocx($path) {
    $dir = Join-Path $env:TEMP "e2e_docx_src_$(Get-Random)"
    New-Item -ItemType Directory -Force -Path "$dir/_rels"      | Out-Null
    New-Item -ItemType Directory -Force -Path "$dir/word"       | Out-Null
    @'
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>
'@ | Out-File -Encoding utf8 -LiteralPath "$dir/[Content_Types].xml"
    @'
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>
'@ | Out-File -Encoding utf8 -FilePath "$dir/_rels/.rels"
    @'
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body><w:p><w:r><w:t>E2E test document SCR-128</w:t></w:r></w:p></w:body>
</w:document>
'@ | Out-File -Encoding utf8 -FilePath "$dir/word/document.xml"
    if (Test-Path $path) { Remove-Item $path -Force }
    $tmpZip = "$path.zip"
    Compress-Archive -Path "$dir/*" -DestinationPath $tmpZip -Force
    Move-Item -Path $tmpZip -Destination $path -Force
    Remove-Item $dir -Recurse -Force
}
function Build-MinimalXlsx($path) {
    $dir = Join-Path $env:TEMP "e2e_xlsx_src_$(Get-Random)"
    New-Item -ItemType Directory -Force -Path "$dir/_rels" | Out-Null
    New-Item -ItemType Directory -Force -Path "$dir/xl/_rels" | Out-Null
    New-Item -ItemType Directory -Force -Path "$dir/xl/worksheets" | Out-Null
    @'
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>
'@ | Out-File -Encoding utf8 -LiteralPath "$dir/[Content_Types].xml"
    @'
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>
'@ | Out-File -Encoding utf8 -FilePath "$dir/_rels/.rels"
    @'
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets>
</workbook>
'@ | Out-File -Encoding utf8 -FilePath "$dir/xl/workbook.xml"
    @'
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>
'@ | Out-File -Encoding utf8 -FilePath "$dir/xl/_rels/workbook.xml.rels"
    @'
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
<sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>E2E SCR-128</t></is></c></row></sheetData>
</worksheet>
'@ | Out-File -Encoding utf8 -FilePath "$dir/xl/worksheets/sheet1.xml"
    if (Test-Path $path) { Remove-Item $path -Force }
    $tmpZip = "$path.zip"
    Compress-Archive -Path "$dir/*" -DestinationPath $tmpZip -Force
    Move-Item -Path $tmpZip -Destination $path -Force
    Remove-Item $dir -Recurse -Force
}
Build-MinimalDocx $tmpDocx
Build-MinimalXlsx $tmpXlsx
# ZIP: build locally
$zipSrc = Join-Path $env:TEMP "e2e_zip_src_$(Get-Random)"
New-Item -ItemType Directory -Force -Path $zipSrc | Out-Null
"hello world" | Set-Content -Path (Join-Path $zipSrc "hello.txt") -Encoding ASCII
if (Test-Path $tmpZip) { Remove-Item $tmpZip -Force }
Compress-Archive -Path (Join-Path $zipSrc "*") -DestinationPath $tmpZip -Force
Remove-Item $zipSrc -Recurse -Force

Write-Host "    pdf:  $([math]::Round((Get-Item $tmpPdf).Length/1KB,1))KB"
Write-Host "    docx: $([math]::Round((Get-Item $tmpDocx).Length/1KB,1))KB"
Write-Host "    xlsx: $([math]::Round((Get-Item $tmpXlsx).Length/1KB,1))KB"
Write-Host "    zip:  $([math]::Round((Get-Item $tmpZip).Length/1KB,1))KB"

$contactId = Get-OrCreate-Contact $Phone
$convId    = Create-Conv $contactId $Phone
Write-Host "==> contact=$contactId conv=$convId" -ForegroundColor Gray

Run-Doc-Scenario "CW->WA PDF"  $tmpPdf  "application/pdf" $convId
Run-Doc-Scenario "CW->WA DOCX" $tmpDocx "application/vnd.openxmlformats-officedocument.wordprocessingml.document" $convId
Run-Doc-Scenario "CW->WA XLSX" $tmpXlsx "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"        $convId
Run-Doc-Scenario "CW->WA ZIP"  $tmpZip  "application/zip" $convId

Remove-Item $tmpPdf, $tmpDocx, $tmpXlsx, $tmpZip -Force -ErrorAction SilentlyContinue

Write-Host ""
Write-Host "=== Done. Verify WA on $Phone (filenames preserved) ===" -ForegroundColor Magenta
