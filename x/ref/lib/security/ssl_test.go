// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"crypto"
	"crypto/md5"
	"crypto/x509"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/test/sectestdata"
)

func TestSSLKeys(t *testing.T) {
	purpose, message := []byte("testing"), []byte("a message")
	keys, certs, opts := sectestdata.VanadiumSSLData()
	for host, key := range keys {
		cert := certs[host]
		signer, err := seclib.NewInMemorySigner(key)
		if err != nil {
			t.Errorf("failed to create signer for %v: %v", host, err)
		}
		sig, err := signer.Sign(purpose, message)
		if err != nil {
			t.Errorf("failed to sign using %v: %v", host, err)
		}
		if !sig.Verify(signer.PublicKey(), message) {
			t.Errorf("failed to verify signature using %v: %v", host, err)
		}
		if _, err := cert.Verify(opts); err != nil {
			t.Errorf("failed to verify cert for %v: %v", host, err)
		}
	}
}

func publicKeyFingerPrint(t *testing.T, pk crypto.PublicKey) string {
	pkb, err := x509.MarshalPKIXPublicKey(pk)
	if err != nil {
		t.Errorf("failed to marshal public key %v", err)
	}
	hash := md5.Sum(pkb)
	return hex.EncodeToString(hash[:])
}

func TestLetsEncryptKeys(t *testing.T) {
	cpriv, _, opts := sectestdata.LetsEncryptData(sectestdata.SingleHostCert)
	purpose, message := []byte("testing"), []byte("another message")
	signer, err := seclib.NewInMemorySigner(cpriv)
	if err != nil {
		t.Errorf("failed to create signer: %v", err)
	}
	sig, err := signer.Sign(purpose, message)
	if err != nil {
		t.Errorf("failed to sign: %v", err)
	}
	if !sig.Verify(signer.PublicKey(), message) {
		t.Errorf("failed to verify signature: %v", err)
	}

	letsencryptDir, err := sectestdata.LetsEncryptDir(sectestdata.SingleHostCert)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(letsencryptDir)
	filename := filepath.Join(letsencryptDir, "www.labdrive.io.letsencrypt")

	certs, err := seclib.ParseX509CertificateFile(filename)
	if err != nil {
		t.Fatalf("failed to load %v: %v", filename, err)
	}
	if _, err := certs[0].Verify(opts); err != nil {
		t.Errorf("failed to verify x509 certificate: %v", err)
	}
	if got, want := len(certs), 3; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// openssl x509 -in testdata/lwww.labdrive.io.letsencrypt --pubkey --noout |
	// openssl ec --pubin --inform PEM --outform DER |openssl md5 -c
	if got, want := publicKeyFingerPrint(t, certs[0].PublicKey), strings.ReplaceAll("b4:1c:fc:66:5a:60:66:ea:e1:c5:46:76:59:8c:fc:6a", ":", ""); got != want {
		t.Errorf("%v: got %v, want %v", filename, got, want)
	}

	// Now parse the root certificate also.
	certs, err = seclib.ParseX509CertificateFile(
		filepath.Join(letsencryptDir, "letsencrypt-stg-int-e1.pem"))
	if err != nil {
		t.Fatalf("failed to load %v: %v", filename, err)
	}
	if got, want := len(certs), 1; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if _, err := certs[0].Verify(opts); err != nil {
		t.Errorf("failed to verify x509 certificate: %v", err)
	}

	// openssl x509 -in testdata/letsencrypt-stg-int-e1.pem --pubkey --noout |
	// openssl ec --pubin --inform PEM --outform DER |openssl md5 -c
	if got, want := publicKeyFingerPrint(t, certs[0].PublicKey), strings.ReplaceAll("8d:49:53:4b:8c:e3:7a:d5:e0:69:95:18:49:1f:7b:bf", ":", ""); got != want {
		t.Errorf("%v: got %v, want %v", filename, got, want)
	}
}

/*

type testService struct{}

func (testService) Echo(ctx *context.T, call rpc.ServerCall, msg string) (string, error) {
	return "echo: " + msg, nil
}

func TestRPC(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	var err error
	fatal := func() {
		if err != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Fatalf("line: %v, err: %v", line, err)
		}
	}
	filename := filepath.Join("testdata", "www.labdrive.io.letsencrypt")
	serverPrincipal, _, certs, err := sslPrinicipal(filename)
	fatal()

	ours := certs[0]
	pastTime, _ := time.Parse("2006-Jan-02", "2021-Nov-02")
	opts := x509.VerifyOptions{
		Roots:       customCertPool(t, filepath.Join("testdata", "letsencrypt-stg-int-e1.pem")),
		CurrentTime: pastTime,
	}
	chains, err := ours.Verify(opts)
	fatal()

	blessings, err := security.BlessingsForX509Chains(serverPrincipal.PublicKey(), chains)
	fatal()
	//	_ = chains

	//blessings, err := serverPrincipal.BlessSelf("server")
	//fatal()

	security.AddToRoots(serverPrincipal, blessings)
	err = serverPrincipal.BlessingStore().SetDefault(blessings)
	fatal()
	_, err = serverPrincipal.BlessingStore().Set(blessings, security.AllPrincipals)
	fatal()

	fmt.Printf("%s\n", serverPrincipal.BlessingStore().DebugString())

	serverCtx, err := v23.WithPrincipal(ctx, serverPrincipal)
	fatal()
	_, server, err := v23.WithNewServer(serverCtx, "", testService{}, nil)
	fatal()

	testutil.WaitForServerReady(server)
	serverObjectName := server.Status().Endpoints[0].Name()

	clientCtx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("simple"))
	fatal()

	clientCtx, client, err := v23.WithNewClient(clientCtx)
	fatal()

	clientCtx, cancel := context.WithTimeout(clientCtx, time.Second)
	defer cancel()
	_, err = client.StartCall(clientCtx, serverObjectName, "Echo", []interface{}{"hi"})
	if err == nil || !strings.Contains(err.Error(), "client does not trust server") {
		t.Fatalf("expected an error saying that the client does not trust the server, not: %v", err)
	}

	t.Log(err)
	t.FailNow()
}
*/
