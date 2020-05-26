// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !nacl

package websocket

import (
	"bytes"
	"log"
	"net"
	"strings"
	"testing"
	"time"

	"v.io/v23/context"
)

func TestAcceptsAreNotSerialized(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	ln, err := WSH{}.Listen(ctx, "wsh", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	portscan := make(chan struct{})

	// Goroutine that continuously accepts connections.
	go func() {
		for {
			conn, err := ln.Accept(ctx)
			if err != nil {
				return
			}
			defer conn.Close()
		}
	}()

	// Imagine some client was port scanning and thus opened a TCP
	// connection (but never sent the bytes)
	go func() {
		conn, err := net.Dial("tcp", ln.Addr().String())
		if err != nil {
			t.Error(err)
		}
		close(portscan)
		// Keep the connection alive by blocking on a read.  (The read
		// should return once the test exits).
		conn.Read(make([]byte, 1024)) //nolint:errcheck
	}()
	// Another client that dials a legitimate connection should not be
	// blocked on the portscanner.
	// (Wait for the portscanner to establish the TCP connection first).
	<-portscan
	conn, err := WS{}.Dial(ctx, ln.Addr().Network(), ln.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}

func TestNonWebsocketRequest(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	ln, err := WSH{}.Listen(ctx, "wsh", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	// Goroutine that continuously accepts connections.
	go func() {
		for {
			_, err := ln.Accept(ctx)
			if err != nil {
				return
			}
		}
	}()

	var out bytes.Buffer
	log.SetOutput(&out)

	// Imagine some client keeps sending non-websocket requests.
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < 2; i++ {
		conn.Write([]byte("GET / HTTP/1.1\r\n\r\n")) //nolint:errcheck
		conn.Read(make([]byte, 1024))                //nolint:errcheck
	}

	logs := out.String()
	if strings.Contains(logs, "panic") {
		t.Errorf("Unexpected panic:\n%s", logs)
	}
}
