// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cloudvm

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

const (
	wellKnowAccount    = "12345678"
	wellKnowRegion     = "us-west-12"
	wellKnownPrivateIP = "10.10.10.3"
	wellKnowPublicIP   = "8.8.16.32"
)

func startAWSMetadataServer(t *testing.T) (net.Addr, func()) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var token string
	http.HandleFunc(awsTokenPath, func(w http.ResponseWriter, req *http.Request) {
		token = time.Now().String()
		w.Header().Add("Server", "EC2ws")
		fmt.Fprint(w, token)
	})

	validSession := func(req *http.Request) bool {
		requestToken := req.Header.Get("X-aws-ec2-metadata-token")
		return requestToken == token
	}

	http.HandleFunc(awsIdentityDocPath, func(w http.ResponseWriter, r *http.Request) {
		if !validSession(r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Add("Server", "EC2ws")
		id := map[string]interface{}{
			"accountId": wellKnowAccount,
			"region":    wellKnowRegion,
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

	http.HandleFunc(awsPrivateIPPath, func(w http.ResponseWriter, r *http.Request) {
		respond(w, r, wellKnownPrivateIP)
	})
	http.HandleFunc(awsPublicIPPath, func(w http.ResponseWriter, r *http.Request) {
		respond(w, r, wellKnownPrivateIP)
	})
	http.HandleFunc(awsPublicIPPath+"/noip", func(w http.ResponseWriter, r *http.Request) {
		respond(w, r, "")
	})

	go http.Serve(l, nil)

	host := "http://" + l.Addr().String()
	awsTokenURL = host + awsTokenPath
	awsIdentityDocURL = host + awsIdentityDocPath
	awsExternalURL = host + awsPublicIPPath
	awsInternalURL = host + awsPrivateIPPath
	return l.Addr(), func() { l.Close() }
}

func TestAWS(t *testing.T) {
	ctx := context.Background()
	host, stop := startAWSMetadataServer(t)
	defer stop()

	if got, want := OnAWS(ctx, time.Second), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	testStats(t, AWSAccountIDStatName, AWSRegionStatName)

	priv, err := AWSPrivateAddrs(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := priv[0].String(), wellKnownPrivateIP; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	pub, err := AWSPublicAddrs(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pub[0].String(), wellKnownPrivateIP; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	externalURL := "http://" + host.String() + awsPublicIPPath + "/noip"
	noip, err := awsGetAddr(ctx, externalURL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(noip), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
