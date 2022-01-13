// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"crypto"
	"crypto/md5"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/test"
	"v.io/x/ref/test/sectestdata"
	"v.io/x/ref/test/testutil"
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

	// Create a server using an ssl wildcard domain cert and the associated
	// x509.VerifyOptions. It has self-signed x509 derived blessings
	// for foo.labdrive.io and bar.labdr.io. The default blessing, stored in
	// the endpoint is for bar.labdr.io.
	privKey, pubCerts, opts := sectestdata.LetsEncryptData(sectestdata.MultipleWildcardCert)
	signer, err := seclib.NewInMemorySigner(privKey)
	fatal()
	serverPrincipal, err := security.CreatePrincipal(signer,
		seclib.NewBlessingStore(signer.PublicKey()),
		seclib.NewBlessingRootsWithX509Options(opts))
	fatal()

	blessingsFoo, err := serverPrincipal.BlessSelfX509("foo.labdrive.io", pubCerts[0])
	fatal()
	blessingsBar, err := serverPrincipal.BlessSelfX509("bar.labdr.io", pubCerts[0])
	fatal()
	_, err = serverPrincipal.BlessingStore().Set(blessingsFoo, "client")
	fatal()
	_, err = serverPrincipal.BlessingStore().Set(blessingsBar, "client")
	fatal()
	err = serverPrincipal.BlessingStore().SetDefault(blessingsBar)
	fatal()

	serverCtx, err := v23.WithPrincipal(ctx, serverPrincipal)
	fatal()
	_, server, err := v23.WithNewServer(serverCtx, "", testService{}, nil)
	fatal()

	testutil.WaitForServerReady(server)
	serverObjectName := server.Status().Endpoints[0].Name()

	// This call will fail since the x509 certificates used are from the letsencrypt
	// staging environment.
	clientPrincipal, err := newClientPrincipal(seclib.ED25519, x509.VerifyOptions{}, serverPrincipal, blessingsBar, "client", "bar.labdr.io")
	fatal()
	clientCtx, cancel, client, err := newRPCClient(ctx, clientPrincipal)
	defer cancel()
	fatal()

	_, err = client.StartCall(clientCtx, serverObjectName, "Echo", []interface{}{"hi"})
	if err == nil || !strings.Contains(err.Error(), "client does not trust server") {
		t.Fatalf("expected an error saying that the client does not trust the server, not: %v", err)
	}

	// This call will succeed since the x509 opts include the correct
	// CA for the staging certificates.
	clientPrincipal, err = newClientPrincipal(seclib.ED25519, opts, serverPrincipal, blessingsBar, "client", "bar.labdr.io")
	fatal()
	clientCtx, cancel, client, err = newRPCClient(ctx, clientPrincipal)
	fatal()
	defer cancel()

	var result string
	results := []interface{}{&result}
	err = client.Call(clientCtx, serverObjectName, "Echo", []interface{}{"hi"}, results)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if got, want := result, "echo: hi"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func newClientPrincipal(keyType seclib.KeyType, opts x509.VerifyOptions, serverPrincipal security.Principal, serverBlessings security.Blessings, extension string, pattern security.BlessingPattern) (security.Principal, error) {
	privKey, err := seclib.NewPrivateKey(seclib.ED25519)
	if err != nil {
		return nil, err
	}
	signer, err := seclib.NewInMemorySigner(privKey)
	if err != nil {
		return nil, err
	}
	clientPrincipal, err := security.CreatePrincipal(signer,
		seclib.NewBlessingStore(signer.PublicKey()),
		seclib.NewBlessingRootsWithX509Options(opts))
	if err != nil {
		return nil, err
	}
	exp, err := security.NewExpiryCaveat(time.Now().Add(time.Hour))
	if err != nil {
		return nil, err
	}
	clientBlessings, err := serverPrincipal.Bless(clientPrincipal.PublicKey(), serverBlessings, extension, exp)
	if err != nil {
		return nil, err
	}
	_, err = clientPrincipal.BlessingStore().Set(clientBlessings, pattern)
	if err != nil {
		return nil, err
	}
	fmt.Printf("server:\n%v\n", serverPrincipal.BlessingStore().DebugString())
	fmt.Printf("client:\n%v\n", clientPrincipal.BlessingStore().DebugString())
	return clientPrincipal, nil
}

func newRPCClient(ctx *context.T, clientPrincipal security.Principal) (*context.T, func(), rpc.Client, error) {
	clientCtx, err := v23.WithPrincipal(ctx, clientPrincipal)
	if err != nil {
		return nil, nil, nil, err
	}
	clientCtx, client, err := v23.WithNewClient(clientCtx)
	if err != nil {
		return nil, nil, nil, err
	}

	clientCtx, cancel := context.WithTimeout(clientCtx, time.Second)
	if err != nil {
		return nil, nil, nil, err
	}
	return clientCtx, cancel, client, nil
}
