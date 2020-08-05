// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cloudvmtest

import (
	"fmt"
	"net"
	"net/http"
	"testing"

	"v.io/x/ref/runtime/internal/cloudvm/cloudpaths"
)

func StartGCPMetadataServer(t *testing.T) (string, func()) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	respond := func(w http.ResponseWriter, r *http.Request, format string, args ...interface{}) {
		w.Header().Add("Metadata-Flavor", "Google")
		if m := r.Header["Metadata-Flavor"]; len(m) != 1 || m[0] != "Google" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		fmt.Fprintf(w, format, args...)
	}
	http.HandleFunc(cloudpaths.GCPProjectIDPath,
		func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, WellKnownAccount)
		})
	http.HandleFunc(cloudpaths.GCPZonePath,
		func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, WellKnownRegion)
		})
	http.HandleFunc(cloudpaths.GCPExternalIPPath,
		func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, WellKnownPublicIP)
		})
	http.HandleFunc(cloudpaths.GCPInternalIPPath,
		func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, WellKnownPrivateIP)
		})
	http.HandleFunc(cloudpaths.GCPExternalIPPath+"/noip",
		func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, "")
		})
	go http.Serve(l, nil)
	return "http://" + l.Addr().String(), func() { l.Close() }
}
