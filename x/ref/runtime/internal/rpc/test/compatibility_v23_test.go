// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package test

import (
	"crypto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/lib/gosh"
	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/test"
	"v.io/x/ref/test/compatibility"
	"v.io/x/ref/test/compatibility/modules/simple/impl"
	"v.io/x/ref/test/sectestdata"
	"v.io/x/ref/test/v23test"
)

var (
	simpleClientBinary string
	simpleServerBinary string
	mounttableBinary   string
	buildOnce          sync.Once
)

func buildV120(t *testing.T, ctx *context.T, bindir string) {
	var err error
	var cleanup func()
	buildOnce.Do(func() {
		simpleClientBinary, simpleServerBinary, mounttableBinary, cleanup, err = compatibility.BuildSimpleAtVersion(ctx, "v0.1.20", bindir, testing.Verbose())
		cleanup()
		if err != nil {
			t.Fatalf("failed to buld binaries: %v", err)
		}
	})
	t.Logf("client     : %s", simpleClientBinary)
	t.Logf("server     : %s", simpleServerBinary)
	t.Logf("mounttable : %s", mounttableBinary)
}

func TestV23PriorServers(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)

	// Run current client against previous mounttable and server.

	ctx, cancel := context.RootContext()
	defer cancel()
	buildV120(t, ctx, v23test.BinDir())

	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	mtCmd := sh.Cmd(mounttableBinary, "--v23.tcp.address=127.0.0.1:0")
	mtCmd.Start()
	mt := mtCmd.S.ExpectVar("NAME")

	srvCmd := sh.Cmd(simpleServerBinary,
		"--v23.namespace.root="+mt,
		"--v23.tcp.address=127.0.0.1:0",
		"--name=v120")
	srvCmd.Start()
	srvCmd.S.ExpectVar("PID")
	srvCmd.S.ExpectVar("NAME")

	numCalls := 2
	cltCmd := sh.FuncCmd(simpleClientCmd,
		mt,
		"v120",
		"TestV23PriorServers",
		numCalls,
	)
	runClient(t, cltCmd, "v120", "TestV23PriorServers", 2)
	srvCmd.Terminate(os.Interrupt)
	mtCmd.Terminate(os.Interrupt)
}

func runClient(t *testing.T, cltCmd *v23test.Cmd, name, message string, numCalls int) {
	cltCmd.PropagateOutput = true
	cltCmd.Start()
	for i := 0; i < numCalls; i++ {
		if got, want := cltCmd.S.ExpectVar("RESPONSE"), name+":"+message; got != want {
			t.Errorf("got %v, want %v", got, want)
			break
		}
		tmp := cltCmd.S.ExpectVar("TOOK")
		took, err := time.ParseDuration(tmp)
		if err != nil {
			t.Errorf("failed to parse time: %v: %v", tmp, err)
			break
		}
		if got, want := took, 2*time.Minute; got > want {
			t.Errorf("client call took %v which is longer than %v", got, want)
			break
		}
	}
	if got, want := cltCmd.S.ExpectVar("DONE"), "OK"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	cltCmd.S.ExpectEOF()
	cltCmd.Wait()
}

func runCurrentServer(sh *v23test.Shell, cmd *gosh.Func, args ...interface{}) *v23test.Cmd {
	srvCmd := sh.FuncCmd(cmd, args...)
	srvCmd.PropagateOutput = true
	srvCmd.Start()
	srvCmd.S.ExpectVar("PID")
	srvCmd.S.ExpectVar("NAME")
	return srvCmd
}

func clientCmd(sh *v23test.Shell, binary string, name, message string, numCalls int) *v23test.Cmd {
	ns := v23.GetNamespace(sh.Ctx)
	mt := ns.Roots()[0]
	return sh.Cmd(binary,
		"--v23.namespace.root="+mt,
		"--name="+name,
		"--num-calls="+strconv.Itoa(numCalls),
		"--message="+message)
}

func TestV23PriorClients(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)

	// Run current servers against previous client.

	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	buildV120(t, sh.Ctx, v23test.BinDir())

	stopMT := sh.StartRootMountTable()

	srvCmd := runCurrentServer(sh, simpleServerCmd, "v120")
	cltCmd := clientCmd(sh, simpleClientBinary, "v120", "PriorClients", 2)
	runClient(t, cltCmd, "v120", "PriorClients", 2)
	srvCmd.Terminate(os.Interrupt)
	stopMT(os.Interrupt)
}

