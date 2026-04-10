package api

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
)

// browseEntry is one item returned by the browse API.
type browseEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
	Size  int64  `json:"size,omitempty"`
}

// browseResponse is the shape returned by GET /api/v1/browse.
type browseResponse struct {
	Path    string        `json:"path"`
	Entries []browseEntry `json:"entries"`
}

// handleBrowse serves GET /api/v1/browse?path=...&mount_id=...
// An empty path ("") means "top level": drive list on Windows, root dir on Unix.
// When mount_id is present the path is relative to the mount root.
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	browsePath := q.Get("path")
	mountIDStr := q.Get("mount_id")

	if mountIDStr != "" && mountIDStr != "0" {
		s.handleBrowseMount(w, r, mountIDStr, browsePath)
		return
	}

	// --- Local filesystem browse ---

	// Empty path → "top level". On Windows this means listing available drives;
	// on other platforms it means the filesystem root ("/").
	if browsePath == "" {
		if runtime.GOOS == "windows" {
			s.handleBrowseDrives(w)
			return
		}
		browsePath = "/"
	}

	// On Windows, a bare drive letter without a separator (e.g. "C:") should
	// resolve to the drive root, not the current working directory on that drive.
	if runtime.GOOS == "windows" && len(browsePath) == 2 && browsePath[1] == ':' {
		browsePath = browsePath + "/"
	}

	abs := filepath.Clean(filepath.FromSlash(browsePath))

	entries, err := os.ReadDir(abs)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, http.StatusNotFound, "path not found", "not_found")
			return
		}
		if os.IsPermission(err) {
			writeError(w, http.StatusForbidden, "permission denied", "forbidden")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to read directory", "internal_error")
		return
	}

	resp := browseResponse{
		Path:    filepath.ToSlash(abs),
		Entries: make([]browseEntry, 0, len(entries)),
	}
	for _, e := range entries {
		be := browseEntry{
			Name:  e.Name(),
			IsDir: e.IsDir(),
		}
		if !e.IsDir() {
			if fi, err := e.Info(); err == nil {
				be.Size = fi.Size()
			}
		}
		resp.Entries = append(resp.Entries, be)
	}

	sortEntries(resp.Entries)
	writeJSON(w, http.StatusOK, resp)
}

// handleBrowseDrives returns the list of available drive letters on Windows.
// It probes A–Z and includes any drive whose root is stat-able.
func (s *Server) handleBrowseDrives(w http.ResponseWriter) {
	var entries []browseEntry
	for c := 'A'; c <= 'Z'; c++ {
		root := string(c) + ":/"
		if _, err := os.Stat(root); err == nil {
			entries = append(entries, browseEntry{Name: string(c) + ":", IsDir: true})
		}
	}
	writeJSON(w, http.StatusOK, browseResponse{
		Path:    "",
		Entries: entries,
	})
}

func (s *Server) handleBrowseMount(w http.ResponseWriter, r *http.Request, mountIDStr, browsePath string) {
	if s.mountsSvc == nil {
		writeError(w, http.StatusServiceUnavailable, "mounts service not available", "service_unavailable")
		return
	}

	mountID, err := strconv.ParseInt(mountIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid mount_id", "bad_request")
		return
	}

	mfs, err := s.mountsSvc.Open(r.Context(), mountID)
	if err != nil {
		writeError(w, http.StatusNotFound, "mount not found or unreachable", "not_found")
		return
	}
	defer mfs.Close()

	if browsePath == "/" {
		browsePath = ""
	}

	entries, err := mfs.ReadDir(browsePath)
	if err != nil {
		writeError(w, http.StatusNotFound, "path not found on mount", "not_found")
		return
	}

	resp := browseResponse{
		Path:    browsePath,
		Entries: make([]browseEntry, 0, len(entries)),
	}
	for _, e := range entries {
		be := browseEntry{
			Name:  e.Name,
			IsDir: e.IsDir,
		}
		if !e.IsDir {
			be.Size = e.Size
		}
		resp.Entries = append(resp.Entries, be)
	}

	sortEntries(resp.Entries)
	writeJSON(w, http.StatusOK, resp)
}

func sortEntries(entries []browseEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})
}
