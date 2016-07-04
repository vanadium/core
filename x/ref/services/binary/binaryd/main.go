// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"fmt"
	"net"
	"net/http"
	"os"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/x/lib/cmdline"
	"v.io/x/lib/netstate"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/internal/binarylib"
)

const defaultDepth = 3

var name, rootDirFlag, httpAddr string

func main() {
	cmdBinaryD.Flags.StringVar(&name, "name", "", "Name to mount the binary repository as.")
	cmdBinaryD.Flags.StringVar(&rootDirFlag, "root-dir", "", "Root directory for the binary repository.")
	cmdBinaryD.Flags.StringVar(&httpAddr, "http", ":0", "TCP address on which the HTTP server runs.")

	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdBinaryD)
}

var cmdBinaryD = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runBinaryD),
	Name:   "binaryd",
	Short:  "Runs the binary daemon.",
	Long: `
Command binaryd runs the binary daemon, which implements the
v.io/v23/services/repository.Binary interface.
`,
}

// toIPPort tries to swap in the 'best' accessible IP for the host part of the
// address, if the provided address has an unspecified IP.
func toIPPort(ctx *context.T, addr string) string {
	// TODO(caprita): consider using netstate.PossibleAddresses()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		ctx.Errorf("SplitHostPort(%v) failed: %v", addr, err)
		os.Exit(1)
	}
	ip := net.ParseIP(host)
	if ip.IsUnspecified() {
		host = "127.0.0.1"
		ips, err := netstate.GetAccessibleIPs()
		if err == nil {
			ls := v23.GetListenSpec(ctx)
			if a, err := ls.AddressChooser.ChooseAddresses("tcp", ips.AsNetAddrs()); err == nil && len(a) > 0 {
				host = a[0].String()
			}
		}
	}
	return net.JoinHostPort(host, port)
}

func runBinaryD(ctx *context.T, env *cmdline.Env, args []string) error {
	rootDir, err := binarylib.SetupRootDir(rootDirFlag)
	if err != nil {
		return fmt.Errorf("SetupRootDir(%q) failed: %v", rootDirFlag, err)
	}
	ctx.Infof("Binary repository rooted at %v", rootDir)

	listener, err := net.Listen("tcp", httpAddr)
	if err != nil {
		return fmt.Errorf("Listen(%s) failed: %v", httpAddr, err)
	}
	rootURL := toIPPort(ctx, listener.Addr().String())
	state, err := binarylib.NewState(rootDir, rootURL, defaultDepth)
	if err != nil {
		return fmt.Errorf("NewState(%v, %v, %v) failed: %v", rootDir, rootURL, defaultDepth, err)
	}
	ctx.Infof("Binary repository HTTP server at: %q", rootURL)
	go func() {
		if err := http.Serve(listener, http.FileServer(binarylib.NewHTTPRoot(ctx, state))); err != nil {
			ctx.Errorf("Serve() failed: %v", err)
			os.Exit(1)
		}
	}()

	dis, err := binarylib.NewDispatcher(ctx, state)
	if err != nil {
		return fmt.Errorf("NewDispatcher() failed: %v\n", err)
	}
	ctx, server, err := v23.WithNewDispatchingServer(ctx, name, dis)
	if err != nil {
		return fmt.Errorf("NewServer() failed: %v", err)
	}
	epName := server.Status().Endpoints[0].Name()
	if name != "" {
		ctx.Infof("Binary repository serving at %q (%q)", name, epName)
	} else {
		ctx.Infof("Binary repository serving at %q", epName)
	}
	// Wait until shutdown.
	<-signals.ShutdownOnSignals(ctx)
	return nil
}
