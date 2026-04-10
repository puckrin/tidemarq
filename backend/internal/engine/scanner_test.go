package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestScanDir_ReturnsFiles(t *testing.T) {
	root := t.TempDir()

	writeFile(t, filepath.Join(root, "a.txt"), "hello")
	writeFile(t, filepath.Join(root, "sub", "b.txt"), "world")
	writeFile(t, filepath.Join(root, "sub", "deep", "c.txt"), "deep")

	files, err := scanDir(context.Background(), root, 4)
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
	files, err := scanDir(context.Background(), root, 4)
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

	files, err := scanDir(context.Background(), root, 4)
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

	files, err := scanDir(context.Background(), root, 4)
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

	files, err := scanDir(ctx, root, 4)
	if err == nil {
		t.Fatal("expected an error from cancelled context, got nil")
	}
	if len(files) > 0 {
		t.Errorf("expected no results from cancelled scan, got %d files", len(files))
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
