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
		sess.Logoff()
		conn.Close()
		return nil, fmt.Errorf("SMB mount share %q: %w", cfg.Share, err)
	}

	root := cfg.Root
	if root == "" {
		root = "."
	}

	return &SMBFS{conn: conn, sess: sess, share: share, root: root}, nil
}

func (s *SMBFS) winPath(relPath string) string {
	if relPath == "" || relPath == "." {
		if s.root == "." {
			return ""
		}
		return strings.ReplaceAll(s.root, "/", `\`)
	}
	p := path.Join(s.root, relPath)
	if p == "." {
		return ""
	}
	return strings.ReplaceAll(p, "/", `\`)
}

func (s *SMBFS) Stat(relPath string) (FileInfo, error) {
	fi, err := s.share.Stat(s.winPath(relPath))
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
	entries, err := s.share.ReadDir(s.winPath(relPath))
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
	return s.share.Open(s.winPath(relPath))
}

func (s *SMBFS) Create(relPath string) (io.WriteCloser, error) {
	p := s.winPath(relPath)
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
	return s.share.MkdirAll(s.winPath(relPath), 0o755)
}

func (s *SMBFS) Remove(relPath string) error {
	return s.share.Remove(s.winPath(relPath))
}

func (s *SMBFS) Rename(oldPath, newPath string) error {
	return s.share.Rename(s.winPath(oldPath), s.winPath(newPath))
}

func (s *SMBFS) Close() error {
	s.share.Umount()
	s.sess.Logoff()
	return s.conn.Close()
}
