# ============================================================
# tidemarq Manual Test — Script 05: Newest-Wins Conflict Setup
# Run AFTER the newest-wins two-way job has completed its
# first sync (both sides identical). Then modify both sides
# with a deliberate time gap so RIGHT is clearly newer.
# ============================================================

$Root = "C:\tidemarq-test"

$leftFile  = "$Root\source-conflict-newest\shared.txt"
$rightFile = "$Root\dest-conflict-newest\shared.txt"

Write-Host "Setting up newest-wins conflict (RIGHT will be newer)..." -ForegroundColor Cyan

# Modify LEFT first
Set-Content -Path $leftFile -Value @"
Shared file — conflict newest-wins test.
MODIFIED ON LEFT — this is the OLDER edit.
Timestamp will be earlier.
"@ -Encoding UTF8
Write-Host "  Modified LEFT (older)" -ForegroundColor Yellow

# Wait 2 seconds so RIGHT has a clearly newer mtime
Start-Sleep -Seconds 2

# Modify RIGHT (will have newer mtime — should WIN)
Set-Content -Path $rightFile -Value @"
Shared file — conflict newest-wins test.
MODIFIED ON RIGHT — this is the NEWER edit.
Timestamp will be later — this content should win.
"@ -Encoding UTF8
Write-Host "  Modified RIGHT (newer — should win)" -ForegroundColor Green

Write-Host ""
Write-Host "Now run the newest-wins job. Expected outcome:" -ForegroundColor Cyan
Write-Host "  - No conflict in the Conflicts view (auto-resolved)"
Write-Host "  - LEFT file content matches RIGHT (newest) content"
Write-Host "  - Audit log shows auto-resolution event"
