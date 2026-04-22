# ============================================================
# tidemarq Manual Test — Script 02: Trigger Mirror Deletions
# Run AFTER the one-way mirror job has completed its first sync.
# Removes files from source-mirror to verify soft-delete behaviour
# in the destination and quarantine queue.
# ============================================================

$Root = "C:\tidemarq-test"

Write-Host "Modifying source-mirror to trigger soft-delete in destination ..." -ForegroundColor Cyan

# Delete two files that were previously synced to the destination.
# After the next mirror job run, these should appear in Quarantine.
$toDelete = @(
    "$Root\source-mirror\active\project-gamma.txt",
    "$Root\source-mirror\shared\config.ini"
)

foreach ($file in $toDelete) {
    if (Test-Path $file) {
        Remove-Item $file -Force
        Write-Host "  Deleted: $file" -ForegroundColor Yellow
    } else {
        Write-Host "  Already gone: $file" -ForegroundColor Gray
    }
}

# Also add a new file — this should sync normally (not quarantine-related).
Set-Content -Path "$Root\source-mirror\active\project-delta.txt" -Value @"
Project Delta — Status: New
Lead: Dave
Deadline: 2025-09-30
Progress: 0%
"@ -Encoding UTF8

Write-Host "  Added: project-delta.txt" -ForegroundColor Green

Write-Host ""
Write-Host "Done. Now run the mirror job again in tidemarq and check:" -ForegroundColor Cyan
Write-Host "  1. project-gamma.txt and config.ini appear in Quarantine (not hard-deleted)"
Write-Host "  2. project-delta.txt appears in the destination"
Write-Host "  3. project-alpha.txt and project-beta.txt remain unchanged in destination"
