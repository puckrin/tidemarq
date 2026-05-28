package mounts_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tidemarq/tidemarq/internal/crypt"
	"github.com/tidemarq/tidemarq/internal/db"
	"github.com/tidemarq/tidemarq/internal/mounts"
	"github.com/tidemarq/tidemarq/migrations"
)

const testSecret = "test-secret-do-not-use-in-production"

func newTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	if err := d.Migrate(migrations.FS); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return d
}

func newService(t *testing.T) (*mounts.Service, *db.DB) {
	t.Helper()
	d := newTestDB(t)
	return mounts.New(d, testSecret), d
}

// rawCredentialBlobs returns the encrypted bytes stored in the mounts row.
// Tests use this to assert that plaintext never reaches disk and that the
// blobs round-trip through crypt.Decrypt with the configured secret.
func rawCredentialBlobs(t *testing.T, d *db.DB, id int64) (passwordEnc, sshKeyEnc []byte) {
	t.Helper()
	row := d.QueryRow(`SELECT password_enc, ssh_key_enc FROM mounts WHERE id = ?`, id)
	if err := row.Scan(&passwordEnc, &sshKeyEnc); err != nil {
		t.Fatalf("scanning raw credentials: %v", err)
	}
	return passwordEnc, sshKeyEnc
}

