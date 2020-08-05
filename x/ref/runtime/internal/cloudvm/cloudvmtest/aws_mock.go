// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cloudvmtest

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"v.io/x/ref/runtime/internal/cloudvm/cloudpaths"
)

func StartAWSMetadataServer(t *testing.T) (string, func()) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var token string
	http.HandleFunc(cloudpaths.AWSTokenPath, func(w http.ResponseWriter, req *http.Request) {
		token = time.Now().String()
		w.Header().Add("Server", "EC2ws")
		fmt.Fprint(w, token)
	})

	validSession := func(req *http.Request) bool {
		requestToken := req.Header.Get("X-aws-ec2-metadata-token")
		return requestToken == token
	}

	http.HandleFunc(cloudpaths.AWSIdentityDocPath, func(w http.ResponseWriter, r *http.Request) {
		if !validSession(r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Add("Server", "EC2ws")
		id := map[string]interface{}{
			"accountId": WellKnownAccount,
			"region":    WellKnownRegion,
		}
		buf, err := json.Marshal(id)
		if err != nil {
			panic(err)
		}
		w.Write(buf)
	})

	respond := func(w http.ResponseWriter, r *http.Request, format string, args ...interface{}) {
		if !validSession(r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Add("Server", "EC2ws")
		fmt.Fprintf(w, format, args...)
	}

	http.HandleFunc(cloudpaths.AWSPrivateIPPath,
		func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, WellKnownPrivateIP)
		})
	http.HandleFunc(cloudpaths.AWSPublicIPPath,
		func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, WellKnownPublicIP)
		})
	http.HandleFunc(cloudpaths.AWSPublicIPPath+"/noip",
		func(w http.ResponseWriter, r *http.Request) {
			respond(w, r, "")
		})

	go http.Serve(l, nil)
	return "http://" + l.Addr().String(), func() { l.Close() }
}
