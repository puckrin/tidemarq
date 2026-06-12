package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/tidemarq/tidemarq/internal/filter"
)

func TestScanDir_ReturnsFiles(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "a.txt"), "hello")
	writeFile(t, filepath.Join(root, "sub", "b.txt"), "world")
	writeFile(t, filepath.Join(root, "sub", "deep", "c.txt"), "deep")

	files, err := scanDir(context.Background(), root, 4, nil, nil)
	if err != nil {
		t.Fatalf("scanDir: %v", err)
	}
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(files))
	}

	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.RelPath
	}
	sort.Strings(paths)

	want := []string{"a.txt", "sub/b.txt", "sub/deep/c.txt"}
	for i, p := range want {
		if paths[i] != p {
			t.Errorf("file[%d]: got %q, want %q", i, paths[i], p)
		}
	}
}

func TestScanDir_EmptyDir(t *testing.T) {
	root := t.TempDir()
	files, err := scanDir(context.Background(), root, 4, nil, nil)
	if err != nil {
		t.Fatalf("scanDir: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("expected 0 files, got %d", len(files))
	}
}

func TestScanDir_SkipsDirectories(t *testing.T) {
	root := t.TempDir()
	// Create a subdirectory but no files inside it.
	if err := os.MkdirAll(filepath.Join(root, "emptydir"), 0755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "file.txt"), "data")

	files, err := scanDir(context.Background(), root, 4, nil, nil)
	if err != nil {
		t.Fatalf("scanDir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].RelPath != "file.txt" {
		t.Errorf("RelPath: got %q, want %q", files[0].RelPath, "file.txt")
	}
}

func TestScanDir_FileMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "file.txt"), "content")

	files, err := scanDir(context.Background(), root, 4, nil, nil)
	if err != nil {
		t.Fatalf("scanDir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	f := files[0]
	if f.Size != int64(len("content")) {
		t.Errorf("Size: got %d, want %d", f.Size, len("content"))
	}
	if f.ModTime.IsZero() {
		t.Error("ModTime is zero")
	}
	if f.Permissions == 0 {
		t.Error("Permissions is zero")
	}
}

// TestScanDir_CancelledContext verifies that a cancelled context causes scanDir
// to return promptly with an error rather than processing the full directory tree.
func TestScanDir_CancelledContext(t *testing.T) {
	root := t.TempDir()
	for i := range 20 {
		writeFile(t, filepath.Join(root, fmt.Sprintf("file%d.txt", i)), "data")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the scan starts

	files, err := scanDir(ctx, root, 4, nil, nil)
	if err == nil {
		t.Fatal("expected an error from cancelled context, got nil")
	}
	if len(files) > 0 {
		t.Errorf("expected no results from cancelled scan, got %d files", len(files))
	}
}

// TestScanDir_AppliesFilter_GlobExcludesAreDropped exercises the milestone-2
// filter wiring: a glob excludes every file under node_modules, and the scan
// result simply omits them — they never appear in FileInfo, so downstream
// engine logic treats them as if they did not exist on disk.
func TestScanDir_AppliesFilter_GlobExcludesAreDropped(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "src", "index.ts"), "a")
	writeFile(t, filepath.Join(root, "src", "util.ts"), "b")
	writeFile(t, filepath.Join(root, "node_modules", "foo", "index.js"), "noise")
	writeFile(t, filepath.Join(root, "apps", "web", "node_modules", "bar.js"), "noise")
	writeFile(t, filepath.Join(root, "README.md"), "ok")

	rs := &filter.Ruleset{
		Rules: []filter.Rule{
			{Type: filter.TypeGlob, Action: filter.Exclude, Pattern: "**/node_modules/**"},
		},
	}

	files, err := scanDir(context.Background(), root, 4, rs, nil)
	if err != nil {
		t.Fatalf("scanDir: %v", err)
	}

	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.RelPath
	}
	sort.Strings(paths)
	want := []string{"README.md", "src/index.ts", "src/util.ts"}
	if len(paths) != len(want) {
		t.Fatalf("got %d files, want %d: %v", len(paths), len(want), paths)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Errorf("file[%d]: got %q, want %q", i, paths[i], want[i])
		}
	}
}

// TestScanDir_AppliesFilter_NilFilterIsLegacyBehaviour confirms that passing
// nil for filters is an explicit "no filtering" — every file appears in the
// scan result, including the ones a ruleset would otherwise exclude.
func TestScanDir_AppliesFilter_NilFilterIsLegacyBehaviour(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.txt"), "1")
	writeFile(t, filepath.Join(root, "node_modules", "x.js"), "2")

	files, err := scanDir(context.Background(), root, 4, nil, nil)
	if err != nil {
		t.Fatalf("scanDir: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("nil filter should return all files, got %d: %+v", len(files), files)
	}
}

// TestScanDir_AppliesFilter_OnScanIgnoresExcluded ensures the running scan
// counter reflects the post-filter total — the UI shows the user what the
// engine will actually process, not the raw on-disk file count.
func TestScanDir_AppliesFilter_OnScanIgnoresExcluded(t *testing.T) {
	root := t.TempDir()
	for i := 0; i < 5; i++ {
		writeFile(t, filepath.Join(root, fmt.Sprintf("k%d.log", i)), "noise")
	}
	writeFile(t, filepath.Join(root, "keep.txt"), "ok")

	rs := &filter.Ruleset{
		Rules: []filter.Rule{{Type: filter.TypeExtension, Action: filter.Exclude, Pattern: ".log"}},
	}

	var lastCount int
	var lastBytes int64
	files, err := scanDir(context.Background(), root, 4, rs, func(n int, b int64) {
		lastCount = n
		lastBytes = b
	})
	if err != nil {
		t.Fatalf("scanDir: %v", err)
	}
	if len(files) != 1 || files[0].RelPath != "keep.txt" {
		t.Fatalf("expected only keep.txt, got %+v", files)
	}
	if lastCount != 1 {
		t.Errorf("onScan final count: got %d, want 1 (excluded files must not count)", lastCount)
	}
	if lastBytes != int64(len("ok")) {
		t.Errorf("onScan final bytes: got %d, want %d", lastBytes, len("ok"))
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
