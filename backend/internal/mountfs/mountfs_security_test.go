package mountfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// ── LocalFS ──────────────────────────────────────────────────────────────────

func TestLocalFS_abs_TraversalRejected(t *testing.T) {
	root := t.TempDir()
	l := &LocalFS{root: root}

	cases := []string{
		"../../etc",
		"../secret",
		"foo/../../bar",
		"..",
		"../",
		"subdir/../..",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			_, err := l.abs(c)
			if err == nil {
				t.Errorf("abs(%q): expected traversal error, got nil", c)
			}
		})
	}
}

func TestLocalFS_abs_ValidPathsAccepted(t *testing.T) {
	root := t.TempDir()
	l := &LocalFS{root: root}

	cases := []string{
		"",
		".",
		"file.txt",
		"subdir/file.txt",
		"a/b/c/deep.txt",
	}
	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			_, err := l.abs(c)
			if err != nil {
				t.Errorf("abs(%q): unexpected error: %v", c, err)
			}
		})
	}
}

// TestLocalFS_Stat_TraversalBlocked confirms that traversal is blocked at the
// public API boundary — the error propagates from abs() through Stat().
func TestLocalFS_Stat_TraversalBlocked(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "mount")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	// Sensitive file outside the mount root.
	secret := filepath.Join(outer, "secret.txt")
	if err := os.WriteFile(secret, []byte("top-secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := NewLocalFS(root)

	_, err := l.Stat("../secret.txt")
	if err == nil {
		t.Fatal("Stat(../secret.txt): expected error, traversal succeeded")
	}
}

// TestLocalFS_Open_TraversalBlocked confirms Open refuses traversal paths.
func TestLocalFS_Open_TraversalBlocked(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "mount")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(outer, "secret.txt")
	if err := os.WriteFile(secret, []byte("top-secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := NewLocalFS(root)

	f, err := l.Open("../secret.txt")
	if err == nil {
		f.Close()
		t.Fatal("Open(../secret.txt): expected error, traversal succeeded")
	}
}

// TestLocalFS_Create_TraversalBlocked confirms Create refuses traversal paths.
func TestLocalFS_Create_TraversalBlocked(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "mount")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}

	l := NewLocalFS(root)

	wc, err := l.Create("../../injected.txt")
	if err == nil {
		wc.Close()
		t.Fatal("Create(../../injected.txt): expected error, traversal succeeded")
	}
	// Confirm nothing was written outside the root.
	if _, statErr := os.Stat(filepath.Join(outer, "injected.txt")); !os.IsNotExist(statErr) {
		t.Fatal("file was created outside mount root despite traversal guard")
	}
}

// TestLocalFS_Remove_TraversalBlocked confirms Remove refuses traversal paths.
func TestLocalFS_Remove_TraversalBlocked(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "mount")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(outer, "important.txt")
	if err := os.WriteFile(target, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := NewLocalFS(root)

	if err := l.Remove("../important.txt"); err == nil {
		t.Fatal("Remove(../important.txt): expected error, traversal succeeded")
	}
	if _, statErr := os.Stat(target); errors.Is(statErr, os.ErrNotExist) {
		t.Fatal("file outside mount root was deleted by traversal")
	}
}

// TestLocalFS_Rename_TraversalBlocked confirms Rename refuses traversal paths
// for both source and destination.
func TestLocalFS_Rename_TraversalBlocked(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "mount")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(root, "file.txt")
	if err := os.WriteFile(inside, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := NewLocalFS(root)

	// Traversal in destination: rename a legitimate file to outside the root.
	if err := l.Rename("file.txt", "../escaped.txt"); err == nil {
		t.Fatal("Rename(file.txt, ../escaped.txt): expected error, traversal in dst succeeded")
	}
	if _, statErr := os.Stat(filepath.Join(outer, "escaped.txt")); !os.IsNotExist(statErr) {
		t.Fatal("file was renamed to outside mount root")
	}

	// Traversal in source: read a file from outside the root.
	if err := l.Rename("../secret", "dest.txt"); err == nil {
		t.Fatal("Rename(../secret, dest.txt): expected error, traversal in src succeeded")
	}
}

