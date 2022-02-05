// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package test

import (
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/x/lib/gosh"
	"v.io/x/ref/test"
	"v.io/x/ref/test/compatibility"
	"v.io/x/ref/test/compatibility/modules/simple/impl"
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
	runClient(t, cltCmd, 2)
	srvCmd.Terminate(os.Interrupt)
	mtCmd.Terminate(os.Interrupt)
}

func runClient(t *testing.T, cltCmd *v23test.Cmd, numCalls int) {
	cltCmd.PropagateOutput = true
	cltCmd.Start()
	for i := 0; i < numCalls; i++ {
		if got, want := cltCmd.S.ExpectVar("RESPONSE"), "v120:TestV23PriorServers"; got != want {
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

func TestV23PriorClients(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)

	// Run current servers against previous client.

	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	buildV120(t, sh.Ctx, v23test.BinDir())

	stopMT := sh.StartRootMountTable()

	srvCmd := sh.FuncCmd(simpleServerCmd, "v120")
	srvCmd.PropagateOutput = true
	srvCmd.Start()
	srvCmd.S.ExpectVar("PID")
	srvCmd.S.ExpectVar("NAME")

	ns := v23.GetNamespace(sh.Ctx)
	mt := ns.Roots()[0]
	numCalls := 2
	cltCmd := sh.Cmd(simpleClientBinary,
		"--v23.namespace.root="+mt,
		"--name=v120",
		"--num-calls="+strconv.Itoa(numCalls),
		"--message=TestV23PriorServers")
	runClient(t, cltCmd, 2)
	srvCmd.Terminate(os.Interrupt)
	stopMT(os.Interrupt)
}

var simpleServerCmd = gosh.RegisterFunc("simpleServer", func(name string) error {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	return impl.RunServer(ctx, name)
})

var simpleClientCmd = gosh.RegisterFunc("simpleClient", func(mt, name, msg string, numCalls int) error {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	ctx, _, err := v23.WithNewNamespace(ctx, mt)
	if err != nil {
		return err
	}
	return impl.RunClient(ctx, name, msg, numCalls)
})
