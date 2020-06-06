package keyfile_test

import (
	"context"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"testing"

	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/internal"
	"v.io/x/ref/lib/security/signing/keyfile"
)

func createSSHKey(dir string) error {
	return internal.SSHKeygenNoPassphrase(
		filepath.Join(dir, "ecdsa.ssh"),
		"ecdsa-ssh",
		"ecdsa",
		"521",
	)
}

func createPEMKey(dir string) error {
	_, key, err := vsecurity.NewPrincipalKey()
	if err != nil {
		return err
	}
	return internal.CreateVanadiumPEMFileNoPassphrase(key, path.Join(dir, "privatekey.pem"))
}

func createKeys(dir string) error {
	if err := createSSHKey(dir); err != nil {
		return err
	}
	return createPEMKey(dir)
}

func TestKeyFiles(t *testing.T) {
	ctx := context.Background()
	tmpDir, err := ioutil.TempDir("", "test-key-files")
	if err != nil {
		t.Fatalf("TempDir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := createKeys(tmpDir); err != nil {
		t.Fatalf("createKeys: %v", err)
	}

	for _, keyFilename := range []string{"privatekey.pem", "ecdsa.ssh"} {
		svc := keyfile.NewSigningService()
		signer, err := svc.Signer(ctx, filepath.Join(tmpDir, keyFilename), nil)
		if err != nil {
			t.Fatalf("failed to get signer for %v: %v", keyFilename, err)
		}
		sig, err := signer.Sign([]byte("testing"), []byte("hello"))
		if err != nil {
			t.Fatalf("failed to sign message for %v: %v", keyFilename, err)
		}
		if !sig.Verify(signer.PublicKey(), []byte("hello")) {
			t.Errorf("failed to verify signature for %v", keyFilename)
		}
	}
}
