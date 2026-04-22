# ============================================================
# tidemarq Manual Test — Script 01: Initial Filesystem Setup
# Run this ONCE before starting the test suite.
# Creates all test directories and seed files under C:\tidemarq-test\
# ============================================================

$Root = "C:\tidemarq-test"

function New-Dir($Path) {
    if (-not (Test-Path $Path)) { New-Item -ItemType Directory -Path $Path -Force | Out-Null }
}

function New-File($Path, $Content) {
    New-Dir (Split-Path $Path)
    Set-Content -Path $Path -Value $Content -Encoding UTF8
}

function New-BinaryFile($Path, $SizeKB) {
    New-Dir (Split-Path $Path)
    $bytes = New-Object byte[] ($SizeKB * 1024)
    (New-Object Random).NextBytes($bytes)
    [System.IO.File]::WriteAllBytes($Path, $bytes)
}

Write-Host "Creating tidemarq test filesystem at $Root ..." -ForegroundColor Cyan

# ── One-way BACKUP test tree ─────────────────────────────────
New-Dir "$Root\dest-backup"

New-File "$Root\source-backup\documents\report-q4.txt" @"
Q4 Financial Report
===================
Revenue:  $1,250,000
Expenses: $980,000
Net:      $270,000

Prepared by: Finance Team
Date: 2024-12-01
"@

New-File "$Root\source-backup\documents\meeting-notes.txt" @"
Meeting Notes — 2024-11-15
Attendees: Alice, Bob, Carol
Action items:
  - Alice: Submit budget by Friday
  - Bob: Review contracts
  - Carol: Update roadmap
"@

New-File "$Root\source-backup\documents\archive\old-report-q3.txt" @"
Q3 Financial Report (ARCHIVED)
Revenue: $1,100,000
"@

New-File "$Root\source-backup\documents\archive\legacy-data.csv" @"
id,name,value,date
1,Widget A,29.99,2024-01-10
2,Widget B,49.99,2024-01-11
3,Widget C,9.99,2024-01-12
4,Widget D,99.99,2024-01-13
5,Widget E,14.99,2024-01-14
"@

New-File "$Root\source-backup\images\photo-caption-001.txt" "Sunset at the beach — July 2024"
New-File "$Root\source-backup\images\photo-caption-002.txt" "Team lunch — August 2024"
New-File "$Root\source-backup\images\photo-caption-003.txt" "Product launch — September 2024"
New-File "$Root\source-backup\images\raw\shoot-notes.txt" "ISO 200, f/4.5, 1/500s — outdoor natural light"

New-File "$Root\source-backup\config.json" @"
{
  "app": "tidemarq-test",
  "version": "1.0.0",
  "debug": false,
  "timeout": 30,
  "retries": 3
}
"@

# ~5 MB binary file to make progress bar visible
New-BinaryFile "$Root\source-backup\media\sample-5mb.bin" 5120

Write-Host "  [OK] source-backup" -ForegroundColor Green

# ── One-way MIRROR test tree ──────────────────────────────────
New-Dir "$Root\dest-mirror"

New-File "$Root\source-mirror\active\project-alpha.txt" @"
Project Alpha — Status: Active
Lead: Alice
Deadline: 2025-03-31
Progress: 45%
"@

New-File "$Root\source-mirror\active\project-beta.txt" @"
Project Beta — Status: Active
Lead: Bob
Deadline: 2025-06-30
Progress: 20%
"@

New-File "$Root\source-mirror\active\project-gamma.txt" @"
Project Gamma — Status: Active
Lead: Carol
Deadline: 2025-01-15
Progress: 80%
"@

New-File "$Root\source-mirror\shared\readme.md" @"
# Shared Resources
This directory contains shared project assets.
Do not modify without team approval.
"@

New-File "$Root\source-mirror\shared\config.ini" @"
[database]
host=localhost
port=5432
name=projects

[cache]
ttl=3600
max_size=512
"@

Write-Host "  [OK] source-mirror" -ForegroundColor Green

