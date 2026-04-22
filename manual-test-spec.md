# tidemarq — Manual Test Specification

**Scope:** UX / end-to-end manual testing of all implemented features  
**Environment:** Local Docker Compose stack  
**Tester prerequisites:** Web browser, Docker Desktop, PowerShell 5.1+

---

## Table of Contents

1. [Environment Setup](#1-environment-setup)
2. [Test Case Index](#2-test-case-index)
3. [TC-AUTH — Authentication](#3-tc-auth--authentication)
4. [TC-DASH — Dashboard](#4-tc-dash--dashboard)
5. [TC-JOB — Job Creation Wizard](#5-tc-job--job-creation-wizard)
6. [TC-BACKUP — One-Way Backup Sync](#6-tc-backup--one-way-backup-sync)
7. [TC-MIRROR — One-Way Mirror Sync](#7-tc-mirror--one-way-mirror-sync)
8. [TC-TWOWAY — Two-Way Sync](#8-tc-twoway--two-way-sync)
9. [TC-CONFLICT — Conflict Resolution](#9-tc-conflict--conflict-resolution)
10. [TC-PROGRESS — Live Progress & Pause/Resume](#10-tc-progress--live-progress--pauseresume)
11. [TC-QUAR — Quarantine Management](#11-tc-quar--quarantine-management)
12. [TC-VER — Version History](#12-tc-ver--version-history)
13. [TC-AUDIT — Audit Log](#13-tc-audit--audit-log)
14. [TC-SETTINGS — Settings & Retention Impact](#14-tc-settings--settings--retention-impact)
15. [TC-USERS — User Management & Role-Based Access](#15-tc-users--user-management--role-based-access)
16. [TC-MOUNTS — Network Mounts (Optional)](#16-tc-mounts--network-mounts-optional)
17. [TC-UNHAPPY — Unhappy Path Testing](#17-tc-unhappy--unhappy-path-testing)

---

## 1. Environment Setup

### 1.1 Prerequisites

- Docker Desktop running
- PowerShell 5.1 or later
- A modern browser (Chrome or Firefox recommended)
- The tidemarq source checked out

### 1.2 Create the test filesystem

Open PowerShell as Administrator and run:

```powershell
cd F:\Projects\tidemarq
.\scripts\01-setup-test-fs.ps1
```

**Expected result:** Script prints OK for each directory group and lists container paths. `C:\tidemarq-test\` exists with the following top-level subdirectories:

```
source-backup       dest-backup
source-mirror       dest-mirror
source-twoway-left  source-twoway-right
source-versions     dest-versions
source-bandwidth    dest-bandwidth
source-conflict-newest  dest-conflict-newest
source-conflict-source  dest-conflict-source
```

### 1.3 Start the stack with test volume mount

```powershell
cd F:\Projects\tidemarq
docker compose -f docker-compose.yml -f docker-compose.test.yml up --build -d
```

**Expected result:** Container starts. Wait ~10 seconds, then navigate to `https://localhost:8717`. Accept the self-signed certificate warning.

### 1.4 Note the container paths

All paths used in the job wizard refer to paths **inside the container**. The host directory `C:\tidemarq-test` is mounted at `/test-data` inside the container. Use these paths throughout the test:

| Purpose | Container path |
|---------|---------------|
| Backup source | `/test-data/source-backup` |
| Backup destination | `/test-data/dest-backup` |
| Mirror source | `/test-data/source-mirror` |
| Mirror destination | `/test-data/dest-mirror` |
| Two-way left | `/test-data/source-twoway-left` |
| Two-way right | `/test-data/source-twoway-right` |
| Versions source | `/test-data/source-versions` |
| Versions destination | `/test-data/dest-versions` |
| Bandwidth source | `/test-data/source-bandwidth` |
| Bandwidth destination | `/test-data/dest-bandwidth` |
| Newest-wins source | `/test-data/source-conflict-newest` |
| Newest-wins destination | `/test-data/dest-conflict-newest` |
| Source-wins source | `/test-data/source-conflict-source` |
| Source-wins destination | `/test-data/dest-conflict-source` |

### 1.5 Default credentials

| Field | Value |
|-------|-------|
| Username | `admin` |
| Password | (value set in `tidemarq.yaml` or the startup log) |

---

## 2. Test Case Index

| ID | Area | Description |
|----|------|-------------|
| TC-AUTH-001 | Auth | Successful login |
| TC-AUTH-002 | Auth | Invalid credentials |
| TC-AUTH-003 | Auth | Empty field validation |
| TC-AUTH-004 | Auth | Session persistence across page refresh |
| TC-AUTH-005 | Auth | Logout |
| TC-DASH-001 | Dashboard | Initial state display |
| TC-DASH-002 | Dashboard | Live job progress on dashboard |
| TC-DASH-003 | Dashboard | Pending review counts update |
| TC-JOB-001 | Job wizard | Name & source validation |
| TC-JOB-002 | Job wizard | Path browser navigation |
| TC-JOB-003 | Job wizard | Mode selection cards |
| TC-JOB-004 | Job wizard | Conflict strategy conditional display |
| TC-JOB-005 | Job wizard | Cron schedule field |
| TC-JOB-006 | Job wizard | Bandwidth limit field |
| TC-JOB-007 | Job wizard | Advanced settings (hash algo, delta, full checksum) |
| TC-JOB-008 | Job wizard | Review step summary |
| TC-JOB-009 | Job wizard | Edit existing job |
| TC-JOB-010 | Job wizard | Delete job with confirmation |
| TC-BACKUP-001 | Backup | First run syncs all files |
| TC-BACKUP-002 | Backup | Second run is idempotent |
| TC-BACKUP-003 | Backup | Source deletion NOT propagated |
| TC-BACKUP-004 | Backup | New source file syncs on next run |
| TC-MIRROR-001 | Mirror | First run syncs all files |
| TC-MIRROR-002 | Mirror | Second run is idempotent |
| TC-MIRROR-003 | Mirror | Source deletion quarantines destination file |
| TC-MIRROR-004 | Mirror | New source file syncs on next run |
| TC-MIRROR-005 | Mirror | Quarantine entry shows correct file details |
| TC-TWOWAY-001 | Two-way | First run syncs both sides |
| TC-TWOWAY-002 | Two-way | One-sided change propagates cleanly |
| TC-TWOWAY-003 | Two-way | Both-side change detected as conflict |
| TC-TWOWAY-004 | Two-way | Ask-user conflict queued in Conflicts view |
| TC-TWOWAY-005 | Two-way | Resolve: keep source |
| TC-TWOWAY-006 | Two-way | Resolve: keep destination |
| TC-TWOWAY-007 | Two-way | Resolve: keep both (.conflict file created) |
| TC-CONFLICT-001 | Conflict | Newest-wins auto-resolution |
| TC-CONFLICT-002 | Conflict | Source-wins auto-resolution |
| TC-CONFLICT-003 | Conflict | Conflicts view filter by job |
| TC-CONFLICT-004 | Conflict | Resolved conflicts table |
| TC-CONFLICT-005 | Conflict | Clear resolved conflicts |
| TC-PROGRESS-001 | Progress | Progress bar visible during run |
| TC-PROGRESS-002 | Progress | Transfer rate and ETA displayed |
| TC-PROGRESS-003 | Progress | Activity log scrolls with file names |
| TC-PROGRESS-004 | Progress | Pause mid-transfer |
| TC-PROGRESS-005 | Progress | Resume after pause |
| TC-QUAR-001 | Quarantine | Quarantined files listed |
| TC-QUAR-002 | Quarantine | Restore from quarantine |
| TC-QUAR-003 | Quarantine | Permanent delete from quarantine |
| TC-QUAR-004 | Quarantine | Retention impact: short retention expires entries |
| TC-QUAR-005 | Quarantine | Retention impact: expired entries move to "recently removed" |
| TC-QUAR-006 | Quarantine | Clear removed entries |
| TC-VER-001 | Versions | Version created on overwrite |
| TC-VER-002 | Versions | Multiple versions stack correctly |
| TC-VER-003 | Versions | Restore previous version |
| TC-VER-004 | Versions | Keep count limit enforced |
| TC-VER-005 | Versions | Keep count = 0 means unlimited |
| TC-AUDIT-001 | Audit | Events logged after job runs |
| TC-AUDIT-002 | Audit | Filter by job |
| TC-AUDIT-003 | Audit | Filter by event type |
| TC-AUDIT-004 | Audit | Export CSV |
| TC-AUDIT-005 | Audit | Export JSON |
| TC-AUDIT-006 | Audit | Retention impact: old entries purged |
| TC-SETTINGS-001 | Settings | Dark/light mode toggle |
| TC-SETTINGS-002 | Settings | OS theme respected on first load |
| TC-SETTINGS-003 | Settings | Default admin password banner |
| TC-SETTINGS-004 | Settings | Save version history count |
| TC-SETTINGS-005 | Settings | Save soft-delete retention |
| TC-SETTINGS-006 | Settings | Save audit log retention |
| TC-SETTINGS-007 | Settings | About tab shows version and uptime |
| TC-USERS-001 | Users | Admin sees Users tab |
| TC-USERS-002 | Users | Create Operator user |
| TC-USERS-003 | Users | Create Viewer user |
| TC-USERS-004 | Users | Viewer cannot run jobs |
| TC-USERS-005 | Users | Viewer cannot create jobs |
| TC-USERS-006 | Users | Operator can run and create jobs |
| TC-USERS-007 | Users | Operator cannot manage users |
| TC-USERS-008 | Users | Edit user role |
| TC-USERS-009 | Users | Change user password |
| TC-USERS-010 | Users | Delete user |
| TC-MOUNTS-001 | Mounts | Create SFTP mount |
| TC-MOUNTS-002 | Mounts | Test SFTP connectivity |
| TC-MOUNTS-003 | Mounts | Create SMB mount |
| TC-MOUNTS-004 | Mounts | Test SMB connectivity |
| TC-MOUNTS-005 | Mounts | Delete mount shows warning |
| TC-MOUNTS-006 | Mounts | Mount path available in job path picker |
| TC-UNHAPPY-001 | Unhappy | Login with wrong password |
| TC-UNHAPPY-002 | Unhappy | Create job with duplicate name |
| TC-UNHAPPY-003 | Unhappy | Create job with non-existent source path |
| TC-UNHAPPY-004 | Unhappy | Bandwidth limit: non-numeric input |
| TC-UNHAPPY-005 | Unhappy | Invalid cron expression |
| TC-UNHAPPY-006 | Unhappy | Run already-running job |
| TC-UNHAPPY-007 | Unhappy | Delete a running job |
| TC-UNHAPPY-008 | Unhappy | Viewer attempts restricted action |
| TC-UNHAPPY-009 | Unhappy | Delete own admin account |
| TC-UNHAPPY-010 | Unhappy | Mount connectivity test failure |
| TC-UNHAPPY-011 | Unhappy | Audit export as non-admin |
| TC-UNHAPPY-012 | Unhappy | Settings save as Viewer |
| TC-UNHAPPY-013 | Unhappy | Job with same source and destination paths |

---

## 3. TC-AUTH — Authentication

### TC-AUTH-001 — Successful login

**Steps:**
1. Navigate to `https://localhost:8717`
2. Accept the self-signed TLS certificate warning
3. Enter the correct admin username and password
4. Click **Sign in**

**Pass:** Redirected to the Dashboard. The tidemarq wordmark and logo appear in the sidebar. No error message visible. The URL is `/` or `/dashboard`.

---

### TC-AUTH-002 — Invalid credentials

**Steps:**
1. Navigate to the login page (log out first if already signed in)
2. Enter username `admin` and password `wrongpassword`
3. Click **Sign in**

**Pass:** An error message reading "Invalid username or password" appears below the form. The page remains on the login view. The password field is not cleared automatically (usability: tester can correct the password without re-typing).

---

### TC-AUTH-003 — Empty field validation

**Steps:**
1. On the login page, leave both fields blank and click **Sign in**
2. Then try with username filled but password blank
3. Then try with password filled but username blank

**Pass:** In all three cases the form does not submit and the browser (or UI) indicates the required fields. No network request fires for blank submissions. No error message from the server appears (validation is client-side).

---

### TC-AUTH-004 — Session persistence across page refresh

**Steps:**
1. Log in successfully
2. Navigate to the Jobs view
3. Press **F5** to hard-refresh the browser
4. Observe the page that loads

**Pass:** The Jobs view reloads without redirecting to the login page. The user remains authenticated. No "401 Unauthorized" error appears.

---

### TC-AUTH-005 — Logout

**Steps:**
1. Log in and navigate to any view
2. Locate the logout control (sidebar or topbar)
3. Click logout

**Pass:** Redirected to the login page. Pressing the browser back button and then visiting a protected page (e.g. `/jobs`) redirects back to the login page — the session token is no longer valid client-side.

---

## 4. TC-DASH — Dashboard

### TC-DASH-001 — Initial state display

**Pre-condition:** No jobs created yet.

**Steps:**
1. Log in and land on the Dashboard

**Pass:**
- Four stat cards visible: **Total Jobs**, **Healthy**, **Errors**, **Pending Review** — all show `0`
- "Currently Running" section shows an empty state or is not shown
- "Pending Review" section shows an empty state or is not shown
- "All Jobs" table shows an empty state message (not an error)

---

### TC-DASH-002 — Live job progress on dashboard

**Pre-condition:** At least one job exists that is currently running.

**Steps:**
1. Trigger a job run (see TC-BACKUP-001 to create one first)
2. Immediately navigate to the Dashboard

**Pass:**
- The running job appears in the "Currently Running" section
- A progress bar shows a non-zero percentage (or 0% at the start)
- **Files Done**, transfer rate, and ETA columns populate with live data
- The progress updates in real-time without a page refresh (WebSocket driven)

---

### TC-DASH-003 — Pending review counts update

**Pre-condition:** A two-way job with `ask-user` conflict strategy has been run and generated a conflict.

**Steps:**
1. Navigate to the Dashboard

**Pass:**
- The **Pending Review** stat card shows a count ≥ 1
- The "Pending Review" section lists the conflict(s) grouped by job
- Clicking "Resolve" on a pending item navigates to the Conflicts view

---

## 5. TC-JOB — Job Creation Wizard

### TC-JOB-001 — Name & source validation

**Steps:**
1. Navigate to **Sync Jobs → New Job** (or the + button)
2. Leave the job name blank and click **Next**
3. Observe the result
4. Enter a name (e.g. `Test Backup`) and leave the source path blank, click **Next**
5. Observe the result

**Pass:**
- Step 1 (name blank): Next is blocked; the name field is highlighted or an error appears beneath it
- Step 1 (path blank): Next is blocked; the source path field is highlighted or an error appears
- At no point does the wizard advance past step 1 with missing required data

---

### TC-JOB-002 — Path browser navigation

**Steps:**
1. In the New Job wizard, Step 1 (Source), click the path browser icon or the path input
2. The path picker opens — navigate to `/test-data`
3. Observe the subdirectories listed
4. Click into `source-backup`
5. Observe the files and subdirectories

**Pass:**
- `/test-data` shows the test subdirectories (`source-backup`, `dest-backup`, etc.)
- Navigating into `source-backup` shows `documents/`, `images/`, `media/`, `config.json`
- Clicking a directory navigates into it; a breadcrumb or back control is visible
- Selecting `source-backup` as the path and confirming populates the source field

---

### TC-JOB-003 — Mode selection cards

**Steps:**
1. Advance to Step 3 (Mode) in the wizard
2. Observe the three mode cards
3. Click each one in turn and read the description

**Pass:**
- Three cards: **One-way backup**, **One-way mirror**, **Two-way**
- Each card shows a description of what the mode does (deletions, direction)
- Clicking a card highlights/selects it; previously selected card is de-selected
- Default selection is **One-way backup**

---

### TC-JOB-004 — Conflict strategy conditional display

**Steps:**
1. In Step 3, select **Two-way**
2. Observe whether the conflict strategy selector appears
3. Go back and select **One-way backup**
4. Observe the conflict strategy selector

**Pass:**
- With **Two-way** selected: a conflict strategy dropdown/selector appears below the mode cards
- With **One-way backup** or **One-way mirror** selected: the conflict strategy selector is hidden or disabled
- The five strategies (newest-wins, largest-wins, source-wins, destination-wins, ask-user) are all present in the dropdown when visible

---

### TC-JOB-005 — Cron schedule field

**Steps:**
1. Advance to Step 4 (Schedule & Transfer)
2. Locate the cron schedule input
3. Enter an invalid expression: `99 99 99 99 99`
4. Try to proceed to the next step
5. Clear it and enter a valid expression: `0 2 * * *`

**Pass:**
- Invalid cron: wizard blocks advance or shows an inline error such as "Invalid cron expression"
- Valid cron (`0 2 * * *`): wizard accepts and advances. A human-readable description ("every day at 02:00") is shown if supported
- Leaving the field empty is valid (no scheduled trigger)

---

### TC-JOB-006 — Bandwidth limit field

**Steps:**
1. In Step 4, locate the bandwidth limit input
2. Enter `-100` (negative number)
3. Enter `abc` (non-numeric)
4. Enter `0` (unlimited)
5. Enter `512` (512 KB/s)

**Pass:**
- Negative values: input rejects or shows validation error "must be a positive number"
- Non-numeric input: input rejects or shows error
- `0`: accepted; UI indicates "unlimited"
- `512`: accepted; no error

---

### TC-JOB-007 — Advanced settings

**Steps:**
1. In Step 4, expand the **Advanced Settings** collapsible section
2. Observe available controls
3. Change hash algorithm from BLAKE3 to SHA-256
4. Toggle **Full content verification** on
5. Toggle **Delta transfer** on

**Pass:**
- Collapsible opens without page reload
- Hash algorithm dropdown shows BLAKE3 (default) and SHA-256
- Full content verification toggle changes state visually (on/off indicator)
- Delta transfer toggle changes state visually
- All three settings are reflected in the Step 5 review summary

---

### TC-JOB-008 — Review step summary

**Steps:**
1. Complete Steps 1–4 of the New Job wizard with the following:
   - Name: `Review Test Job`
   - Source: `/test-data/source-backup`
   - Destination: `/test-data/dest-backup`
   - Mode: One-way backup
   - Bandwidth: 512 KB/s
   - Cron: `0 3 * * 1` (weekly Monday)
   - Hash: SHA-256
2. Reach Step 5 (Review)

**Pass:**
- All entered values are visible in the review summary
- Job name, source, destination, mode, bandwidth, cron, and hash algorithm all match what was entered
- A **Create Job** (or **Save**) button is present and enabled
- A **Back** button allows returning to previous steps

---

### TC-JOB-009 — Edit existing job

**Pre-condition:** At least one job exists.

**Steps:**
1. Navigate to **Sync Jobs**
2. Find a job in the list and click its **Edit** (pencil) icon
3. Change the job name
4. Change the bandwidth limit
5. Click through to Save

**Pass:**
- The wizard pre-populates all current job settings
- After saving, the Jobs list shows the updated name and the job detail page shows the updated bandwidth limit
- No new job is created; the same job ID is retained

---

### TC-JOB-010 — Delete job with confirmation

**Pre-condition:** At least one job exists that is not currently running.

**Steps:**
1. Navigate to **Sync Jobs**
2. Find a job and click its **Delete** (trash) icon
3. Read the confirmation modal
4. Click **Cancel**
5. Verify job still exists
6. Click **Delete** again, then confirm deletion

**Pass:**
- A modal dialog appears asking for confirmation before deletion
- Cancelling does not delete the job
- Confirming removes the job from the list immediately
- Navigating to the (now-deleted) job's URL shows a not-found state or redirects

---

## 6. TC-BACKUP — One-Way Backup Sync

### Setup — Create the backup job

1. Navigate to **Sync Jobs → New Job**
2. Complete the wizard:
   - **Name:** `Backup Test`
   - **Source:** `/test-data/source-backup`
   - **Destination:** `/test-data/dest-backup`
   - **Mode:** One-way backup
   - **Watch:** Off (for manual control during testing)
   - **Cron:** (leave blank)
   - **Bandwidth:** 0 (unlimited)
3. Save the job

---

### TC-BACKUP-001 — First run syncs all files

**Steps:**
1. On the Jobs list or Job Detail, click **Run now** for `Backup Test`
2. Wait for the job to reach status **Idle** (completed)

**Pass:**
- Job status transitions: `idle → running → idle`
- On the host, `C:\tidemarq-test\dest-backup\` now contains the same directory structure as `source-backup`:
  - `documents\report-q4.txt` ✓
  - `documents\meeting-notes.txt` ✓
  - `documents\archive\old-report-q3.txt` ✓
  - `documents\archive\legacy-data.csv` ✓
  - `images\photo-caption-001.txt` ✓
  - `images\photo-caption-002.txt` ✓
  - `images\photo-caption-003.txt` ✓
  - `images\raw\shoot-notes.txt` ✓
  - `config.json` ✓
  - `media\sample-5mb.bin` ✓
- Job Detail shows non-zero **Files Done** count and **Data Moved**

---

### TC-BACKUP-002 — Second run is idempotent

**Steps:**
1. Click **Run now** for `Backup Test` again (immediately, without changing any files)
2. Wait for completion

**Pass:**
- Job completes with status **Idle**
- Job Detail shows **Files Done** = 0 (or all files shown as "Unchanged")
- **Data Moved** = 0 bytes
- No errors in the audit log for this run
- The destination directory has not changed (timestamps on existing files are not bumped)

---

### TC-BACKUP-003 — Source deletion NOT propagated

**Steps:**
1. On the host, delete `C:\tidemarq-test\source-backup\images\photo-caption-003.txt`
2. Run the `Backup Test` job
3. Wait for completion
4. Check the destination on the host

**Pass:**
- `C:\tidemarq-test\dest-backup\images\photo-caption-003.txt` still exists — it was NOT deleted
- Job completes cleanly with no errors
- This is the defining behaviour of **one-way backup** mode: source deletions do not propagate

**Restore the deleted file before continuing:**
```powershell
Set-Content "C:\tidemarq-test\source-backup\images\photo-caption-003.txt" "Product launch — September 2024"
```

---

### TC-BACKUP-004 — New source file syncs on next run

**Steps:**
1. Create a new file on the host:
   ```powershell
   Set-Content "C:\tidemarq-test\source-backup\new-file.txt" "Newly added file for backup test"
   ```
2. Run the `Backup Test` job and wait for completion

**Pass:**
- `C:\tidemarq-test\dest-backup\new-file.txt` exists with content "Newly added file for backup test"
- Job Detail shows Files Done = 1 (only the new file was copied)

---

## 7. TC-MIRROR — One-Way Mirror Sync

### Setup — Create the mirror job

1. Navigate to **Sync Jobs → New Job**
2. Complete the wizard:
   - **Name:** `Mirror Test`
   - **Source:** `/test-data/source-mirror`
   - **Destination:** `/test-data/dest-mirror`
   - **Mode:** One-way mirror
   - **Watch:** Off
   - **Cron:** (leave blank)
3. Save the job

---

### TC-MIRROR-001 — First run syncs all files

**Steps:**
1. Click **Run now** for `Mirror Test`
2. Wait for completion

**Pass:**
- All source files appear in `C:\tidemarq-test\dest-mirror\`:
  - `active\project-alpha.txt`, `project-beta.txt`, `project-gamma.txt` ✓
  - `shared\readme.md`, `shared\config.ini` ✓
- Job status returns to **Idle**, no errors

---

### TC-MIRROR-002 — Second run is idempotent

**Steps:**
1. Run `Mirror Test` again immediately

**Pass:**
- Files Done = 0, Data Moved = 0 bytes
- No files changed or re-copied in the destination

---

### TC-MIRROR-003 — Source deletion quarantines destination file

**Steps:**
1. Run the setup script to delete two files from source and add one new file:
   ```powershell
   .\scripts\02-trigger-mirror-deletions.ps1
   ```
2. Run the `Mirror Test` job
3. Wait for completion
4. Navigate to **Quarantine** in the sidebar

**Pass:**
- In the destination, `active\project-gamma.txt` and `shared\config.ini` do NOT appear as regular files (they have been moved to quarantine, not hard-deleted)
- In the destination, `active\project-delta.txt` NOW exists (new file synced in)
- The **Quarantine** view lists two entries under the `Mirror Test` job:
  - `active/project-gamma.txt`
  - `shared/config.ini`
- Each entry shows a file size, the job name, and an expiry countdown

---

### TC-MIRROR-004 — New source file syncs on next run

**Pass** (verified in TC-MIRROR-003): `active/project-delta.txt` appeared in the destination after the source was updated.

---

### TC-MIRROR-005 — Quarantine entry shows correct file details

**Steps:**
1. Navigate to **Quarantine**
2. Expand or review the `Mirror Test` quarantine entries

**Pass:**
- Each entry shows: file path (in monospace font), file size, the job that quarantined it, and the date quarantined
- An expiry indicator shows days remaining before permanent deletion (based on the soft-delete retention setting)
- **Restore** and **Delete** action buttons are present per entry

---

## 8. TC-TWOWAY — Two-Way Sync

### Setup — Create the two-way job

1. Navigate to **Sync Jobs → New Job**
2. Complete the wizard:
   - **Name:** `Two-Way Ask-User`
   - **Source:** `/test-data/source-twoway-left`
   - **Destination:** `/test-data/source-twoway-right`
   - **Mode:** Two-way
   - **Conflict strategy:** Ask user
   - **Watch:** Off
   - **Cron:** (leave blank)
3. Save the job

---

### TC-TWOWAY-001 — First run syncs both sides

**Steps:**
1. Run `Two-Way Ask-User` and wait for completion

**Pass:**
- `left-only.txt` now exists in `source-twoway-right` (propagated left → right)
- `right-only.txt` now exists in `source-twoway-left` (propagated right → left)
- `shared-doc.txt` is identical on both sides
- `reference\manual.txt` is identical on both sides
- No conflicts reported (files were identical at start)

---

### TC-TWOWAY-002 — One-sided change propagates cleanly

**Steps:**
1. Create a new file only on the left side:
   ```powershell
   Set-Content "C:\tidemarq-test\source-twoway-left\new-from-left.txt" "Created only on left side"
   ```
2. Run `Two-Way Ask-User` and wait for completion

**Pass:**
- `new-from-left.txt` now exists in `source-twoway-right`
- No conflicts generated
- Job completes with status **Idle**, no errors

---

### TC-TWOWAY-003 — Both-side change detected as conflict

**Steps:**
1. Run the conflict creation script:
   ```powershell
   .\scripts\03-create-conflicts.ps1
   ```
2. Run `Two-Way Ask-User` and wait for completion

**Pass:**
- Job completes with status **Idle** (not **Error**)
- The **Conflicts** badge in the sidebar shows a count ≥ 1
- `shared-doc.txt` conflict appears in the Conflicts view
- `reference\manual.txt` does NOT appear in the Conflicts view (one-sided change — propagated cleanly)

---

### TC-TWOWAY-004 — Ask-user conflict queued in Conflicts view

**Steps:**
1. Navigate to **Conflicts**
2. Find the conflict for `Two-Way Ask-User` / `shared-doc.txt`

**Pass:**
- The conflict shows:
  - File path in monospace font
  - Last-modified timestamps for each side
  - File sizes for each side
- Four resolution options are visible: **Keep Source**, **Keep Destination**, **Keep Both**, **Discard Both** (or similar labelling)
- The conflict status is "Pending"

---

### TC-TWOWAY-005 — Resolve: keep source

**Steps:**
1. On the `shared-doc.txt` conflict, click **Keep Source** (or equivalent)
2. Wait for the UI to update
3. Check the file on both sides on the host

**Pass:**
- The conflict disappears from the Pending list (moves to Resolved)
- Both `C:\tidemarq-test\source-twoway-left\shared-doc.txt` and `C:\tidemarq-test\source-twoway-right\shared-doc.txt` contain the LEFT side content ("Last editor: LEFT side")
- An entry appears in the Audit Log for this resolution event

> After this test, restore both sides to a neutral state before running further conflict tests:
> ```powershell
> $c = "Shared file restored to neutral state."
> Set-Content "C:\tidemarq-test\source-twoway-left\shared-doc.txt" $c
> Set-Content "C:\tidemarq-test\source-twoway-right\shared-doc.txt" $c
> ```
> Then run `Two-Way Ask-User` once to re-sync.

---

### TC-TWOWAY-006 — Resolve: keep destination

**Steps:**
1. Recreate a conflict:
   ```powershell
   .\scripts\03-create-conflicts.ps1
   ```
2. Run `Two-Way Ask-User` and wait for the conflict to appear
3. On the new `shared-doc.txt` conflict, click **Keep Destination**

**Pass:**
- Both sides now contain the RIGHT side content ("Last editor: RIGHT side")
- Conflict marked as Resolved

---

### TC-TWOWAY-007 — Resolve: keep both (.conflict file created)

**Steps:**
1. Recreate a conflict using the script again (restore neutral state first as above)
2. Run the job, then click **Keep Both** on the conflict

**Pass:**
- The original file on one side retains its content
- A new file with a `.conflict.<timestamp>` suffix appears alongside it (e.g. `shared-doc.txt.conflict.20241110-142533`)
- Both files are visible in the destination directory on the host
- No data is lost

---

## 9. TC-CONFLICT — Conflict Resolution

### Setup — Create two additional two-way jobs with auto-resolution strategies

#### Newest-wins job
1. New Job:
   - **Name:** `Two-Way Newest-Wins`
   - **Source:** `/test-data/source-conflict-newest`
   - **Destination:** `/test-data/dest-conflict-newest`
   - **Mode:** Two-way
   - **Conflict strategy:** Newest wins
2. Run the job once (first sync, no conflict yet)

#### Source-wins job
1. New Job:
   - **Name:** `Two-Way Source-Wins`
   - **Source:** `/test-data/source-conflict-source`
   - **Destination:** `/test-data/dest-conflict-source`
   - **Mode:** Two-way
   - **Conflict strategy:** Source wins
2. Run the job once (first sync, no conflict yet)

---

### TC-CONFLICT-001 — Newest-wins auto-resolution

**Steps:**
1. Run the newest-wins conflict setup:
   ```powershell
   .\scripts\05-setup-conflict-newest-wins.ps1
   ```
2. Run the `Two-Way Newest-Wins` job and wait for completion

**Pass:**
- No conflict appears in the Conflicts view (auto-resolved)
- Both `source-conflict-newest\shared.txt` and `dest-conflict-newest\shared.txt` contain the RIGHT content ("MODIFIED ON RIGHT — this is the NEWER edit")
- Audit log contains an entry for the auto-resolved conflict, listing the strategy used

---

### TC-CONFLICT-002 — Source-wins auto-resolution

**Steps:**
1. Modify both sides of the source-wins tree:
   ```powershell
   Set-Content "C:\tidemarq-test\source-conflict-source\shared.txt" "SOURCE CONTENT — this should win"
   Start-Sleep -Seconds 2
   Set-Content "C:\tidemarq-test\dest-conflict-source\shared.txt" "DESTINATION CONTENT — this should lose"
   ```
2. Run the `Two-Way Source-Wins` job and wait for completion

**Pass:**
- No conflict in the Conflicts view
- Both sides now contain "SOURCE CONTENT — this should win"
- Audit log records the auto-resolution

---

### TC-CONFLICT-003 — Conflicts view filter by job

**Pre-condition:** Multiple pending conflicts exist across multiple jobs.

**Steps:**
1. Navigate to **Conflicts**
2. Use the job filter to select only `Two-Way Ask-User`

**Pass:**
- Only conflicts belonging to `Two-Way Ask-User` are displayed
- Conflicts from other jobs are hidden
- The filter UI updates the list without a page reload

---

### TC-CONFLICT-004 — Resolved conflicts table

**Steps:**
1. Navigate to **Conflicts**
2. Scroll past the Pending section to the Resolved section (within each job group)

**Pass:**
- Previously resolved conflicts appear in a table with: file path, resolution strategy used, resolved timestamp
- The most recently resolved appear first (or sorted by date)
- Resolved conflicts are visually distinct from pending ones (different styling)

---

### TC-CONFLICT-005 — Clear resolved conflicts

**Steps:**
1. Navigate to **Conflicts**
2. Click **Clear resolved** for a job that has resolved conflicts

**Pass:**
- The resolved conflicts table for that job becomes empty
- Pending conflicts (if any) are unaffected
- The action is confirmed (either with a modal or immediate with a success toast)

---

## 10. TC-PROGRESS — Live Progress & Pause/Resume

> Use the `Backup Test` job (which includes a 5 MB binary file) to ensure the run lasts long enough to observe.

### TC-PROGRESS-001 — Progress bar visible during run

**Steps:**
1. Navigate to the **Job Detail** page for `Backup Test`
2. Delete all files in `C:\tidemarq-test\dest-backup\` to force a full re-transfer
3. Click **Run now**
4. Observe the Job Detail page

**Pass:**
- A progress bar appears showing percentage complete (0%→100%)
- The bar fills visually as files transfer
- The current action label shows "Copying" (or "Scanning" during initial scan)

---

### TC-PROGRESS-002 — Transfer rate and ETA displayed

**Steps:**
1. While the job is running (triggered above), observe the stats row

**Pass:**
- **Transfer Rate** shows a non-zero value (e.g. "12.4 MB/s")
- **ETA** shows a non-zero time remaining (e.g. "3s") or "—" when near completion
- **Data Moved** increments as the run progresses
- **Files Done** increments as each file completes

---

### TC-PROGRESS-003 — Activity log shows current file

**Steps:**
1. While the job is running, observe the Recent Activity section on the Job Detail page

**Pass:**
- The list shows the last ≤10 file operations
- Each entry shows the file path (in monospace) and its action (Copied / Unchanged / Removed)
- The list scrolls as new entries arrive — earlier entries scroll off the top
- File paths are readable (not garbled or truncated badly)

---

### TC-PROGRESS-004 — Pause mid-transfer

**Steps:**
1. Trigger a fresh run of `Backup Test` (clear dest first so it takes a moment)
2. Once the job is running (status = Running), click **Pause** in the Job Detail header
3. Observe the job status

**Pass:**
- Job status changes from **Running** to **Paused**
- The progress bar freezes at its current position
- The **Pause** button is replaced with a **Resume** button
- The activity log stops updating
- Files transferred so far remain in the destination (partial progress is not lost)

---

### TC-PROGRESS-005 — Resume after pause

**Steps:**
1. With the job in **Paused** state (from TC-PROGRESS-004), click **Resume**

**Pass:**
- Job status returns to **Running**
- The progress bar resumes from where it paused (not from 0%)
- Files transferred before pausing are not re-transferred
- The job completes normally to **Idle**

---

## 11. TC-QUAR — Quarantine Management

> Pre-condition: TC-MIRROR-003 has been run and two files are in quarantine.

### TC-QUAR-001 — Quarantined files listed

**Steps:**
1. Navigate to **Quarantine** in the sidebar

**Pass:**
- At least two entries visible under `Mirror Test`: `active/project-gamma.txt` and `shared/config.ini`
- Each entry shows: file path (monospace), file size, job name, quarantine date, days remaining
- A **Restore** and **Delete** button appear per entry

---

### TC-QUAR-002 — Restore from quarantine

**Steps:**
1. Click **Restore** on the `active/project-gamma.txt` quarantine entry
2. Navigate to the destination on the host

**Pass:**
- `C:\tidemarq-test\dest-mirror\active\project-gamma.txt` reappears with its original content
- The quarantine entry for `project-gamma.txt` is removed from the Quarantine view (or moves to "recently removed")
- Running the Mirror job again does NOT re-quarantine the restored file (unless source-mirror still lacks it — which it does after script 02, so the file WILL be re-quarantined on next run; that is expected)

---

### TC-QUAR-003 — Permanent delete from quarantine

**Steps:**
1. Click **Delete** on the `shared/config.ini` quarantine entry
2. Confirm the deletion in the modal

**Pass:**
- The quarantine entry for `shared/config.ini` is permanently removed
- The file does NOT reappear in `dest-mirror\` (it is gone)
- A confirmation modal appeared before the action was taken (no accidental deletion)

---

### TC-QUAR-004 — Retention impact: short retention expires entries

> This test verifies that the soft-delete retention setting actually purges entries, not just configures them.

**Steps:**
1. Navigate to **Settings → General**
2. Set **Soft delete retention** to `1` day and click **Save**
3. In the database or by inspection: note that any quarantine entries older than 1 day should now be eligible for purge
4. To force an observable effect, temporarily set retention to `0` days — if the UI enforces a minimum of 1, use 1 and note that entries from yesterday would be purged on next sweep
5. Alternatively: check if the server performs a sweep on startup or periodically (it does daily) — restart the container with `docker compose restart` and wait
6. Navigate back to **Quarantine**

**Pass:**
- Quarantine entries older than the retention period do not appear in the active quarantine list
- They appear in the "Recently Removed" section instead, with a "Permanently removed" status
- The purge is performed automatically (tester does not manually delete them)

> **Note:** If all quarantine entries are recent (just created), set retention back to `30` days and verify entries remain. Then manually observe the "Removed" section for any that expired.

---

### TC-QUAR-005 — Retention impact: expired entries move to "recently removed"

**Steps:**
1. Navigate to **Quarantine**
2. Scroll to or locate the "Recently Removed" subsection within each job group

**Pass:**
- Any quarantine entries that have expired (past retention) or been manually deleted appear in "Recently Removed"
- Each shows: file path, removal date, why it was removed (expired or manually deleted)
- "Recently Removed" entries cannot be restored (they are gone permanently)

---

### TC-QUAR-006 — Clear removed entries

**Steps:**
1. Navigate to **Quarantine**
2. Click **Clear removed** for a job that has recently-removed entries

**Pass:**
- The "Recently Removed" table for that job clears
- Active quarantine entries (not yet expired) are unaffected
- A success toast appears or the list updates immediately

---

## 12. TC-VER — Version History

### Setup — Create the versions job

1. New Job:
   - **Name:** `Version History Test`
   - **Source:** `/test-data/source-versions`
   - **Destination:** `/test-data/dest-versions`
   - **Mode:** One-way backup
   - **Watch:** Off
2. Save and run the job once (first sync)

---

### TC-VER-001 — Version created on overwrite

**Steps:**
1. First sync has completed (TC-VER setup)
2. Modify the evolving doc:
   ```powershell
   .\scripts\04-update-for-versions.ps1
   ```
3. Run `Version History Test` again
4. Navigate to **Job Detail** for this job and look for versioning information

**Pass:**
- The destination now has the updated `evolving-doc.txt` content (Version 2)
- A version entry for `evolving-doc.txt` is accessible (via the Job Detail or a Versions section)
- The version entry contains the Version 1 content (the previous file before overwrite)
- A **Restore** button is present on the version entry

---

### TC-VER-002 — Multiple versions stack correctly

**Steps:**
1. Run the update script twice more with a job run between each:
   ```powershell
   .\scripts\04-update-for-versions.ps1   # Version 3
   # Run job
   .\scripts\04-update-for-versions.ps1   # Version 4
   # Run job
   ```

**Pass:**
- The destination contains Version 4 content
- Three version entries exist for `evolving-doc.txt`: Versions 1, 2, and 3 (all previous states)
- Each version shows its content timestamp and is individually restorable

---

### TC-VER-003 — Restore previous version

**Steps:**
1. From the Job Detail or version history list, locate Version 2 of `evolving-doc.txt`
2. Click **Restore** on that version entry

**Pass:**
- The destination file `evolving-doc.txt` now contains the Version 2 content
- A success toast or status message confirms the restore
- A new version entry is created for the state before restore (so Version 4 is preserved as a version)
- The original Version 2 entry remains in the list (restore does not consume the version)

---

### TC-VER-004 — Keep count limit enforced

**Steps:**
1. Navigate to **Settings → General**
2. Set **Version history: versions to keep per file** to `2`
3. Click **Save**
4. Run the update script and the job twice more, creating Versions 5 and 6
5. Inspect the version history for `evolving-doc.txt`

**Pass:**
- Only the **2 most recent** versions are retained (e.g. Versions 5 and 6 — or 4 and 5 depending on current state)
- Older versions have been automatically purged
- The destination file contains the latest content
- The purge is silent (no error; old versions simply no longer appear)

---

### TC-VER-005 — Keep count = 0 means unlimited

**Steps:**
1. Navigate to **Settings → General**
2. Set **Version history** to `0` (unlimited)
3. Click **Save**
4. Run the update script and job 3 more times
5. Inspect the version history for `evolving-doc.txt`

**Pass:**
- All versions are retained (count grows without pruning)
- No versions are automatically deleted
- The version list shows all entries accumulated

---

## 13. TC-AUDIT — Audit Log

### TC-AUDIT-001 — Events logged after job runs

**Steps:**
1. Run any job (e.g. `Backup Test`)
2. Navigate to **Audit Log**

**Pass:**
- At least one entry appears for the job run
- Each entry shows: timestamp, event type badge (color-coded), job name, and a human-readable message
- Entries appear in reverse-chronological order (most recent first)

---

### TC-AUDIT-002 — Filter by job

**Steps:**
1. In the Audit Log, use the **Job** dropdown to select `Backup Test`

**Pass:**
- Only entries for `Backup Test` are displayed
- Events from other jobs are hidden
- Filter updates the list without a page reload

---

### TC-AUDIT-003 — Filter by event type

**Steps:**
1. In the Audit Log, use the **Event type** pills to select **Error** only

**Pass:**
- If any error events exist, only those are shown
- Non-error events (Sync, Info) are hidden
- If no errors exist, the list shows an empty state (not a UI error)

---

### TC-AUDIT-004 — Export CSV

**Steps:**
1. In the Audit Log with no filters active (or all jobs selected), click **Export CSV**

**Pass:**
- A file download begins automatically
- The downloaded file has a `.csv` extension
- Opening the file in a spreadsheet shows: columns for timestamp, event type, job name, message
- The data matches what is visible in the UI
- The file is not empty and is properly formatted (commas, quoted strings where needed)

---

### TC-AUDIT-005 — Export JSON

**Steps:**
1. In the Audit Log, click **Export JSON**

**Pass:**
- A file download begins
- The downloaded file has a `.json` extension
- Opening it shows a JSON array of objects, each with fields: timestamp, event_type (or type), job (or job_name), message
- The JSON is valid (no parse errors in browser dev tools or `jq`)
- Content matches the CSV export for the same data

---

### TC-AUDIT-006 — Retention impact: old entries purged

**Steps:**
1. Navigate to **Settings → General**
2. Set **Audit log retention** to `1` day and click **Save**
3. Restart the container: `docker compose restart`
4. Wait ~30 seconds and navigate back to **Audit Log**

**Pass:**
- Audit log entries older than 1 day are no longer visible
- Recent entries (today's runs) still appear
- No error or warning is shown in the UI for the purge (it is silent)

> After this test, reset audit log retention to a reasonable value (e.g. `30` days) to avoid losing audit data for subsequent tests.

---

## 14. TC-SETTINGS — Settings & Retention Impact

### TC-SETTINGS-001 — Dark/light mode toggle

**Steps:**
1. Navigate to **Settings → General**
2. Note the current theme
3. Toggle to the other theme
4. Observe the entire UI

**Pass:**
- Background, sidebar, cards, and text colours all change to match the selected theme
- The toggle's own state updates to reflect the selection
- Navigating away and back retains the chosen theme (persisted, not reset on navigation)

---

### TC-SETTINGS-002 — OS theme respected on first load

**Steps:**
1. Clear browser local storage for the site (DevTools → Application → Local Storage → Clear)
2. Set your OS to dark mode (Windows: Settings → Personalisation → Colours → Dark)
3. Hard-refresh the page

**Pass:**
- tidemarq loads in dark mode automatically (without the tester manually selecting it)
- Repeat with OS set to light mode: tidemarq loads in light mode

---

### TC-SETTINGS-003 — Default admin password banner

> This test is only visible within 5 minutes of first startup with a default password. Run it on a fresh container start if the banner was not previously observed.

**Steps:**
1. Start the container for the first time with no overridden admin password
2. Log in and navigate to **Settings → General**

**Pass:**
- A warning banner appears indicating the default admin password is still in use
- The banner prompts the tester to change the password
- After changing the admin password (via Users tab), the banner is no longer shown on next page load

---

### TC-SETTINGS-004 — Save version history count

**Steps:**
1. Navigate to **Settings → General**
2. Change **Version history: versions to keep per file** from its current value to `5`
3. Click **Save**
4. Refresh the page and return to Settings

**Pass:**
- A success toast appears after saving
- After refresh, the field still shows `5` (setting is persisted)
- The setting affects version pruning as verified in TC-VER-004

---

### TC-SETTINGS-005 — Save soft-delete retention

**Steps:**
1. Navigate to **Settings → General**
2. Change **Soft delete retention** to `14`
3. Click **Save**
4. Refresh and verify

**Pass:**
- Success toast on save
- Field shows `14` after refresh
- Quarantine entry expiry countdowns reflect the new setting (new quarantine entries show "14 days remaining")

---

### TC-SETTINGS-006 — Save audit log retention

**Steps:**
1. Navigate to **Settings → General**
2. Change **Audit log retention** to `90` days
3. Click **Save**
4. Refresh and verify

**Pass:**
- Success toast on save
- Field shows `90` after refresh

---

### TC-SETTINGS-007 — About tab shows version and uptime

**Steps:**
1. Navigate to **Settings → About**

**Pass:**
- **Version** field shows a version string (e.g. `1.0.0` or a git hash)
- **Database** field shows `SQLite`
- **Uptime** field shows a running count (e.g. "2h 14m" or similar) that is non-zero
- None of these fields show errors or placeholder text

---

## 15. TC-USERS — User Management & Role-Based Access

### TC-USERS-001 — Admin sees Users tab

**Steps:**
1. Log in as admin
2. Navigate to **Settings**

**Pass:**
- A **Users** tab is visible and clickable
- The Users tab shows a table with at least the admin user listed

---

### TC-USERS-002 — Create Operator user

**Steps:**
1. In **Settings → Users**, click **Add User** (or equivalent)
2. Enter:
   - Username: `operator1`
   - Password: `TestPass123!`
   - Role: Operator
3. Click **Add**

**Pass:**
- `operator1` appears in the Users table with role "Operator"
- A success toast or inline confirmation appears
- The new user row shows username, role, and created date

---

### TC-USERS-003 — Create Viewer user

**Steps:**
1. Add another user:
   - Username: `viewer1`
   - Password: `TestPass123!`
   - Role: Viewer
2. Click **Add**

**Pass:**
- `viewer1` appears in the Users table with role "Viewer"

---

### TC-USERS-004 — Viewer cannot run jobs

**Steps:**
1. Log out (as admin)
2. Log in as `viewer1` / `TestPass123!`
3. Navigate to **Sync Jobs**
4. Find any job and attempt to click **Run now**

**Pass:**
- The **Run now** button is either absent, disabled, or clicking it produces a "Permission denied" / "Unauthorised" message
- The job does NOT start running (status remains Idle)
- The Viewer is not prompted for confirmation — the action is simply unavailable

---

### TC-USERS-005 — Viewer cannot create jobs

**Steps:**
1. Still logged in as `viewer1`
2. Navigate to **Sync Jobs**
3. Attempt to find the **New Job** button or navigate to `/jobs/new`

**Pass:**
- The **New Job** button is absent from the Jobs view
- Navigating directly to the new-job URL either redirects or shows an access denied message
- No job creation form is presented

---

### TC-USERS-006 — Operator can run and create jobs

**Steps:**
1. Log out, log in as `operator1` / `TestPass123!`
2. Navigate to **Sync Jobs**
3. Click **Run now** on an existing job
4. Navigate to **New Job** and begin filling in the wizard

**Pass:**
- **Run now** triggers the job (status moves to Running)
- The **New Job** wizard loads and is fully functional
- The job can be saved and appears in the list

---

### TC-USERS-007 — Operator cannot manage users

**Steps:**
1. Still logged in as `operator1`
2. Navigate to **Settings**

**Pass:**
- The **Users** tab is either absent from the Settings view or, if visible, contains a message like "Admin access required"
- The operator cannot see other users' details, add new users, or delete users
- The General tab (theme, retention) should still be viewable

---

### TC-USERS-008 — Edit user role

**Steps:**
1. Log in as admin
2. Navigate to **Settings → Users**
3. Click **Edit** on `operator1`
4. Change the role from Operator to Viewer
5. Save

**Pass:**
- The Users table shows `operator1` with role "Viewer"
- If `operator1` is currently logged in and attempts a restricted action (e.g. create a job), they receive an access-denied response

---

### TC-USERS-009 — Change user password

**Steps:**
1. In **Settings → Users**, click **Edit** on `viewer1`
2. Enter a new password: `NewPass456!`
3. Leave the role unchanged
4. Save

**Pass:**
- Success toast appears
- Log out, then log in as `viewer1` with the NEW password `NewPass456!` — succeeds
- Attempting to log in with the OLD password `TestPass123!` fails with "Invalid username or password"

---

### TC-USERS-010 — Delete user

**Steps:**
1. Log in as admin
2. In **Settings → Users**, click **Delete** on `viewer1`
3. Confirm the deletion modal

**Pass:**
- `viewer1` no longer appears in the Users table
- Attempting to log in as `viewer1` with any password fails
- A confirmation modal was shown before deletion

---

## 16. TC-MOUNTS — Network Mounts (Optional)

> These tests require an accessible SFTP or SMB server. Skip if not available in the test environment. Mark as N/A rather than Fail if infrastructure is absent.

### TC-MOUNTS-001 — Create SFTP mount

**Steps:**
1. Navigate to **Mounts**
2. Click **Add Mount**
3. Fill in:
   - Name: `Test SFTP`
   - Type: SFTP
   - Host: `<sftp-server-hostname>`
   - Port: 22
   - Username: `<sftp-user>`
   - Password: `<sftp-password>`
4. Click **Save**

**Pass:**
- The mount appears in the Mounts list with type "SFTP"
- Name, host, and username are displayed
- Password is NOT shown (credentials are stored encrypted, not echoed)

---

### TC-MOUNTS-002 — Test SFTP connectivity

**Steps:**
1. On the `Test SFTP` mount card, click **Test**

**Pass:**
- A green checkmark or "Connected" indicator appears within a few seconds
- If the server is unreachable, a red indicator and error message appear instead (test failure is reported, not a UI crash)

---

### TC-MOUNTS-003 — Create SMB mount

**Steps:**
1. Click **Add Mount**
2. Fill in:
   - Name: `Test SMB`
   - Type: SMB
   - Host: `<smb-server-hostname>`
   - Port: 445
   - Share: `<share-name>`
   - Username: `<smb-user>`
   - Password: `<smb-password>`
3. Save

**Pass:**
- Mount appears in the list with type "SMB" and the share name displayed

---

### TC-MOUNTS-004 — Test SMB connectivity

**Steps:**
1. Click **Test** on the `Test SMB` mount

**Pass:**
- Green checkmark if reachable, red indicator with error message if not

---

### TC-MOUNTS-005 — Delete mount shows warning

**Steps:**
1. Create a job that uses `Test SFTP` as its source (any destination)
2. Navigate to **Mounts** and click **Delete** on `Test SFTP`
3. Read the confirmation modal

**Pass:**
- The confirmation modal warns that deleting this mount will break any jobs that reference it
- Cancelling does not delete the mount
- Confirming deletes the mount and the mount disappears from the list

---

### TC-MOUNTS-006 — Mount path available in job path picker

**Steps:**
1. Navigate to **New Job**
2. In the source path picker, look for a mount section or a dropdown to switch between local filesystem and mounts
3. Select `Test SMB` as the mount type

**Pass:**
- The path picker shows the mount's directory structure (browsable)
- Selecting a directory within the mount populates the source path
- The path or mount selection is reflected in the Review step of the wizard

---

## 17. TC-UNHAPPY — Unhappy Path Testing

### TC-UNHAPPY-001 — Login with wrong password (covered in TC-AUTH-002)

Reference TC-AUTH-002.

---

### TC-UNHAPPY-002 — Create job with duplicate name

**Steps:**
1. Note an existing job name (e.g. `Backup Test`)
2. Navigate to **New Job**
3. Enter the same name: `Backup Test`
4. Complete all other fields with valid values
5. Click **Create Job**

**Pass:**
- The UI shows an error message: the name is already in use (or "A job with this name already exists")
- The wizard does not navigate away; the tester can correct the name
- No duplicate job is created

---

### TC-UNHAPPY-003 — Create job with non-existent source path

**Steps:**
1. Navigate to **New Job**
2. Manually type `/test-data/this-does-not-exist` in the source path field (bypass the browser)
3. Complete remaining fields with valid values
4. Click **Create Job**

**Pass:**
- Either: the wizard validates the path and shows an error before saving
- Or: the job is saved but the first run immediately fails with a clear error message ("Source path does not exist" or similar) visible in Job Detail
- In neither case does the job appear to run successfully with a non-existent source

---

### TC-UNHAPPY-004 — Bandwidth limit: non-numeric input

**Steps:**
1. In the New Job wizard, Step 4, type `fast` in the bandwidth limit field
2. Attempt to advance to the next step

**Pass:**
- The step does not advance; an error appears ("Bandwidth limit must be a number")
- No network request fires with the invalid value

---

### TC-UNHAPPY-005 — Invalid cron expression

**Steps:**
1. In the cron schedule field, enter `* * * *` (only 4 fields, one short)
2. Attempt to advance

**Pass:**
- Error shown: "Invalid cron expression" or equivalent
- The wizard does not advance

---

### TC-UNHAPPY-006 — Run already-running job

**Steps:**
1. Trigger a long-running job (e.g. `Backup Test` with dest cleared)
2. While it is running (status = Running), click **Run now** again

**Pass:**
- The **Run now** button is disabled, absent, or clicking it shows a message like "Job is already running"
- A second concurrent run is NOT started

---

### TC-UNHAPPY-007 — Delete a running job

**Steps:**
1. While `Backup Test` is running, click **Delete** on it
2. Confirm the deletion modal

**Pass:**
- Either: deletion is blocked while the job is running ("Cannot delete a running job — stop it first")
- Or: the job is cancelled and then deleted — in this case the deletion completes but the running operation is stopped first
- In neither case does a "zombie" job process continue running in the background

---

### TC-UNHAPPY-008 — Viewer attempts restricted action

**Steps:**
1. Log in as `viewer1`
2. Using browser developer tools (Network tab), send a manual POST to `/api/v1/jobs/<id>/run`

**Pass:**
- The API returns HTTP 403 Forbidden
- The response body contains a JSON error: `{ "error": "...", "code": "..." }`
- No side effect occurs (the job does not run)

---

### TC-UNHAPPY-009 — Delete own admin account

**Steps:**
1. Log in as admin
2. Navigate to **Settings → Users**
3. Attempt to click **Delete** on the `admin` row (your own account)

**Pass:**
- Either: the **Delete** button is absent/disabled for the currently logged-in user's own account
- Or: clicking it shows an error "You cannot delete your own account"
- The admin account is NOT deleted

---

### TC-UNHAPPY-010 — Mount connectivity test failure

**Steps:**
1. Create an SMB or SFTP mount with a deliberately wrong host (e.g. `192.0.2.1`, an unroutable address, port 9999)
2. Click **Test** on the mount

**Pass:**
- The test result shows a red indicator and an error message (e.g. "Connection refused" or "Network unreachable")
- The UI does not hang or crash; the error appears within a reasonable timeout (≤30 seconds)
- The mount itself is not deleted by the failed test

---

### TC-UNHAPPY-011 — Audit export as non-admin (Operator)

**Steps:**
1. Log in as `operator1` (restore to Operator role first if changed in TC-USERS-008)
2. Navigate to **Audit Log**
3. Attempt to click **Export CSV** or **Export JSON**

**Pass:**
- Either: the Export buttons are not shown to Operator/Viewer roles
- Or: clicking them shows "Admin access required" or the download returns a 403 error
- No file is downloaded

---

### TC-UNHAPPY-012 — Settings save as Viewer

**Steps:**
1. Log in as `viewer1`
2. Navigate to **Settings → General**
3. Attempt to change the theme or a retention setting and click **Save**

**Pass:**
- Either: the Save button is absent or disabled for Viewer
- Or: clicking Save shows an error "Insufficient permissions"
- The setting is not actually changed on the server (verify by logging back in as admin and confirming the value is unchanged)

---

### TC-UNHAPPY-013 — Job with same source and destination paths

**Steps:**
1. Navigate to **New Job**
2. Set source to `/test-data/source-backup`
3. Set destination to `/test-data/source-backup` (same path)
4. Complete other fields and click **Create Job**

**Pass:**
- The wizard shows an error: "Source and destination paths cannot be the same"
- The job is not created
- No data loss occurs (the engine does not overwrite/delete files within the same directory)

---

## Appendix A — Reset Cheat Sheet

Use these commands to restore test directories to a clean state between test runs:

```powershell
# Clear destinations (force a full re-sync next run)
Remove-Item -Recurse -Force "C:\tidemarq-test\dest-backup\*"
Remove-Item -Recurse -Force "C:\tidemarq-test\dest-mirror\*"
Remove-Item -Recurse -Force "C:\tidemarq-test\dest-versions\*"
Remove-Item -Recurse -Force "C:\tidemarq-test\dest-bandwidth\*"
Remove-Item -Recurse -Force "C:\tidemarq-test\dest-conflict-newest\*"
Remove-Item -Recurse -Force "C:\tidemarq-test\dest-conflict-source\*"

# Reset two-way sources to neutral state
$neutral = "Shared file — neutral state. Restored for next test."
Set-Content "C:\tidemarq-test\source-twoway-left\shared-doc.txt"   $neutral
Set-Content "C:\tidemarq-test\source-twoway-right\shared-doc.txt"  $neutral
Set-Content "C:\tidemarq-test\source-conflict-newest\shared.txt"   $neutral
Set-Content "C:\tidemarq-test\dest-conflict-newest\shared.txt"     $neutral
Set-Content "C:\tidemarq-test\source-conflict-source\shared.txt"   $neutral
Set-Content "C:\tidemarq-test\dest-conflict-source\shared.txt"     $neutral

# Restore deleted mirror source files
Set-Content "C:\tidemarq-test\source-mirror\active\project-gamma.txt" "Project Gamma — restored"
Set-Content "C:\tidemarq-test\source-mirror\shared\config.ini" "[restored]`nhost=localhost"

# Re-run full setup if needed
.\scripts\01-setup-test-fs.ps1
```

---

## Appendix B — Pass/Fail Scoring

| Result | Meaning |
|--------|---------|
| **PASS** | All stated outcomes observed exactly |
| **PARTIAL** | Most outcomes correct; minor deviation noted (describe) |
| **FAIL** | A stated outcome was not met; describe actual behaviour |
| **N/A** | Test skipped (infrastructure not available) |
| **BLOCKED** | Cannot run test due to a dependency failing earlier |

Record results per TC ID. Raise a bug report for each FAIL with: TC ID, steps to reproduce, expected outcome, actual outcome, screenshot.
