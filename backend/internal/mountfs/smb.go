package mountfs

import (
	"fmt"
	"io"
	"net"
	"path"
	"strings"

	"github.com/hirochachacha/go-smb2"
)

// SMBConfig holds connection parameters for an SMB/CIFS mount.
type SMBConfig struct {
	Host     string
	Port     int
	Domain   string
	Username string
	Password string
	Share    string // share name, e.g. "backup"
	Root     string // sub-path within the share
}

// SMBFS is a MountFS backed by an SMB share.
type SMBFS struct {
	conn  net.Conn
	sess  *smb2.Session
	share *smb2.Share
	root  string
}

// NewSMB dials the SMB server, authenticates, and mounts the share.
func NewSMB(cfg SMBConfig) (*SMBFS, error) {
	port := cfg.Port
	if port == 0 {
		port = 445
	}
	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", port))

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("TCP dial %s: %w", addr, err)
	}

	dialer := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     cfg.Username,
			Password: cfg.Password,
			Domain:   cfg.Domain,
		},
	}
	sess, err := dialer.Dial(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("SMB negotiate/auth: %w", err)
	}

	share, err := sess.Mount(cfg.Share)
	if err != nil {
		_ = sess.Logoff()
		conn.Close()
		return nil, fmt.Errorf("SMB mount share %q: %w", cfg.Share, err)
	}

	root := cfg.Root
	if root == "" {
		root = "."
	}

	return &SMBFS{conn: conn, sess: sess, share: share, root: root}, nil
}

// winPath joins relPath onto the SMB root, verifies the result is still within
// root, then converts POSIX separators to Windows backslashes as required by
// the SMB client library.
//
// root is always a relative POSIX path within the share (e.g. "backups/logs"
// or "."). path.Join is used internally; strings.ReplaceAll converts the
// result for the SMB wire protocol.
func (s *SMBFS) winPath(relPath string) (string, error) {
	if relPath == "" || relPath == "." {
		if s.root == "." {
			return "", nil
		}
		return strings.ReplaceAll(s.root, "/", `\`), nil
	}

	joined := path.Join(s.root, relPath)

	// Verify the joined path hasn't escaped the root. SMB roots are always
	// relative (within a share), so we use the relative-root confinement check.
	if !smbPathInRoot(joined, s.root) {
		return "", fmt.Errorf("path %q escapes SMB mount root", relPath)
	}

	if joined == "." {
		return "", nil
	}
	return strings.ReplaceAll(joined, "/", `\`), nil
}

// smbPathInRoot reports whether joined is still within root. Both are relative
// POSIX paths (within an SMB share); path.Join has already cleaned them.
//
// Two cases:
//   - root == "." : the share root; joined must not start with "..".
//   - any other  : joined must equal root or start with root+"/".
func smbPathInRoot(joined, root string) bool {
	root = path.Clean(root)
	joined = path.Clean(joined)
	switch root {
	case ".":
		return joined != ".." && !strings.HasPrefix(joined, "../")
	default:
		return joined == root || strings.HasPrefix(joined, root+"/")
	}
}

func (s *SMBFS) Stat(relPath string) (FileInfo, error) {
	p, err := s.winPath(relPath)
	if err != nil {
		return FileInfo{}, err
	}
	fi, err := s.share.Stat(p)
	if err != nil {
		return FileInfo{}, err
	}
	return FileInfo{
		Name:    fi.Name(),
		Size:    fi.Size(),
		ModTime: fi.ModTime(),
		IsDir:   fi.IsDir(),
		Mode:    fi.Mode(),
	}, nil
}

func (s *SMBFS) ReadDir(relPath string) ([]FileInfo, error) {
	p, err := s.winPath(relPath)
	if err != nil {
		return nil, err
	}
	entries, err := s.share.ReadDir(p)
	if err != nil {
		return nil, err
	}
	infos := make([]FileInfo, 0, len(entries))
	for _, fi := range entries {
		infos = append(infos, FileInfo{
			Name:    fi.Name(),
			Size:    fi.Size(),
			ModTime: fi.ModTime(),
			IsDir:   fi.IsDir(),
			Mode:    fi.Mode(),
		})
	}
	return infos, nil
}

func (s *SMBFS) Open(relPath string) (io.ReadCloser, error) {
	p, err := s.winPath(relPath)
	if err != nil {
		return nil, err
	}
	return s.share.Open(p)
}

func (s *SMBFS) Create(relPath string) (io.WriteCloser, error) {
	p, err := s.winPath(relPath)
	if err != nil {
		return nil, err
	}
	// Ensure parent directory exists.
	parentDir := path.Dir(strings.ReplaceAll(p, `\`, "/"))
	if parentDir != "" && parentDir != "." {
		if err := s.share.MkdirAll(strings.ReplaceAll(parentDir, "/", `\`), 0o755); err != nil {
			return nil, err
		}
	}
	return s.share.Create(p)
}

func (s *SMBFS) MkdirAll(relPath string) error {
	p, err := s.winPath(relPath)
	if err != nil {
		return err
	}
	return s.share.MkdirAll(p, 0o755)
}

func (s *SMBFS) Remove(relPath string) error {
	p, err := s.winPath(relPath)
	if err != nil {
		return err
	}
	return s.share.Remove(p)
}

func (s *SMBFS) Rename(oldPath, newPath string) error {
	old, err := s.winPath(oldPath)
	if err != nil {
		return err
	}
	nw, err := s.winPath(newPath)
	if err != nil {
		return err
	}
	return s.share.Rename(old, nw)
}

func (s *SMBFS) Close() error {
	_ = s.share.Umount()
	_ = s.sess.Logoff()
	return s.conn.Close()
}