func TestService_Create_StoresEncryptedAndSanitizes(t *testing.T) {
	svc, d := newService(t)
	ctx := context.Background()

	const plainPassword = "hunter2-correct-horse-battery-staple"
	const plainKey = "-----BEGIN OPENSSH PRIVATE KEY-----\nFAKEKEYDATA\n-----END OPENSSH PRIVATE KEY-----"

	view, err := svc.Create(ctx, mounts.MountInput{
		Name:     "nas",
		Type:     "sftp",
		Host:     "nas.local",
		Port:     22,
		Username: "backup",
		Password: plainPassword,
		SSHKey:   plainKey,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if !view.HasPassword || !view.HasSSHKey {
		t.Errorf("view should advertise HasPassword=true, HasSSHKey=true; got %+v", view)
	}
	// View must not carry any credential field; the JSON struct tag check is
	// the type-level guarantee, but we cross-check by formatting the value too.
	if strings.Contains(formatView(view), plainPassword) || strings.Contains(formatView(view), plainKey) {
		t.Error("MountView leaked plaintext credentials")
	}

	passwordEnc, sshKeyEnc := rawCredentialBlobs(t, d, view.ID)
	if bytes.Contains(passwordEnc, []byte(plainPassword)) {
		t.Error("password_enc contains plaintext password")
	}
	if bytes.Contains(sshKeyEnc, []byte(plainKey)) {
		t.Error("ssh_key_enc contains plaintext SSH key")
	}

	// Independent decrypt with the same secret recovers the plaintext.
	key := crypt.KeyFromSecret(testSecret)
	gotPwd, err := crypt.Decrypt(key, passwordEnc)
	if err != nil {
		t.Fatalf("decrypt password: %v", err)
	}
	if string(gotPwd) != plainPassword {
		t.Errorf("decrypted password mismatch: got %q, want %q", gotPwd, plainPassword)
	}
	gotKey, err := crypt.Decrypt(key, sshKeyEnc)
	if err != nil {
		t.Fatalf("decrypt ssh key: %v", err)
	}
	if string(gotKey) != plainKey {
		t.Errorf("decrypted ssh key mismatch")
	}
}

func TestService_Create_EmptyCredentialsRemainNil(t *testing.T) {
	svc, d := newService(t)
	ctx := context.Background()

	view, err := svc.Create(ctx, mounts.MountInput{
		Name:     "guest-share",
		Type:     "smb",
		Host:     "fileserver",
		SMBShare: "public",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if view.HasPassword || view.HasSSHKey {
		t.Errorf("expected HasPassword=false, HasSSHKey=false; got %+v", view)
	}

	pwd, key := rawCredentialBlobs(t, d, view.ID)
	if len(pwd) != 0 {
		t.Errorf("password_enc should be empty, got %d bytes", len(pwd))
	}
	if len(key) != 0 {
		t.Errorf("ssh_key_enc should be empty, got %d bytes", len(key))
	}
}

func TestService_Create_DuplicateName_ReturnsErrConflict(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	if _, err := svc.Create(ctx, mounts.MountInput{Name: "dup", Type: "sftp", Host: "h"}); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	_, err := svc.Create(ctx, mounts.MountInput{Name: "dup", Type: "smb", Host: "h2"})
	if !errors.Is(err, mounts.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestService_Get_UnknownID_ReturnsErrNotFound(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Get(context.Background(), 99999)
	if !errors.Is(err, mounts.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestService_List_ReturnsSanitizedViews(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	const secret = "do-not-leak-me"
	for _, name := range []string{"a", "b", "c"} {
		if _, err := svc.Create(ctx, mounts.MountInput{
			Name: name, Type: "sftp", Host: "h", Password: secret,
		}); err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
	}

	views, err := svc.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(views) != 3 {
		t.Fatalf("expected 3 views, got %d", len(views))
	}
	for _, v := range views {
		if !v.HasPassword {
			t.Errorf("view %s should have HasPassword=true", v.Name)
		}
		if strings.Contains(formatView(v), secret) {
			t.Errorf("view %s leaked password plaintext", v.Name)
		}
	}
}

// TestService_Update_EmptyPasswordPreservesExisting is the most security-critical
// test in this file. The docstring on Service.Update promises that an empty
// Password/SSHKey leaves the stored credential untouched — a regression here
// would silently wipe credentials whenever someone edited a mount's name.
func TestService_Update_EmptyPasswordPreservesExisting(t *testing.T) {
	svc, d := newService(t)
	ctx := context.Background()

	const original = "original-password"
	view, err := svc.Create(ctx, mounts.MountInput{
		Name: "m", Type: "sftp", Host: "h", Username: "u", Password: original,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	before, _ := rawCredentialBlobs(t, d, view.ID)

	updated, err := svc.Update(ctx, view.ID, mounts.MountInput{
		Name: "m-renamed", Type: "sftp", Host: "h", Username: "u",
		// Password omitted (empty) — must preserve existing.
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated.HasPassword {
		t.Error("HasPassword flipped to false after omitted-password Update")
	}

	after, _ := rawCredentialBlobs(t, d, view.ID)
	if !bytes.Equal(before, after) {
		t.Error("password_enc was mutated by an Update that omitted the password")
	}
}

func TestService_Update_NewPasswordReplacesAndRotatesNonce(t *testing.T) {
	svc, d := newService(t)
	ctx := context.Background()

	view, err := svc.Create(ctx, mounts.MountInput{
		Name: "m", Type: "sftp", Host: "h", Password: "old",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	before, _ := rawCredentialBlobs(t, d, view.ID)

	if _, err := svc.Update(ctx, view.ID, mounts.MountInput{
		Name: "m", Type: "sftp", Host: "h", Password: "new",
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	after, _ := rawCredentialBlobs(t, d, view.ID)

	if bytes.Equal(before, after) {
		t.Error("password_enc unchanged after Update with new password")
	}
	key := crypt.KeyFromSecret(testSecret)
	plain, err := crypt.Decrypt(key, after)
	if err != nil {
		t.Fatalf("decrypt updated password: %v", err)
	}
	if string(plain) != "new" {
		t.Errorf("decrypted password: got %q, want %q", plain, "new")
	}
}

// TestService_Update_SamePassword_RotatesNonce confirms that re-encrypting the
// same plaintext yields a different blob — a sanity check that nonces are
// randomised, not derived from the plaintext.
func TestService_Update_SamePassword_RotatesNonce(t *testing.T) {
	svc, d := newService(t)
	ctx := context.Background()

	const pwd = "same-password"
	view, err := svc.Create(ctx, mounts.MountInput{
		Name: "m", Type: "sftp", Host: "h", Password: pwd,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	first, _ := rawCredentialBlobs(t, d, view.ID)

	if _, err := svc.Update(ctx, view.ID, mounts.MountInput{
		Name: "m", Type: "sftp", Host: "h", Password: pwd,
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	second, _ := rawCredentialBlobs(t, d, view.ID)

	if bytes.Equal(first, second) {
		t.Error("re-encrypting the same plaintext produced an identical blob (nonce not rotated)")
	}
}

func TestService_Update_UnknownID_ReturnsErrNotFound(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Update(context.Background(), 12345, mounts.MountInput{
		Name: "x", Type: "sftp", Host: "h",
	})
	if !errors.Is(err, mounts.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestService_Update_NameCollision_ReturnsErrConflict(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	if _, err := svc.Create(ctx, mounts.MountInput{Name: "a", Type: "sftp", Host: "h"}); err != nil {
		t.Fatalf("Create a: %v", err)
	}
	b, err := svc.Create(ctx, mounts.MountInput{Name: "b", Type: "sftp", Host: "h"})
	if err != nil {
		t.Fatalf("Create b: %v", err)
	}

	_, err = svc.Update(ctx, b.ID, mounts.MountInput{Name: "a", Type: "sftp", Host: "h"})
	if !errors.Is(err, mounts.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestService_Delete(t *testing.T) {
	svc, _ := newService(t)
	ctx := context.Background()

	view, err := svc.Create(ctx, mounts.MountInput{Name: "doomed", Type: "sftp", Host: "h"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := svc.Delete(ctx, view.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := svc.Get(ctx, view.ID); !errors.Is(err, mounts.ErrNotFound) {
		t.Errorf("Get after Delete: expected ErrNotFound, got %v", err)
	}
	if err := svc.Delete(ctx, view.ID); !errors.Is(err, mounts.ErrNotFound) {
		t.Errorf("second Delete: expected ErrNotFound, got %v", err)
	}
}

// TestService_Open_WrongSecret_FailsBeforeNetwork verifies that a Service
// rebuilt with a different secret cannot decrypt previously stored credentials.
// This is the on-disk-protection guarantee: someone with the database but
// without the secret cannot recover the credentials.
func TestService_Open_WrongSecret_FailsBeforeNetwork(t *testing.T) {
	d := newTestDB(t)
	svcA := mounts.New(d, "secret-A")
	svcB := mounts.New(d, "secret-B")
	ctx := context.Background()

	view, err := svcA.Create(ctx, mounts.MountInput{
		Name: "m", Type: "sftp", Host: "127.0.0.1", Port: 22, Username: "u", Password: "p",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	_, err = svcB.Open(ctx, view.ID)
	if err == nil {
		t.Fatal("Open with wrong secret should fail")
	}
	if !strings.Contains(err.Error(), "decrypting") {
		t.Errorf("expected decryption error, got %v", err)
	}
	// The same Service that wrote the row must be able to decrypt — confirms
	// the failure was the secret mismatch, not the data itself.
	if _, _, err := openDecryptOnly(t, d, view.ID, "secret-A"); err != nil {
		t.Errorf("decrypt with original secret failed: %v", err)
	}
}

func TestService_Open_UnknownID_ReturnsErrNotFound(t *testing.T) {
	svc, _ := newService(t)
	_, err := svc.Open(context.Background(), 99999)
	if !errors.Is(err, mounts.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
	if err := svc.TestConnectivity(context.Background(), 99999); !errors.Is(err, mounts.ErrNotFound) {
		t.Errorf("TestConnectivity: expected ErrNotFound, got %v", err)
	}
}

// openDecryptOnly performs the decryption half of Service.Open without
// attempting to dial the remote — used to assert that the failure in
// TestService_Open_WrongSecret_FailsBeforeNetwork was the key, not the blob.
func openDecryptOnly(t *testing.T, d *db.DB, id int64, secret string) (password, sshKey string, err error) {
	t.Helper()
	pwdBlob, keyBlob := rawCredentialBlobs(t, d, id)
	key := crypt.KeyFromSecret(secret)
	if len(pwdBlob) > 0 {
		plain, derr := crypt.Decrypt(key, pwdBlob)
		if derr != nil {
			return "", "", derr
		}
		password = string(plain)
	}
	if len(keyBlob) > 0 {
		plain, derr := crypt.Decrypt(key, keyBlob)
		if derr != nil {
			return "", "", derr
		}
		sshKey = string(plain)
	}
	return password, sshKey, nil
}

func formatView(v *mounts.MountView) string {
	return v.Name + "|" + v.Host + "|" + v.Username + "|" + v.SMBShare + "|" + v.SMBDomain + "|" + v.SFTPHostKey
}
