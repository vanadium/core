// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"v.io/x/ref/services/http/httplib"

	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	v23 "v.io/v23"
	"v.io/v23/context"
	_ "v.io/x/ref/runtime/factories/roaming"
)

var (
	server   = flag.String("server", "", "Name of the server to connect to")
	endpoint string
	ctx      *context.T
)

func traceDataHandler(w http.ResponseWriter, req *http.Request) {
	var data []byte

	vdlReq := httplib.VDLRequestFromHTTPRequest(req)
	client := v23.GetClient(ctx)
	err := client.Call(ctx, endpoint, "RawDo", []interface{}{vdlReq}, []interface{}{&data})

	if err != nil {
		log.Println(err)
		w.Write([]byte(fmt.Sprintf("%T", err))) //nolint:errcheck
	} else {
		w.Write(data) //nolint:errcheck
	}
}

func findPortAndListen(mux *http.ServeMux) {
	fmtPort := func(port int) string { return fmt.Sprintf("localhost:%d", port) }
	currPort := 3000

	for {
		ln, err := net.Listen("tcp", fmtPort(currPort))
		if err == nil {
			log.Println("Monitoring on " + fmtPort(currPort) + "/debug/requests...")
			defer ln.Close()
			http.Serve(ln, mux) //nolint:errcheck
			break
		}
		currPort++
	}
}

func main() {
	var shutdown v23.Shutdown
	ctx, shutdown = v23.Init()
	defer shutdown()

	if len(*server) == 0 {
		fmt.Println("usage: http --server endpoint [--v23.credentials cred_dir]")
		return
	}

	endpoint = *server + "/__debug/http"

	mux := http.NewServeMux()
	mux.Handle("/debug/events", http.HandlerFunc(traceDataHandler))
	mux.Handle("/debug/requests", http.HandlerFunc(traceDataHandler))
	findPortAndListen(mux)
}
