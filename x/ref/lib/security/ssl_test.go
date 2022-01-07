// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"crypto"
	"crypto/md5"
	"crypto/x509"
	"path/filepath"
	"testing"

	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/test/sectestdata"
)

func publicKeyFingerPrint(t *testing.T, pk crypto.PublicKey) string {
	pkb, err := x509.MarshalPKIXPublicKey(pk)
	if err != nil {
		t.Errorf("failed to marshal public key %v", err)
		return ""
	}
	const hextable = "0123456789abcdef"
	hash := md5.Sum(pkb)
	var repr [md5.Size * 3]byte
	for i, v := range hash {
		repr[i*3] = hextable[v>>4]
		repr[i*3+1] = hextable[v&0x0f]
		repr[i*3+2] = ':'
	}
	return string(repr[:len(repr)-1])
}

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

func TestLetsEncryptKeys(t *testing.T) {
	cpriv, _, opts := sectestdata.LetsEncryptData()
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

	letsencryptDir, err := sectestdata.LetsEncryptDir()
	if err != nil {
		t.Fatal(err)
	}
	//	defer os.RemoveAll(letsencryptDir)
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
	if got, want := publicKeyFingerPrint(t, certs[0].PublicKey), "b4:1c:fc:66:5a:60:66:ea:e1:c5:46:76:59:8c:fc:6a"; got != want {
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
	if got, want := publicKeyFingerPrint(t, certs[0].PublicKey), "8d:49:53:4b:8c:e3:7a:d5:e0:69:95:18:49:1f:7b:bf"; got != want {
		t.Errorf("%v: got %v, want %v", filename, got, want)
	}
}

/*

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
	}

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

	t.FailNow()

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