# ── Two-way SYNC test tree ────────────────────────────────────
# Both sides start with IDENTICAL content so first sync is clean
$sharedDocContent = @"
Shared Collaboration Document
==============================
Last editor: (none)
Last edit: 2024-11-01

Section 1: Introduction
This document is shared between both sync locations.

Section 2: Notes
- Item A
- Item B
- Item C
"@

New-File "$Root\source-twoway-left\shared-doc.txt" $sharedDocContent
New-File "$Root\source-twoway-left\left-only.txt" @"
Left-side local notes
Created: 2024-11-01
Content only relevant to the left location.
"@
New-File "$Root\source-twoway-left\reference\manual.txt" @"
User Manual v1.0
================
Chapter 1: Getting Started
Chapter 2: Configuration
Chapter 3: Troubleshooting
"@

New-File "$Root\source-twoway-right\shared-doc.txt" $sharedDocContent
New-File "$Root\source-twoway-right\right-only.txt" @"
Right-side local notes
Created: 2024-11-01
Content only relevant to the right location.
"@
New-File "$Root\source-twoway-right\reference\manual.txt" @"
User Manual v1.0
================
Chapter 1: Getting Started
Chapter 2: Configuration
Chapter 3: Troubleshooting
"@

Write-Host "  [OK] source-twoway-left / source-twoway-right" -ForegroundColor Green

# ── Version history test tree ─────────────────────────────────
New-Dir "$Root\dest-versions"

New-File "$Root\source-versions\evolving-doc.txt" @"
Evolving Document — Version 1
==============================
Initial content created for version history testing.
This line will be preserved across all versions.
Status: DRAFT
"@

New-File "$Root\source-versions\stable-file.txt" @"
This file never changes.
It should always show 0 mutations on second run.
"@

Write-Host "  [OK] source-versions" -ForegroundColor Green

# ── Bandwidth limiting test tree ──────────────────────────────
New-Dir "$Root\dest-bandwidth"

# ~10 MB file — large enough to observe transfer rate in the UI
New-BinaryFile "$Root\source-bandwidth\transfer-test-10mb.bin" 10240

Write-Host "  [OK] source-bandwidth" -ForegroundColor Green

# ── Conflict (newest-wins) test setup ────────────────────────
New-Dir "$Root\dest-conflict-newest"

New-File "$Root\source-conflict-newest\shared.txt" @"
Shared file — conflict newest-wins test.
Original content.
"@

# ── Conflict (source-wins) test setup ────────────────────────
New-Dir "$Root\dest-conflict-source"

New-File "$Root\source-conflict-source\shared.txt" @"
Shared file — conflict source-wins test.
Original content.
"@

Write-Host "  [OK] conflict test trees" -ForegroundColor Green

# ── Summary ───────────────────────────────────────────────────
Write-Host ""
Write-Host "Filesystem created successfully." -ForegroundColor Cyan
Write-Host ""
Write-Host "Container paths (use these in the tidemarq UI):" -ForegroundColor Yellow
Write-Host "  /test-data/source-backup       -> one-way backup source"
Write-Host "  /test-data/dest-backup         -> one-way backup destination"
Write-Host "  /test-data/source-mirror       -> one-way mirror source"
Write-Host "  /test-data/dest-mirror         -> one-way mirror destination"
Write-Host "  /test-data/source-twoway-left  -> two-way left side"
Write-Host "  /test-data/source-twoway-right -> two-way right side"
Write-Host "  /test-data/source-versions     -> version history source"
Write-Host "  /test-data/dest-versions       -> version history destination"
Write-Host "  /test-data/source-bandwidth    -> bandwidth limit source"
Write-Host "  /test-data/dest-bandwidth      -> bandwidth limit destination"
Write-Host "  /test-data/source-conflict-newest  -> newest-wins conflict source"
Write-Host "  /test-data/dest-conflict-newest    -> newest-wins conflict destination"
Write-Host "  /test-data/source-conflict-source  -> source-wins conflict source"
Write-Host "  /test-data/dest-conflict-source    -> source-wins conflict destination"
