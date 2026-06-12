package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/tidemarq/tidemarq/internal/engine"
	"github.com/tidemarq/tidemarq/internal/filter"
)

// destFiles walks dst and returns every regular-file relative path, slash-
// normalised and sorted. The .tidemarq-quarantine subtree is excluded
// because the engine writes housekeeping files there; tests check the
// quarantine subtree separately when they care.
func destFiles(t *testing.T, dst string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(dst, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".tidemarq-quarantine" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(dst, p)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walking destination: %v", err)
	}
	sort.Strings(out)
	return out
}

// quarantineFiles returns the relative paths of files quarantined under dst,
// each with the timestamp suffix stripped so tests can assert what the engine
// soft-deleted regardless of when it happened.
func quarantineFiles(t *testing.T, dst string) []string {
	t.Helper()
	qroot := filepath.Join(dst, ".tidemarq-quarantine")
	if _, err := os.Stat(qroot); os.IsNotExist(err) {
		return nil
	}
	var out []string
	_ = filepath.WalkDir(qroot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(qroot, p)
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(out)
	return out
}

func TestEngine_Filter_GlobExcludesAreNotCopied(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	writeFile(t, filepath.Join(src, "src", "index.ts"), "real")
	writeFile(t, filepath.Join(src, "src", "util.ts"), "real")
	writeFile(t, filepath.Join(src, "node_modules", "foo", "index.js"), "noise")
	writeFile(t, filepath.Join(src, "apps", "web", "node_modules", "bar.js"), "noise")
	writeFile(t, filepath.Join(src, "README.md"), "ok")

	rs := &filter.Ruleset{
		Rules: []filter.Rule{
			{Type: filter.TypeGlob, Action: filter.Exclude, Pattern: "**/node_modules/**"},
		},
	}
	result, err := eng.Run(context.Background(), engine.Config{
		JobID: jobID, SourcePath: src, DestinationPath: dst, Filters: rs,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.FilesCopied != 3 {
		t.Errorf("FilesCopied: got %d, want 3 (excluded files do not count)", result.FilesCopied)
	}

	got := destFiles(t, dst)
	want := []string{"README.md", "src/index.ts", "src/util.ts"}
	if !equalSlices(got, want) {
		t.Errorf("destination contents:\n got %v\nwant %v", got, want)
	}
}

func TestEngine_Filter_ExtensionExclude(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	writeFile(t, filepath.Join(src, "app.log"), "log")
	writeFile(t, filepath.Join(src, "data", "debug.LOG"), "log")
	writeFile(t, filepath.Join(src, "keep.txt"), "keep")

	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeExtension, Action: filter.Exclude, Pattern: ".log"}},
	}
	_, err := eng.Run(context.Background(), engine.Config{
		JobID: jobID, SourcePath: src, DestinationPath: dst, Filters: rs,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := destFiles(t, dst)
	want := []string{"keep.txt"}
	if !equalSlices(got, want) {
		t.Errorf("destination contents:\n got %v\nwant %v", got, want)
	}
}

func TestEngine_Filter_SizeExclude_LargerThan(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	writeFile(t, filepath.Join(src, "small.bin"), strings.Repeat("a", 1024))
	writeFile(t, filepath.Join(src, "huge.bin"), strings.Repeat("a", 100*1024))

	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeSize, Action: filter.Exclude, SizeAboveBytes: 8192}},
	}
	_, err := eng.Run(context.Background(), engine.Config{
		JobID: jobID, SourcePath: src, DestinationPath: dst, Filters: rs,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := destFiles(t, dst)
	want := []string{"small.bin"}
	if !equalSlices(got, want) {
		t.Errorf("destination contents:\n got %v\nwant %v", got, want)
	}
}

func TestEngine_Filter_ModifiedBeforeDaysAgo(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	stalePath := filepath.Join(src, "stale.txt")
	freshPath := filepath.Join(src, "fresh.txt")
	writeFile(t, stalePath, "stale")
	writeFile(t, freshPath, "fresh")

	// Backdate stale.txt 60 days; leave fresh.txt at "now".
	stale := time.Now().AddDate(0, 0, -60)
	if err := os.Chtimes(stalePath, stale, stale); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeModified, Action: filter.Exclude, ModifiedBeforeDaysAgo: 30}},
	}
	_, err := eng.Run(context.Background(), engine.Config{
		JobID: jobID, SourcePath: src, DestinationPath: dst, Filters: rs,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := destFiles(t, dst)
	want := []string{"fresh.txt"}
	if !equalSlices(got, want) {
		t.Errorf("destination contents:\n got %v\nwant %v", got, want)
	}
}

func TestEngine_Filter_ExcludeHidden(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	writeFile(t, filepath.Join(src, ".env"), "secret")
	writeFile(t, filepath.Join(src, ".git", "config"), "git")
	writeFile(t, filepath.Join(src, "src", ".cache", "x"), "cache")
	writeFile(t, filepath.Join(src, "src", "main.go"), "code")
	writeFile(t, filepath.Join(src, "README.md"), "readme")

	rs := &filter.Ruleset{ExcludeHidden: true}
	_, err := eng.Run(context.Background(), engine.Config{
		JobID: jobID, SourcePath: src, DestinationPath: dst, Filters: rs,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := destFiles(t, dst)
	want := []string{"README.md", "src/main.go"}
	if !equalSlices(got, want) {
		t.Errorf("destination contents:\n got %v\nwant %v", got, want)
	}
}

// TestEngine_Filter_ExplicitIncludeBeatsExcludeHidden mirrors the
// internal/filter unit test: an explicit include rule must override the
// ExcludeHidden fallback, otherwise a user cannot keep a single dotfile
// (.env, .editorconfig, etc.) in an otherwise-clean tree.
func TestEngine_Filter_ExplicitIncludeBeatsExcludeHidden(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	writeFile(t, filepath.Join(src, ".env"), "kept")
	writeFile(t, filepath.Join(src, ".git", "config"), "dropped")
	writeFile(t, filepath.Join(src, "main.go"), "kept")

	rs := &filter.Ruleset{
		ExcludeHidden: true,
		Rules: []filter.Rule{
			{Type: filter.TypeGlob, Action: filter.Include, Pattern: ".env"},
		},
	}
	_, err := eng.Run(context.Background(), engine.Config{
		JobID: jobID, SourcePath: src, DestinationPath: dst, Filters: rs,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := destFiles(t, dst)
	want := []string{".env", "main.go"}
	if !equalSlices(got, want) {
		t.Errorf("destination contents:\n got %v\nwant %v", got, want)
	}
}

// TestEngine_Filter_IdempotentWithFilters is the key correctness guarantee:
// running the same job twice with a filter must produce zero mutations on
// the second run. A regression here would manifest as "every run copies the
// same files again" — the worst possible outcome.
func TestEngine_Filter_IdempotentWithFilters(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	writeFile(t, filepath.Join(src, "keep.txt"), "stay")
	writeFile(t, filepath.Join(src, "skip.log"), "noise")
	writeFile(t, filepath.Join(src, "node_modules", "x.js"), "noise")

	rs := &filter.Ruleset{
		Rules: []filter.Rule{
			{Type: filter.TypeExtension, Action: filter.Exclude, Pattern: ".log"},
			{Type: filter.TypeGlob, Action: filter.Exclude, Pattern: "**/node_modules/**"},
		},
	}
	cfg := engine.Config{JobID: jobID, SourcePath: src, DestinationPath: dst, Filters: rs}

	first, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	if first.FilesCopied != 1 {
		t.Errorf("first run: FilesCopied = %d, want 1 (only keep.txt)", first.FilesCopied)
	}

	second, err := eng.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if second.FilesCopied != 0 {
		t.Errorf("second run: FilesCopied = %d, want 0 (idempotency)", second.FilesCopied)
	}
	if second.FilesSkipped != 1 {
		t.Errorf("second run: FilesSkipped = %d, want 1", second.FilesSkipped)
	}
}

// TestEngine_Filter_MirrorDoesNotQuarantineExcludedDestFile is the
// correctness check for the milestone-2 design decision: in mirror mode the
// engine deletes destination files that have no source. If we apply the
// filter to source-only, an excluded source file would look "missing" and
// its destination copy would be soft-deleted to quarantine — destroying
// the user's data. The fix: apply the same ruleset to both sides so the
// engine never sees the destination copy at all.
func TestEngine_Filter_MirrorDoesNotQuarantineExcludedDestFile(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	// Pre-seed destination as if a previous unfiltered run had copied the
	// .log file across; then introduce a filter that would have excluded it.
	writeFile(t, filepath.Join(src, "keep.txt"), "kept")
	writeFile(t, filepath.Join(dst, "kept_from_before.log"), "preserved")
	writeFile(t, filepath.Join(dst, "keep.txt"), "kept")

	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeExtension, Action: filter.Exclude, Pattern: ".log"}},
	}
	_, err := eng.Run(context.Background(), engine.Config{
		JobID: jobID, SourcePath: src, DestinationPath: dst, Mode: "one-way-mirror", Filters: rs,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The .log file must still be at dst — neither deleted nor quarantined.
	if _, err := os.Stat(filepath.Join(dst, "kept_from_before.log")); os.IsNotExist(err) {
		t.Error("excluded destination file was removed; mirror must skip files the filter excludes on both sides")
	}
	if q := quarantineFiles(t, dst); len(q) != 0 {
		t.Errorf("excluded destination file was quarantined: %v", q)
	}
}

// TestEngine_Filter_TighteningFilterDoesNotDeleteFromBackupDest verifies
// that adding a filter to an existing backup job does not retroactively
// delete files that were copied before the filter was added — backup mode
// is append-only by spec.
func TestEngine_Filter_TighteningFilterDoesNotDeleteFromBackupDest(t *testing.T) {
	eng, jobID, src, dst := testEnv(t)

	// First run: no filter, all files copy.
	writeFile(t, filepath.Join(src, "a.log"), "1")
	writeFile(t, filepath.Join(src, "b.txt"), "2")
	if _, err := eng.Run(context.Background(), engine.Config{
		JobID: jobID, SourcePath: src, DestinationPath: dst,
	}); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	// Second run with a filter that excludes .log files.
	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeExtension, Action: filter.Exclude, Pattern: ".log"}},
	}
	if _, err := eng.Run(context.Background(), engine.Config{
		JobID: jobID, SourcePath: src, DestinationPath: dst, Filters: rs,
	}); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	// a.log must still be on dst (backup mode never deletes from dst).
	if _, err := os.Stat(filepath.Join(dst, "a.log")); os.IsNotExist(err) {
		t.Error("backup mode deleted a previously-copied file when its source was newly excluded")
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
