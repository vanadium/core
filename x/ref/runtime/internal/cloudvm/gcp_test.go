// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cloudvm

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"v.io/x/ref/lib/stats"
)

func startGCPMetadataServer(t *testing.T) (net.Addr, func()) {
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
	http.HandleFunc(gcpProjectIDPath, func(w http.ResponseWriter, r *http.Request) {
		respond(w, r, wellKnowAccount)
	})
	http.HandleFunc(gcpZonePath, func(w http.ResponseWriter, r *http.Request) {
		respond(w, r, wellKnowRegion)
	})
	http.HandleFunc(gcpExternalIPPath, func(w http.ResponseWriter, r *http.Request) {
		respond(w, r, wellKnownPrivateIP)
	})
	http.HandleFunc(gcpInternalIPPath, func(w http.ResponseWriter, r *http.Request) {
		respond(w, r, wellKnownPrivateIP)
	})
	http.HandleFunc(gcpExternalIPPath+"/noip", func(w http.ResponseWriter, r *http.Request) {
		respond(w, r, "")
	})
	go http.Serve(l, nil)
	host := "http://" + l.Addr().String()
	gcpExternalURL = host + gcpExternalIPPath
	gcpInternalURL = host + gcpInternalIPPath
	gcpProjectIDURL = host + gcpProjectIDPath
	gcpZoneIDUrl = host + gcpZonePath
	return l.Addr(), func() { l.Close() }
}

func testStats(t *testing.T, idStat, regionStat string) {
	val, err := stats.Value(idStat)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := val.(string), "12345678"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	val, err = stats.Value(regionStat)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := val.(string), "us-west-12"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestGCE(t *testing.T) {
	ctx := context.Background()
	host, stop := startGCPMetadataServer(t)
	defer stop()

	if got, want := OnGCP(ctx, time.Second), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	testStats(t, GCPProjectIDStatName, GCPRegionStatName)

	priv, err := GCPPrivateAddrs(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := priv[0].String(), wellKnownPrivateIP; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	pub, err := GCPPublicAddrs(ctx, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := pub[0].String(), wellKnownPrivateIP; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	externalURL := "http://" + host.String() + gcpExternalIPPath + "/noip"
	noip, err := gcpGetAddr(ctx, externalURL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(noip), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
