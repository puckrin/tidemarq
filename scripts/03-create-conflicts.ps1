# ============================================================
# tidemarq Manual Test — Script 03: Create Two-Way Conflicts
# Run AFTER the two-way sync job has completed its FIRST sync
# (so both sides are in a known-synced state).
# Modifies shared-doc.txt on BOTH sides to force a conflict.
# ============================================================

$Root = "C:\tidemarq-test"

Write-Host "Creating conflict scenarios for two-way sync ..." -ForegroundColor Cyan

# ── Conflict 1: shared-doc.txt modified on BOTH sides ────────
# The job's last-synced hash will differ from both current hashes.
$leftDoc = "$Root\source-twoway-left\shared-doc.txt"
$rightDoc = "$Root\source-twoway-right\shared-doc.txt"

Set-Content -Path $leftDoc -Value @"
Shared Collaboration Document
==============================
Last editor: LEFT side
Last edit: 2024-11-10

Section 1: Introduction
This document is shared between both sync locations.

Section 2: Notes
- Item A
- Item B (updated on LEFT)
- Item C
- Item D (added on LEFT)
"@ -Encoding UTF8
Write-Host "  Modified LEFT:  shared-doc.txt" -ForegroundColor Yellow

# Small delay to ensure different mtime
Start-Sleep -Milliseconds 100

Set-Content -Path $rightDoc -Value @"
Shared Collaboration Document
==============================
Last editor: RIGHT side
Last edit: 2024-11-11

Section 1: Introduction
This document is shared between both sync locations.

Section 2: Notes
- Item A (updated on RIGHT)
- Item B
- Item C
- Item E (added on RIGHT)
"@ -Encoding UTF8
Write-Host "  Modified RIGHT: shared-doc.txt" -ForegroundColor Yellow

# ── Conflict 2: reference/manual.txt — only modify LEFT ──────
# This should sync cleanly (not a conflict) — left change propagates to right.
$leftManual = "$Root\source-twoway-left\reference\manual.txt"
Set-Content -Path $leftManual -Value @"
User Manual v1.1
================
Chapter 1: Getting Started
Chapter 2: Configuration
Chapter 3: Troubleshooting
Chapter 4: Advanced Topics (new in v1.1)
"@ -Encoding UTF8
Write-Host "  Modified LEFT:  reference/manual.txt (one-sided — should sync cleanly)" -ForegroundColor Green

Write-Host ""
Write-Host "Done. Now run the two-way job again in tidemarq and verify:" -ForegroundColor Cyan
Write-Host "  1. shared-doc.txt appears in the Conflicts view (changed on both sides)"
Write-Host "  2. reference/manual.txt does NOT appear in Conflicts (one-sided change)"
Write-Host "  3. reference/manual.txt v1.1 content appears on the RIGHT side after sync"
