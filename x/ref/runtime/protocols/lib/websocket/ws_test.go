// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package websocket_test

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"sync"
	"testing"
	"time"

	"v.io/x/ref/runtime/protocols/lib/tcputil"
	websocket "v.io/x/ref/runtime/protocols/lib/websocket"

	"v.io/v23/context"
	"v.io/v23/flow"
)

func TestWSToWS(t *testing.T) {
	runTest(t, websocket.WS{}, websocket.WS{}, "ws", "ws")
}

func TestWSToWSH(t *testing.T) {
	runTest(t, websocket.WS{}, websocket.WSH{}, "ws", "wsh")
}

func TestWSHToWSH(t *testing.T) {
	runTest(t, websocket.WSH{}, websocket.WSH{}, "wsh", "wsh")
}

func TestTCPToWSH(t *testing.T) {
	runTest(t, tcputil.TCP{}, websocket.WSH{}, "tcp", "wsh")
}

var randData []byte

const (
	chunkSize = 1 << 10
	numChunks = 10
)

func init() {
	randData = make([]byte, chunkSize*numChunks)
	if _, err := rand.Read(randData); err != nil {
		panic(err)
	}
}

func runTest(t *testing.T, dialObj, listenObj flow.Protocol, dialP, listenP string) {
	ctx, cancel := context.RootContext()
	defer cancel()
	address := "127.0.0.1:0"
	timeout := time.Second
	acceptCh := make(chan flow.Conn)

	ln, err := listenObj.Listen(ctx, listenP, address)
	if err != nil {
		t.Fatal(err)
	}
	errCh := make(chan error, 1)
	go func() {
		a, err := ln.Accept(ctx)
		errCh <- err
		acceptCh <- a
	}()

	dialed, err := dialObj.Dial(ctx, dialP, ln.Addr().String(), timeout)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(4)
	errsCh := make(chan error, 4)
	go func() {
		errsCh <- writeData(t, dialed, randData)
		wg.Done()
	}()

	go func() {
		errsCh <- readData(t, dialed, randData)
		wg.Done()
	}()
	accepted := <-acceptCh
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}

	go func() {
		errsCh <- writeData(t, accepted, randData)
		wg.Done()
	}()
	go func() {
		errsCh <- readData(t, accepted, randData)
		wg.Done()
	}()

	wg.Wait()
	close(errsCh)
	for err := range errsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func writeData(t *testing.T, c flow.Conn, data []byte) error {
	for i := 0; i < numChunks; i++ {
		if _, err := c.WriteMsg(data[:chunkSize]); err != nil {
			return err
		}
		data = data[chunkSize:]
	}
	return nil
}

func readData(t *testing.T, c flow.Conn, expected []byte) error {
	read := make([]byte, len(expected))
	read = read[:0]
	for i := 0; i < numChunks; i++ {
		b, err := c.ReadMsg()
		if err != nil {
			return err
		}
		if len(b) != chunkSize {
			return fmt.Errorf("got message of size %v, want %v", len(b), chunkSize)
		}
		read = append(read, b...)
	}
	if !bytes.Equal(read, expected) {
		return fmt.Errorf("read %v, want %v", read, expected)
	}
	return nil
}
