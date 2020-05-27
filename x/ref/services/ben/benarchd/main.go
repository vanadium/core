// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc . -help

package main

import (
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/x/lib/cmdline"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/roaming"
	"v.io/x/ref/services/ben/benarchd/internal"
	"v.io/x/ref/services/internal/dbutil"
)

var (
	flagStore          string
	flagHTTPAddr       string
	flagPublicHTTPAddr string
	flagName           string
	flagAssets         string
	cmdRoot            = &cmdline.Command{
		Runner: v23cmd.RunnerFunc(run),
		Name:   "benarchd",
		Short:  "Runs the daemon that archives benchmark results",
		Long: `
Command benarchd implements a benchmark archival daemon.

It provides a Vanadium RPC service to accept results to archive (typically
provided by the benup tool - https://godoc.org/v.io/x/ref/services/ben/benup)
and a web-interface to browse the results.

A SQL database is used for persistent storage, configured via the --store
flag.
`,
	}
)

func run(ctx *context.T, env *cmdline.Env, args []string) error {
	// Initialize assets to be used by the web interface
	assets, err := internal.NewAssets(flagAssets)
	if err != nil {
		return err
	}

	// Initialize the store
	driver, dataSource, err := parseStore(ctx, flagStore)
	if err != nil {
		return err
	}
	var db *sql.DB
	switch driver {
	case "sqlconfig":
		// I'll admit, I have no clude of SERIALIZABLE is the right choice.
		db, err = dbutil.NewSQLDBConnFromFile(dataSource, "SERIALIZABLE")
	default:
		db, err = sql.Open(driver, dataSource)
	}
	if err != nil {
		return err
	}
	defer db.Close()
	store, err := internal.NewSQLStore(driver, db)
	if err != nil {
		return err
	}

	// Start the HTTP service
	ln, err := net.Listen("tcp", flagHTTPAddr)
	if err != nil {
		return err
	}
	ctx.Infof("HTTP server at http://%v", ln.Addr())
	go http.Serve(ln, internal.NewHTTPHandler(assets, store)) //nolint:errcheck

	// Start the v23 RPC service
	pubAddr := fmt.Sprintf("http://%v", ln.Addr())
	if flagPublicHTTPAddr != "" {
		pubAddr = flagPublicHTTPAddr
	}
	_, v23server, err := v23.WithNewServer(ctx, flagName, internal.NewArchiver(store, pubAddr+"/?q="), internal.Authorizer())
	if err != nil {
		return err
	}
	if eps := v23server.Status().Endpoints; len(eps) > 0 {
		ctx.Infof("v23 server at %v", eps[0].Name())
	}

	<-signals.ShutdownOnSignals(ctx)
	return nil
}

func parseStore(ctx *context.T, spec string) (driver, dataSource string, err error) {
	if len(spec) == 0 {
		spec = fmt.Sprintf("sqlite3:%v/benarchd.db", os.TempDir())
		ctx.Infof("No --store specified, using %q", spec)
	}
	pos := strings.Index(spec, ":")
	if pos < 0 {
		return "", "", fmt.Errorf("invalid --store, must be in the format <engine>:<parameters>")
	}
	return spec[:pos], spec[pos+1:], nil
}

func main() {
	cmdline.HideGlobalFlagsExcept()
	cmdRoot.Flags.StringVar(&flagName, "name", "", "Vanadium object name to export this service under")
	cmdRoot.Flags.StringVar(&flagStore, "store", "", "Specification of the persistent store to use. Format: <engine>:<parameters>, where <engine> can be 'sqlite3', 'mysql' or 'sqlconfig'. For 'sqlconfig', <parameters> is a path to a file containing a JSON-serialized SqlConfig (https://godoc.org/v.io/x/lib/dbutil#SqlConfig), for the others it is the data source name (e.g., filename for sqlite3)")
	cmdRoot.Flags.StringVar(&flagPublicHTTPAddr, "exthttp", "", "The address of the HTTP server to advertise externally, typically used if the HTTP server is running behind a proxy or on a machine that is unaware of its publicly accessible hostname/IP address. If empty, derived from --http.")
	cmdRoot.Flags.StringVar(&flagHTTPAddr, "http", "127.0.0.1:0", "Address on which to serve HTTP requests")
	cmdRoot.Flags.StringVar(&flagAssets, "assets", "", "If set, the directory containing assets (template definitions, css, javascript files etc.) to use in the web interface. If not set, compiled-in assets will be used instead.")
	cmdline.Main(cmdRoot)
}