// TestLocalFS_NormalOperation confirms valid operations still work correctly
// after the traversal guards are in place.
func TestLocalFS_NormalOperation(t *testing.T) {
	root := t.TempDir()
	l := NewLocalFS(root)

	// Create and read a file at a nested path.
	wc, err := l.Create("subdir/hello.txt")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := wc.Write([]byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	wc.Close()

	rc, err := l.Open("subdir/hello.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	buf := make([]byte, 16)
	n, _ := rc.Read(buf)
	rc.Close()
	if string(buf[:n]) != "hello" {
		t.Errorf("content: got %q, want %q", string(buf[:n]), "hello")
	}

	// Stat.
	fi, err := l.Stat("subdir/hello.txt")
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.Name != "hello.txt" {
		t.Errorf("Name: got %q", fi.Name)
	}

	// ReadDir.
	entries, err := l.ReadDir("subdir")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "hello.txt" {
		t.Errorf("ReadDir: unexpected entries: %v", entries)
	}

	// Remove.
	if err := l.Remove("subdir/hello.txt"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := l.Stat("subdir/hello.txt"); !os.IsNotExist(err) {
		t.Error("file should be gone after Remove")
	}
}

// ── SFTP path function ────────────────────────────────────────────────────────

func TestSFTPFS_absPath_TraversalRejected(t *testing.T) {
	cases := []struct {
		root    string
		relPath string
	}{
		// Absolute root
		{root: "/remote/root", relPath: "../../etc"},
		{root: "/remote/root", relPath: "../secret"},
		{root: "/remote/root", relPath: "foo/../../bar"},
		{root: "/remote/root", relPath: ".."},
		// Relative root ("." = server working directory)
		{root: ".", relPath: "../../etc"},
		{root: ".", relPath: "../secret"},
		// Deeper relative root
		{root: "backups/data", relPath: "../../etc"},
		{root: "backups/data", relPath: "../../../secret"},
	}

	for _, tc := range cases {
		t.Run(tc.root+"|"+tc.relPath, func(t *testing.T) {
			s := &SFTPFS{root: tc.root}
			_, err := s.absPath(tc.relPath)
			if err == nil {
				t.Errorf("absPath(root=%q, rel=%q): expected traversal error, got nil", tc.root, tc.relPath)
			}
		})
	}
}

func TestSFTPFS_absPath_ValidPathsAccepted(t *testing.T) {
	cases := []struct {
		root    string
		relPath string
	}{
		{root: "/remote/root", relPath: ""},
		{root: "/remote/root", relPath: "."},
		{root: "/remote/root", relPath: "file.txt"},
		{root: "/remote/root", relPath: "subdir/file.txt"},
		{root: "/remote/root", relPath: "a/b/c"},
		// Slash root: entire server accessible by design.
		{root: "/", relPath: "etc"},
		{root: "/", relPath: "home/user/file"},
		// Relative root at CWD.
		{root: ".", relPath: "file.txt"},
		{root: ".", relPath: "subdir/deep/file"},
		// Relative sub-root.
		{root: "backups/data", relPath: "2024/jan"},
		{root: "backups/data", relPath: "file.txt"},
	}

	for _, tc := range cases {
		t.Run(tc.root+"|"+tc.relPath, func(t *testing.T) {
			s := &SFTPFS{root: tc.root}
			got, err := s.absPath(tc.relPath)
			if err != nil {
				t.Errorf("absPath(root=%q, rel=%q): unexpected error: %v", tc.root, tc.relPath, err)
			}
			_ = got
		})
	}
}

// TestSFTPPathInRoot exercises the confinement logic for all root variants.
func TestSFTPPathInRoot(t *testing.T) {
	cases := []struct {
		joined string
		root   string
		want   bool
	}{
		// Absolute root — must be at-or-below root.
		{"/remote/root/file", "/remote/root", true},
		{"/remote/root", "/remote/root", true},
		{"/remote/rootextra", "/remote/root", false}, // prefix match without separator
		{"/etc", "/remote/root", false},
		{"/remote", "/remote/root", false},
		// Slash root — entire server.
		{"/etc/passwd", "/", true},
		{"/home/user", "/", true},
		// Relative root at "." — anything non-escaping.
		{"file.txt", ".", true},
		{"subdir/file", ".", true},
		{"..", ".", false},
		{"../etc", ".", false},
		// Relative sub-root.
		{"backups/data/file", "backups/data", true},
		{"backups/data", "backups/data", true},
		{"backups/dataextra", "backups/data", false},
		{"backups/other", "backups/data", false},
		{"../other", "backups/data", false},
	}

	for _, tc := range cases {
		t.Run(tc.root+"|"+tc.joined, func(t *testing.T) {
			got := sftpPathInRoot(tc.joined, tc.root)
			if got != tc.want {
				t.Errorf("sftpPathInRoot(%q, %q) = %v, want %v", tc.joined, tc.root, got, tc.want)
			}
		})
	}
}

// ── SMB path function ─────────────────────────────────────────────────────────

func TestSMBFS_winPath_TraversalRejected(t *testing.T) {
	cases := []struct {
		root    string
		relPath string
	}{
		// Share root (".")
		{root: ".", relPath: "../../etc"},
		{root: ".", relPath: "../secret"},
		{root: ".", relPath: "foo/../../bar"},
		{root: ".", relPath: ".."},
		// Sub-root within share
		{root: "backups/logs", relPath: "../../secret"},
		{root: "backups/logs", relPath: "../../../etc"},
		{root: "backups/logs", relPath: ".."},
	}

	for _, tc := range cases {
		t.Run(tc.root+"|"+tc.relPath, func(t *testing.T) {
			s := &SMBFS{root: tc.root}
			_, err := s.winPath(tc.relPath)
			if err == nil {
				t.Errorf("winPath(root=%q, rel=%q): expected traversal error, got nil", tc.root, tc.relPath)
			}
		})
	}
}

func TestSMBFS_winPath_ValidPathsAccepted(t *testing.T) {
	cases := []struct {
		root    string
		relPath string
		wantWin string
	}{
		{root: ".", relPath: "", wantWin: ""},
		{root: ".", relPath: ".", wantWin: ""},
		{root: ".", relPath: "file.txt", wantWin: `file.txt`},
		{root: ".", relPath: "subdir/file.txt", wantWin: `subdir\file.txt`},
		{root: "backups/logs", relPath: "2024/jan", wantWin: `backups\logs\2024\jan`},
		{root: "backups/logs", relPath: "file.txt", wantWin: `backups\logs\file.txt`},
		{root: "backups/logs", relPath: "", wantWin: `backups\logs`},
	}

	for _, tc := range cases {
		t.Run(tc.root+"|"+tc.relPath, func(t *testing.T) {
			s := &SMBFS{root: tc.root}
			got, err := s.winPath(tc.relPath)
			if err != nil {
				t.Errorf("winPath(root=%q, rel=%q): unexpected error: %v", tc.root, tc.relPath, err)
				return
			}
			if got != tc.wantWin {
				t.Errorf("winPath(root=%q, rel=%q) = %q, want %q", tc.root, tc.relPath, got, tc.wantWin)
			}
		})
	}
}

// TestSMBPathInRoot exercises the confinement logic for the SMB case.
func TestSMBPathInRoot(t *testing.T) {
	cases := []struct {
		joined string
		root   string
		want   bool
	}{
		// Share root at "."
		{"file.txt", ".", true},
		{"subdir/file", ".", true},
		{"..", ".", false},
		{"../etc", ".", false},
		// Sub-root
		{"backups/logs/file", "backups/logs", true},
		{"backups/logs", "backups/logs", true},
		{"backups/logsextra", "backups/logs", false},
		{"backups/other", "backups/logs", false},
		{"../other", "backups/logs", false},
	}

	for _, tc := range cases {
		t.Run(tc.root+"|"+tc.joined, func(t *testing.T) {
			got := smbPathInRoot(tc.joined, tc.root)
			if got != tc.want {
				t.Errorf("smbPathInRoot(%q, %q) = %v, want %v", tc.joined, tc.root, got, tc.want)
			}
		})
	}
}
