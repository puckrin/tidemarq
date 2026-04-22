# ============================================================
# tidemarq Manual Test — Script 04: Increment Version Content
# Run this script multiple times (with a job run between each)
# to build up version history on evolving-doc.txt.
# Each run appends a timestamped edit to simulate real usage.
# ============================================================

$Root    = "C:\tidemarq-test"
$DocPath = "$Root\source-versions\evolving-doc.txt"

# Read current content to determine which version we're writing
$current = if (Test-Path $DocPath) { Get-Content $DocPath -Raw } else { "" }
$verMatch = [regex]::Match($current, 'Version (\d+)')
$nextVer  = if ($verMatch.Success) { [int]$verMatch.Groups[1].Value + 1 } else { 2 }

$timestamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"

Set-Content -Path $DocPath -Value @"
Evolving Document — Version $nextVer
==============================
Updated: $timestamp

This document has been updated $nextVer times.
Run 04-update-for-versions.ps1 again after the next job sync
to increment to version $($nextVer + 1).

Change log:
$(for ($i = 1; $i -le $nextVer; $i++) { "  v$i — Edit recorded" })

Status: ACTIVE
"@ -Encoding UTF8

Write-Host "evolving-doc.txt updated to Version $nextVer ($timestamp)" -ForegroundColor Green
Write-Host ""
Write-Host "Now run the 'Version History' job in tidemarq, then:"
Write-Host "  - Check Job Detail for version count on this file"
Write-Host "  - Run this script again to create version $($nextVer + 1)"
