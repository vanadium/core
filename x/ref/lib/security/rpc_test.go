// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	gocontext "context"
	"crypto/x509"
	"runtime"
	"strings"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/test"
	"v.io/x/ref/test/sectestdata"
	"v.io/x/ref/test/testutil"
)

type testService struct{}

func (testService) Echo(ctx *context.T, call rpc.ServerCall, msg string) (string, error) {
	return "echo: " + msg, nil
}

func TestX509RPC(t *testing.T) {
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
	signer, err := seclib.NewSignerFromKey(ctx, privKey)
	fatal()
	serverPrincipal, err := seclib.CreatePrincipalOpts(ctx,
		seclib.WithSigner(signer),
		seclib.WithX509Certificate(pubCerts[0]),
		seclib.WithX509VerifyOptions(opts))
	fatal()

	blessingsFoo, err := serverPrincipal.BlessSelf("foo.labdrive.io")
	fatal()
	blessingsBar, err := serverPrincipal.BlessSelf("bar.labdr.io")
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
	clientPrincipal, err := newClientPrincipal(ctx, keys.ED25519, x509.VerifyOptions{}, serverPrincipal, blessingsBar, "client", "bar.labdr.io")
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
	clientPrincipal, err = newClientPrincipal(ctx, keys.ED25519, opts, serverPrincipal, blessingsBar, "client", "bar.labdr.io")
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

func newClientPrincipal(ctx gocontext.Context, kt keys.CryptoAlgo, opts x509.VerifyOptions, serverPrincipal security.Principal, serverBlessings security.Blessings, extension string, pattern security.BlessingPattern) (security.Principal, error) {
	signer := sectestdata.V23Signer(kt, sectestdata.V23KeySetA)

	clientPrincipal, err := seclib.CreatePrincipalOpts(ctx,
		seclib.WithSigner(signer),
		seclib.WithX509VerifyOptions(opts))
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
