package mountfs

import (
	"fmt"
	"io"
	"net"
	"path"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTPConfig holds connection parameters for an SFTP mount.
type SFTPConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string // used if non-empty
	PrivateKey []byte // PEM-encoded private key; used if non-nil
	HostKey    string // known host key fingerprint for TOFU; empty = accept any
	Root       string // remote root path
}

// SFTPFS is a MountFS backed by an SFTP connection.
type SFTPFS struct {
	client *sftp.Client
	sshc   *ssh.Client
	root   string
}

// NewSFTP dials the SFTP server and returns an SFTPFS.
func NewSFTP(cfg SFTPConfig) (*SFTPFS, error) {
	var authMethods []ssh.AuthMethod

	if len(cfg.PrivateKey) > 0 {
		signer, err := ssh.ParsePrivateKey(cfg.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("parsing SSH private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}
	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method configured")
	}

	var hostKeyCallback ssh.HostKeyCallback
	if cfg.HostKey == "" {
		// Accept any host key (TOFU: caller stores the fingerprint on first connect).
		hostKeyCallback = ssh.InsecureIgnoreHostKey() //nolint:gosec
	} else {
		// Accept only the stored fingerprint.
		hostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fingerprint := ssh.FingerprintSHA256(key)
			if fingerprint != cfg.HostKey {
				return fmt.Errorf("SSH host key mismatch: got %s, expected %s", fingerprint, cfg.HostKey)
			}
			return nil
		}
	}

	port := cfg.Port
	if port == 0 {
		port = 22
	}
	addr := fmt.Sprintf("%s:%d", cfg.Host, port)

	sshCfg := &ssh.ClientConfig{
		User:            cfg.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
	}

	sshc, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		return nil, fmt.Errorf("SSH dial %s: %w", addr, err)
	}

	client, err := sftp.NewClient(sshc)
	if err != nil {
		sshc.Close()
		return nil, fmt.Errorf("SFTP client: %w", err)
	}

	root := cfg.Root
	if root == "" {
		root = "."
	}

	return &SFTPFS{client: client, sshc: sshc, root: root}, nil
}

func (s *SFTPFS) absPath(relPath string) string {
	if relPath == "" || relPath == "." {
		return s.root
	}
	return path.Join(s.root, relPath)
}

func (s *SFTPFS) Stat(relPath string) (FileInfo, error) {
	fi, err := s.client.Stat(s.absPath(relPath))
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

func (s *SFTPFS) ReadDir(relPath string) ([]FileInfo, error) {
	entries, err := s.client.ReadDir(s.absPath(relPath))
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

func (s *SFTPFS) Open(relPath string) (io.ReadCloser, error) {
	return s.client.Open(s.absPath(relPath))
}

func (s *SFTPFS) Create(relPath string) (io.WriteCloser, error) {
	p := s.absPath(relPath)
	if err := s.client.MkdirAll(path.Dir(p)); err != nil {
		return nil, err
	}
	return s.client.Create(p)
}

func (s *SFTPFS) MkdirAll(relPath string) error {
	return s.client.MkdirAll(s.absPath(relPath))
}

func (s *SFTPFS) Remove(relPath string) error {
	return s.client.Remove(s.absPath(relPath))
}

func (s *SFTPFS) Rename(oldPath, newPath string) error {
	return s.client.Rename(s.absPath(oldPath), s.absPath(newPath))
}

func (s *SFTPFS) Close() error {
	s.client.Close()
	return s.sshc.Close()
}
