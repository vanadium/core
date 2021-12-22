// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/internal"
	"v.io/x/ref/test"
	"v.io/x/ref/test/testutil"
)

func customCertPool(t *testing.T, cafile string) *x509.CertPool {
	rf, err := os.Open(cafile)
	if err != nil {
		t.Fatal(err)
	}
	defer rf.Close()
	rootCert, err := internal.LoadCertificate(rf)
	if err != nil {
		t.Fatal(err)
	}
	certPool := x509.NewCertPool()
	certPool.AddCert(rootCert)
	return certPool
}

func TestSSLKeys(t *testing.T) {
	certPool := customCertPool(t, filepath.Join("testdata", "root-ca.pem"))
	purpose, message := []byte("testing"), []byte("a message")
	for _, tc := range []struct {
		prefix string
		pk     string
	}{
		{"ec256.vanadium.io", "7e:1f:4d:6d:99:3b:6c:51:16:90:83:cf:07:a9:a3:fc"},
		{"ed25519.vanadium.io", "c4:33:17:69:02:42:8c:19:5d:69:77:02:71:c5:1d:7e"},
		{"rsa2048.vanadium.io", "53:fb:b2:07:10:fd:9c:89:16:f5:76:4b:e8:5c:17:30"},
		{"rsa4096.vanadium.io", "10:d6:7b:2f:7d:a2:6b:96:c1:27:50:05:ce:d6:d5:26"},
	} {

		privKeyFile := tc.prefix + ".key"
		cpriv, err := seclib.ParsePEMPrivateKeyFile(filepath.Join("testdata", privKeyFile),
			nil)
		if err != nil {
			t.Errorf("failed to load %v: %v", privKeyFile, err)
		}
		signer, err := seclib.NewInMemorySigner(cpriv)
		if err != nil {
			t.Errorf("failed to create signer for %v: %v", privKeyFile, err)
		}
		sig, err := signer.Sign(purpose, message)
		if err != nil {
			t.Errorf("failed to sign using %v: %v", privKeyFile, err)
		}
		if !sig.Verify(signer.PublicKey(), message) {
			t.Errorf("failed to verify signature using %v: %v", privKeyFile, err)
		}

		opts := x509.VerifyOptions{
			Roots: certPool,
		}
		crtFile := tc.prefix + ".crt"
		cert, err := seclib.ParseX509CertificateFile(filepath.Join("testdata", crtFile), opts)
		if err != nil {
			t.Errorf("failed to load %v: %v", crtFile, err)
			continue
		}

		if got, want := cert.PublicKey.String(), tc.pk; got != want {
			t.Errorf("%v: got %v, want %v", crtFile, got, want)
		}
	}
}

func TestLetsEncryptKeys(t *testing.T) {
	filename := filepath.Join("testdata", "www.labdrive.io.letsencrypt")
	purpose, message := []byte("testing"), []byte("another message")
	cpriv, err := seclib.ParsePEMPrivateKeyFile(filename, nil)
	if err != nil {
		t.Errorf("failed to load %v: %v", filename, err)
	}
	signer, err := seclib.NewInMemorySigner(cpriv)
	if err != nil {
		t.Errorf("failed to create signer for %v: %v", filename, err)
	}
	sig, err := signer.Sign(purpose, message)
	if err != nil {
		t.Errorf("failed to sign using %v: %v", filename, err)
	}
	if !sig.Verify(signer.PublicKey(), message) {
		t.Errorf("failed to verify signature using %v: %v", filename, err)
	}

	pastTime, _ := time.Parse("2006-Jan-02", "2021-Nov-02")
	opts := x509.VerifyOptions{
		Roots:       customCertPool(t, filepath.Join("testdata", "letsencrypt-stg-int-e1.pem")),
		CurrentTime: pastTime,
	}
	cert, err := seclib.ParseX509CertificateFile(filename, opts)
	if err != nil {
		t.Fatalf("failed to load %v: %v", filename, err)
	}

	// openssl x509 -in testdata/lwww.labdrive.io.letsencrypt --pubkey --noout |
	// openssl ec --pubin --inform PEM --outform DER |openssl md5 -c
	if got, want := cert.PublicKey.String(), "b4:1c:fc:66:5a:60:66:ea:e1:c5:46:76:59:8c:fc:6a"; got != want {
		t.Errorf("%v: got %v, want %v", filename, got, want)
	}

	// Now parse the root certificate also.
	cert, err = seclib.ParseX509CertificateFile(
		filepath.Join("testdata", "letsencrypt-stg-int-e1.pem"), opts)
	if err != nil {
		t.Fatalf("failed to load %v: %v", filename, err)
	}
	// openssl x509 -in testdata/letsencrypt-stg-int-e1.pem --pubkey --noout |
	// openssl ec --pubin --inform PEM --outform DER |openssl md5 -c
	if got, want := cert.PublicKey.String(), "8d:49:53:4b:8c:e3:7a:d5:e0:69:95:18:49:1f:7b:bf"; got != want {
		t.Errorf("%v: got %v, want %v", filename, got, want)
	}

}

