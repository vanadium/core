// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/namespace"
	"v.io/v23/naming"
	"v.io/x/lib/gosh"
	"v.io/x/ref/runtime/factories/library"
	"v.io/x/ref/test"
	"v.io/x/ref/test/v23test"
)

var (
	localHostName string
	serverName    = "virtualized-server"
)

func init() {
	library.AllowMultipleInitializations = true
	library.CloudVM = true
	var err error
	localHostName, err = os.Hostname()
	if err != nil {
		panic(err)
	}
}

func assertWithLogs(t *testing.T, err error, msg string, logs []*bytes.Buffer) {
	if err == nil {
		return
	}
	_, file, line, _ := runtime.Caller(2)
	loc := fmt.Sprintf("%v:%v", filepath.Base(file), line)
	for i, log := range logs {
		t.Logf("%v: start log from: %s:%v ------", loc, msg, i)
		t.Log(log.String())
		t.Logf("%v:   end log from: %s:%v ------", loc, msg, i)
	}
	t.Fatalf("%v: %s: %s", loc, msg, err)
}

func TestV23Virtualized(t *testing.T) {
	v23test.SkipUnlessRunningIntegrationTests(t)
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()
	sh.StartRootMountTable()

	ns := v23.GetNamespace(sh.Ctx)
	ns.CacheCtl(naming.DisableCache(true))

	// Test a server with only a public dns name and make sure that
	// the client code can correctly resolve that name and RPC to the
	// server.
	si := startServer(t, sh,
		"--name="+serverName,
		"--v23.virtualized.advertise-private-addresses=false",
		"--v23.virtualized.dns.public-name="+localHostName)

	var err error
	assert := func(msg string, logs ...*bytes.Buffer) {
		assertWithLogs(t, err, msg, logs)
	}
	// Make sure localHostName is the list of servers addresses.
	servers, err := waitForMountedServersIncluding(t, sh.Ctx, ns, serverName, localHostName)
	assert("wait-for-server", si.log)
	// Make sure localHostName is the only server address.
	if got, want := len(servers), 1; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	// Make sure that the client can resolve the dnsname supplied.
	log, err := runClient(sh, clientProcess)
	assert("run-client", log)

	// Kill the server.
	si.cmd.Terminate(os.Interrupt)
	err = waitForNoMountedServers(t, sh.Ctx, ns, serverName)
	assert("waiting for server to be removed from the mounttable", log)

	// Run a server with private addresses exported.
	si = startServer(t, sh,
		"--name="+serverName,
		"--v23.virtualized.advertise-private-addresses=true",
		"--v23.virtualized.dns.public-name="+localHostName)

	_, err = waitForMountedServersIncluding(t, sh.Ctx, ns, serverName, localHostName)
	assert("wait-for-server-with-dns-name", si.log)
	servers, err = waitForMountedServersIncluding(t, sh.Ctx, ns, serverName, "127.0.0.1")
	assert("wait-for-server-with-localhost", si.log)

	// Make sure there is at least one private address.
	if got, want := len(servers), 2; got < want {
		t.Errorf("got %v, want >= %v", got, want)
	}

	// Make sure the dns name appears only once.
	found := 0
	for _, server := range servers {
		ep, err := naming.ParseEndpoint(server.Server)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(ep.Address, localHostName) {
			found++
		}
	}
	// expect the dns name to be present only once.
	if got, want := found, 1; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func runClient(sh *v23test.Shell, fn *gosh.Func) (*bytes.Buffer, error) {
	log := &bytes.Buffer{}
	cmd := sh.FuncCmd(fn)
	cmd.AddStderrWriter(log)
	cmd.AddStdoutWriter(log)
	cmd.Run()
	if got, want := cmd.S.ExpectVar("RESPONSE"), "ping: "+localHostName; got != want {
		return log, fmt.Errorf("Got %q, want %q", got, want)
	}
	return log, nil
}

var clientProcess = gosh.RegisterFunc("clientProcess",
	func() error {
		ctx, shutdown := test.V23Init()
		defer shutdown()
		var response string
		if err := v23.GetClient(ctx).Call(ctx, serverName, "Ping", []interface{}{localHostName}, []interface{}{&response}); err != nil {
			return err
		}
		fmt.Printf("RESPONSE=%v\n", response)
		return nil
	})

type serverInfo struct {
	cmd *v23test.Cmd
	log *bytes.Buffer
}

func startServer(t *testing.T, sh *v23test.Shell, args ...string) *serverInfo {
	var (
		binary = v23test.BuildGoPkg(sh, "v.io/x/ref/runtime/internal/cmd/cloudvm")
		log    = &bytes.Buffer{}
	)
	cmd := sh.Cmd(binary, args...)
	cmd.AddStdoutWriter(log)
	cmd.AddStderrWriter(log)
	cmd.Start()
	return &serverInfo{
		cmd: cmd,
		log: log,
	}
}

func waitForNoMountedServers(t *testing.T, ctx *context.T, ns namespace.T, name string) error {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	for {
		if _, err := ns.Resolve(ctx, name); err != nil {
			if errors.Is(err, naming.ErrNoSuchName) {
				return nil
			}
			return err
		}
		if time.Since(start) > time.Second*5 {
			break
		}
		time.Sleep(time.Millisecond * 200)
	}
	return fmt.Errorf("timed out after %v waiting for %v to be removed", time.Since(start), name)
}

func waitForMountedServersIncluding(t *testing.T, ctx *context.T, ns namespace.T, name, including string) ([]naming.MountedServer, error) {
	start := time.Now()
	for {
		cctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		resolved, err := ns.Resolve(cctx, name)
		if err != nil {
			if errors.Is(err, naming.ErrNoSuchName) {
				time.Sleep(time.Millisecond * 200)
				continue
			}
			return nil, err
		}
		if len(resolved.Servers) > 0 {
			if len(including) == 0 {
				return resolved.Servers, nil
			}
			for _, server := range resolved.Servers {
				ep, err := naming.ParseEndpoint(server.Server)
				if err != nil {
					return nil, err
				}
				if strings.Contains(ep.Address, including) {
					return resolved.Servers, nil
				}
			}
		}
		if time.Since(start) > time.Second*5 {
			break
		}
		time.Sleep(time.Millisecond * 200)
	}
	return nil, fmt.Errorf("timed out after %v waiting for %v", time.Since(start), name)
}

func TestMain(m *testing.M) {
	v23test.TestMain(m)
}