func createPersistentPrincipal(ctx *context.T, t *testing.T, name, dir string, key crypto.PrivateKey) security.Principal {
	store, err := seclib.CreateFilesystemStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	p, err := seclib.CreatePrincipalOpts(ctx, seclib.UseStore(store), seclib.UsePrivateKey(key, nil))
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.BlessSelf(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := seclib.SetDefaultBlessings(p, b); err != nil {
		t.Fatal(err)
	}
	// Add the public key of the root shell so that the mounttable and any other
	// services started by it will be recognized by the new principals we create
	// here.
	defaultBlessing, _ := v23.GetPrincipal(ctx).BlessingStore().Default()
	if err := security.AddToRoots(p, defaultBlessing); err != nil {
		t.Fatal(err)
	}
	return p
}

func createCredentialsDirectories(ctx *context.T, t *testing.T, dir string, clientKeyAlgo keys.CryptoAlgo, serverKeyAlgos ...keys.CryptoAlgo) {

	clientName := "client-" + clientKeyAlgo.String()
	clientKey := sectestdata.V23PrivateKey(clientKeyAlgo, sectestdata.V23KeySetA)
	client := createPersistentPrincipal(ctx, t, clientName, filepath.Join(dir, clientName), clientKey)

	for _, kt := range serverKeyAlgos {
		serverBlessingName := "server-" + kt.String()
		serverKey := sectestdata.V23PrivateKey(kt, sectestdata.V23KeySetB)
		server := createPersistentPrincipal(ctx, t, serverBlessingName, filepath.Join(dir, "server-"+kt.String()), serverKey)

		serverDefault, _ := server.BlessingStore().Default()

		cav, err := security.NewExpiryCaveat(time.Now().Add(24 * time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		blessing, err := server.Bless(client.PublicKey(), serverDefault, clientName, cav)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.BlessingStore().Set(blessing, security.BlessingPattern(serverBlessingName)); err != nil {
			t.Fatal(err)
		}
		if err := security.AddToRoots(client, blessing); err != nil {
			t.Fatal(err)
		}
	}
}

func clientCmdCreds(sh *v23test.Shell, binary string, name, message, credentials string, numCalls int) *v23test.Cmd {
	ns := v23.GetNamespace(sh.Ctx)
	mt := ns.Roots()[0]
	return sh.Cmd(binary,
		"--v23.credentials="+credentials,
		"--v23.namespace.root="+mt,
		"--name="+name,
		"--num-calls="+strconv.Itoa(numCalls),
		"--message="+message)
}

func TestV23AllSigningAlgos(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	buildV120(t, sh.Ctx, v23test.BinDir())

	stopMT := sh.StartRootMountTable()

	for _, clientKeyType := range sectestdata.SupportedKeyAlgos {
		credsDir := t.TempDir()
		clientCreds := filepath.Join(credsDir, "client-"+clientKeyType.String())
		createCredentialsDirectories(sh.Ctx, t, credsDir, clientKeyType, sectestdata.SupportedKeyAlgos...)
		for _, serverKeyType := range sectestdata.SupportedKeyAlgos {
			serverCreds := filepath.Join(credsDir, "server-"+serverKeyType.String())

			name := clientKeyType.String() + "-" + serverKeyType.String()
			srvCmd := runCurrentServer(sh, simpleServerCmdWithPrincipal, name, serverCreds)
			cltCmd := sh.FuncCmd(simpleClientCmdWithPrincipal, name, "AllSigningAlgos", clientCreds, 2)
			runClient(t, cltCmd, name, "AllSigningAlgos", 2)
			srvCmd.Terminate(os.Interrupt)
		}
	}

	stopMT(os.Interrupt)
}

func TestV23PriorClientsAgainstNewSigningAlgos(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)

	// Run current servers with different signing algorithms against previous client.

	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	buildV120(t, sh.Ctx, v23test.BinDir())

	stopMT := sh.StartRootMountTable()

	// Generate new principals using the current code, but only with backwards compatible
	// key types. Any attempt to communicate with an ED25519 server will result in a protocol
	// error.
	compatKeyDir := t.TempDir()
	name := "PriorClientsAgainstNewSigningAlgos-working"
	createCredentialsDirectories(sh.Ctx, t, compatKeyDir, keys.ECDSA256, keys.ECDSA256, keys.ED25519)
	serverCreds := filepath.Join(compatKeyDir, "server-"+keys.ECDSA256.String())
	clientCreds := filepath.Join(compatKeyDir, "client-"+keys.ECDSA256.String())
	srvCmd := runCurrentServer(sh, simpleServerCmdWithPrincipal, name, serverCreds)
	cltCmd := clientCmdCreds(sh, simpleClientBinary, name, "PriorWorking", clientCreds, 2)
	runClient(t, cltCmd, name, "PriorWorking", 2)
	srvCmd.Terminate(os.Interrupt)

	// An old client against an ED25519 server will result in a client side
	// 'unknown message error'.
	sh.ContinueOnError = true

	name = "PriorClientsAgainstNewSigningAlgos-bad-message"
	serverCreds = filepath.Join(compatKeyDir, "server-"+keys.ED25519.String())
	srvCmd = runCurrentServer(sh, simpleServerCmdWithPrincipal, name, serverCreds)
	cltCmd = clientCmdCreds(sh, simpleClientBinary, name, "PriorUnknownMessage", clientCreds, 2)

	_, stderr := cltCmd.StdoutStderr()
	if !strings.Contains(stderr, "unknown message type:") {
		t.Fatalf("missing or unexpected error/failure message: %v", stderr)
	}
	sh.Err = nil
	srvCmd.Terminate(os.Interrupt)

	// The rsa public key will cause the client to fail on startup since rsa is not supported
	// by the v120 releases. More recent releases will behave in the same way as the ED25519 case.
	incompatKeyDir := t.TempDir()
	createCredentialsDirectories(sh.Ctx, t, incompatKeyDir, keys.ECDSA256, keys.ECDSA256, keys.RSA2048)
	name = "PriorClientsAgainstNewSigningAlgos-no-load"

	clientCreds = filepath.Join(incompatKeyDir, "client-"+keys.ECDSA256.String())
	cltCmd = clientCmdCreds(sh, simpleClientBinary, name, "PriorLoadError", clientCreds, 2)

	_, stderr = cltCmd.StdoutStderr()
	if !strings.Contains(stderr, "failed to load BlessingStore: unrecognized PublicKey type *rsa.PublicKey") {
		t.Fatalf("missing or unexpected error/failure message: %v", stderr)
	}

	sh.Err = nil
	stopMT(os.Interrupt)
}

var simpleServerCmd = gosh.RegisterFunc("simpleServer", func(name string) error {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	return impl.RunServer(ctx, name)
})

var simpleServerCmdWithPrincipal = gosh.RegisterFunc("simpleServerCmdWithPrincipal", runServerWithPrincipal)

func runServerWithPrincipal(name, credsDir string) error {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	p, err := seclib.LoadPrincipalOpts(ctx, seclib.LoadFromReadonly(seclib.FilesystemStoreReader(credsDir)))
	if err != nil {
		return err
	}
	ctx, err = v23.WithPrincipal(ctx, p)
	if err != nil {
		return err
	}
	return impl.RunServer(ctx, name)
}

var simpleClientCmd = gosh.RegisterFunc("simpleClient", func(mt, name, msg string, numCalls int) error {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	ctx, _, err := v23.WithNewNamespace(ctx, mt)
	if err != nil {
		return err
	}
	return impl.RunClient(ctx, name, msg, numCalls)
})

var simpleClientCmdWithPrincipal = gosh.RegisterFunc("simpleClientCmdWithPrincipal", func(name, msg, credsDir string, numCalls int) error {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	p, err := seclib.LoadPrincipalOpts(ctx, seclib.LoadFromReadonly(seclib.FilesystemStoreReader(credsDir)))
	if err != nil {
		return err
	}
	ctx, err = v23.WithPrincipal(ctx, p)
	if err != nil {
		return err
	}
	return impl.RunClient(ctx, name, msg, numCalls)
})