func sslPrinicipal(filename string) (security.Principal, crypto.PrivateKey, []*x509.Certificate, error) {
	key, err := seclib.ParsePEMPrivateKeyFile(filename, nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load %v: %v", filename, err)
	}
	signer, err := seclib.NewInMemorySigner(key)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create signer for %v: %v", filename, err)
	}
	p, err := seclib.NewPrincipalFromSigner(signer)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create new principal: %v", err)
	}

	blocks, err := internal.LoadCABundlePEMFile(filename)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to load CA Bundle %v", err)
	}

	certs, err := internal.ParseX509Certificates(blocks)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse x509 certs: %v", err)
	}
	return p, key, certs, nil
}

func TestRootCABlessings(t *testing.T) {
	filename := filepath.Join("testdata", "www.labdrive.io.letsencrypt")
	p, privKey, certs, err := sslPrinicipal(filename)
	if err != nil {
		t.Fatal(err)
	}

	for _, c := range certs {
		fmt.Printf("%v: %T: %v .. %v\n", c.Issuer, c.PublicKey, c.NotBefore, c.NotAfter)
	}

	ours := certs[0]
	pastTime, _ := time.Parse("2006-Jan-02", "2021-Nov-02")

	pk := privKey.(*ecdsa.PrivateKey)

	// system pool would be better than ca bundle since it's less
	// configuration for the user. think about how to achieve that if this
	// approach works at all.
	opts := x509.VerifyOptions{
		Roots:       customCertPool(t, filepath.Join("testdata", "letsencrypt-stg-int-e1.pem")),
		CurrentTime: pastTime,
	}
	chains, err := ours.Verify(opts)
	if err != nil {
		t.Fatalf(".... %v", err)
	}

	/*	for i, cc := range chains {
		for _, c := range cc {
			fmt.Printf("%v: %v: %T: %v .. %v\n", i, c.Issuer, c.PublicKey, c.NotBefore, c.NotAfter)
			fmt.Printf("DNS %v\n", c.DNSNames)
			fmt.Printf("Subject %v\n", c.Subject.CommonName)
			fmt.Printf("Issuer %v\n", c.Issuer.CommonName)
		}
	}*/
	pubKey, err := security.NewPublicKey(&pk.PublicKey)
	if err != nil {
		t.Fatalf(".... %v", err)
	}
	blessings, err := security.BlessingsForX509Chains(pubKey, chains)
	if err != nil {
		t.Fatalf(".... %v", err)
	}
	p.BlessingStore().SetDefault(blessings)

	fmt.Printf("%v\n", p.BlessingStore().DebugString())

	/*
		root := chains[0][len(chains[0])-1]

		rootPublicKey, err := seclib.NewPublicKey(root.PublicKey)
		if err != nil {
			t.Fatalf(".... %v", err)
		}
		rootBlessing, err := security.NamelessBlessing(rootPublicKey)
		if err != nil {
			t.Fatalf(".... %v", err)
		}



		fmt.Printf("RB ... %v\n", rootBlessing.PublicKey())

		vcerts, err := internal.ChainFromX509(chains[0])
		if err != nil {
			t.Fatalf(".... %v", err)
		}
		_ = vcerts

		cav, _ := security.NewExpiryCaveat(time.Now().Add(time.Hour))
		blessings, err := p.BlessSelf("www.labdrive.io", cav)
		if err != nil {
			t.Errorf("failed to bless self: %v", err)
		}
		fmt.Printf("B: %s\n", blessings.String())
	*/
	t.FailNow()

}

type testService struct{}

func (testService) Echo(ctx *context.T, call rpc.ServerCall, msg string) (string, error) {
	return "echo: " + msg, nil
}

func newCtxPrincipal(rootCtx *context.T) *context.T {
	ctx, err := v23.WithPrincipal(rootCtx, testutil.NewPrincipal("defaultBlessings"))
	if err != nil {
		panic(err)
	}
	return ctx
}

func TestRPC(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	filename := filepath.Join("testdata", "www.labdrive.io.letsencrypt")
	serverPrincipal, _, _, err := sslPrinicipal(filename)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Printf("SP 1:\n%v\n", serverPrincipal.BlessingStore().DebugString())
	//serverPrincipal = testutil.NewPrincipal("oh")
	fmt.Printf("SP 2:\n %v\n", serverPrincipal.BlessingStore().DebugString())
	serverCtx, err := v23.WithPrincipal(ctx, serverPrincipal)
	if err != nil {
		t.Fatal(err)
	}
	//	serverCtx = newCtxPrincipal(ctx)

	_, server, err := v23.WithNewServer(serverCtx, "", testService{}, security.AllowEveryone())
	if err != nil {
		t.Fatal(err)
	}

	testutil.WaitForServerReady(server)
	serverObjectName := server.Status().Endpoints[0].Name()

	clientCtx, err := v23.WithPrincipal(ctx, testutil.NewPrincipal("simple"))
	if err != nil {
		t.Fatal(err)
	}

	clientCtx, client, err := v23.WithNewClient(clientCtx)
	if err != nil {
		panic(err)
	}

	fmt.Printf("startCall\n")
	call, err := client.StartCall(ctx, serverObjectName, "Echo", []interface{}{"hi"})
	if err == nil || !strings.Contains(err.Error(), "client does not trust server") {
		t.Errorf("expected an error saying that the client does not trust the server, not: %v", err)
	}
	var result string
	err = call.Finish(&result)

	fmt.Printf("RESULT... %v: %q\n", err, result)
	t.Log(err)
	t.FailNow()
}
