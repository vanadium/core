// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ipc

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/agent"
)

type echo int

func (e echo) Echo(m string) (string, error) {
	return m, nil
}

type delayedEcho struct {
	gotA, gotB   chan struct{}
	doneA, doneB chan string
}

func newDelayedEcho() delayedEcho {
	return delayedEcho{make(chan struct{}, 1), make(chan struct{}, 1), make(chan string, 1), make(chan string, 1)}
}

func (d delayedEcho) EchoA() (string, error) {
	close(d.gotA)
	return <-d.doneA, nil
}

func (d delayedEcho) EchoB(prefix string) (string, error) {
	close(d.gotB)
	return prefix + <-d.doneB, nil
}

type returnError int

func (returnError) BreakMe(a, b string) (int, error) {
	return 0, fmt.Errorf("%s %s", a, b)
}

type tunnel struct {
	ipc *IPC
}

func (t *tunnel) Tunnel() (str string, err error) {
	conn := t.ipc.Connections()[0]
	err = conn.Call("Echo", []interface{}{"Tunnel"}, &str)
	return
}

func newServer(t *testing.T, servers ...interface{}) (ipc *IPC, path string, f func()) {
	dir, err := ioutil.TempDir("", "sock")
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(dir, "sock")
	ipc = NewIPC()
	for _, s := range servers {
		if err := ipc.Serve(s); err != nil {
			t.Fatal(err)
		}
	}
	if err := ipc.Listen(path); err != nil {
		os.RemoveAll(dir)
		t.Fatal(err)
	}
	return ipc, path, func() { ipc.Close(); os.RemoveAll(dir) }
}

func isEpipe(err error) bool {
	for operr, ok := err.(*net.OpError); ok; operr, ok = err.(*net.OpError) {
		err = operr.Err
	}
	return err == syscall.EPIPE
}

func TestDoubleServe(t *testing.T) {
	ipc := NewIPC()
	var a echo
	var b echo
	if err := ipc.Serve(a); err != nil {
		t.Fatal(err)
	}
	if err := ipc.Serve(b); err == nil {
		t.Fatal("Expected error")
	}
}

func TestListen(t *testing.T) {
	ipc1, path, cleanup := newServer(t)
	defer cleanup()
	if stat, err := os.Stat(path); err == nil {
		if stat.Mode()&os.ModeSocket == 0 {
			t.Fatalf("Not a socket: %#o", stat.Mode())
		}
	} else {
		t.Fatal(err)
	}
	ipc1.Close()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Expected NotExist, got %v", err)
	}
}

func TestBadConnect(t *testing.T) {
	t.Skip()
	timeA := time.Now()
	ipc, path, cleanup := newServer(t)
	timeB := time.Now()
	if got := ipc.IdleStartTime(); got.Before(timeA) || got.After(timeB) {
		t.Fatalf("Wanted time between %v and %v, got %v instead", timeA, timeB, got)
	}
	defer cleanup()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	enc := vom.NewEncoder(conn)
	dec := vom.NewDecoder(conn)

	var theirInfo agent.ConnInfo
	if err = dec.Decode(&theirInfo); err != nil {
		t.Fatal(err)
	}
	if got := ipc.IdleStartTime(); !got.IsZero() {
		t.Fatalf("Wanted zero time, got %v instead", got)
	}

	timeA = time.Now()
	if err = enc.Encode(agent.RpcMessageReq{Value: agent.RpcRequest{Id: 0, Method: "foo", NumArgs: 0}}); err != nil {
		if !isEpipe(err) {
			// The server will close the connection when it gets
			// bad data. If that happens fast enough we get EPIPE.
			t.Fatal(err)
		}
	}
	var response agent.RpcMessage
	if err = dec.Decode(&response); err != io.EOF {
		t.Fatalf("Expected eof, got %v", err)
	}
	conn.Close()
	// Give a chance for the ipc object to observe the connection closure.
	for len(ipc.Connections()) > 0 {
		runtime.Gosched()
	}
	timeB = time.Now()
	if got := ipc.IdleStartTime(); got.Before(timeA) || got.After(timeB) {
		t.Fatalf("Wanted time between %v and %v, got %v instead", timeA, timeB, got)
	}
}

func TestBadMinVersion(t *testing.T) {
	_, path, cleanup := newServer(t)
	defer cleanup()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	enc := vom.NewEncoder(conn)
	dec := vom.NewDecoder(conn)

	var info agent.ConnInfo
	if err = dec.Decode(&info); err != nil {
		t.Fatal(err)
	}

	info.MaxVersion++
	info.MinVersion = info.MaxVersion
	if err = enc.Encode(info); err != nil {
		t.Fatal(err)
	}

	var response agent.RpcMessage
	if err = dec.Decode(&response); err != io.EOF {
		t.Fatalf("Expected EOF, got %v", err)
	}
}

func TestBadMaxVersion(t *testing.T) {
	_, path, cleanup := newServer(t)
	defer cleanup()
	conn, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	enc := vom.NewEncoder(conn)
	dec := vom.NewDecoder(conn)

	var info agent.ConnInfo
	if err = dec.Decode(&info); err != nil {
		t.Fatal(err)
	}

	info.MinVersion--
	info.MaxVersion = info.MinVersion
	if err = enc.Encode(info); err != nil {
		t.Fatal(err)
	}

	var response agent.RpcMessage
	if err = dec.Decode(&response); err != io.EOF {
		t.Fatalf("Expected EOF, got %v", err)
	}
}

func TestEcho(t *testing.T) {
	var e echo
	_, path, cleanup := newServer(t, e)
	defer cleanup()

	ipc2 := NewIPC()
	defer ipc2.Close()
	client, err := ipc2.Connect(path, 0)
	if err != nil {
		t.Fatal(err)
	}

	var result string
	err = client.Call("Echo", []interface{}{"hello"}, &result)
	if err != nil {
		t.Fatal(err)
	}
	if result != "hello" {
		t.Fatalf("Expected hello, got %q", result)
	}
}

func TestOutOfOrder(t *testing.T) {
	delay := newDelayedEcho()
	_, path, cleanup := newServer(t, delay)
	defer cleanup()

	ipc2 := NewIPC()
	defer ipc2.Close()
	client, err := ipc2.Connect(path, 0)
	if err != nil {
		t.Fatal(err)
	}

	var resultA string
	errA := make(chan error)
	go func() { errA <- client.Call("EchoA", nil, &resultA) }()
	<-delay.gotA

	delay.doneB <- " world"
	var resultB string
	err = client.Call("EchoB", []interface{}{"hello"}, &resultB)
	if err != nil {
		t.Fatal(err)
	}
	if resultB != "hello world" {
		t.Fatalf("Expected 'hello word', got %q", resultB)
	}
	delay.doneA <- "foobar"
	if err := <-errA; err != nil {
		t.Fatal(err)
	}
	if resultA != "foobar" {
		t.Fatalf("Expected foobar, got %q", resultA)
	}

}

func TestErrorReturn(t *testing.T) {
	var s returnError
	_, path, cleanup := newServer(t, s)
	defer cleanup()

	ipc2 := NewIPC()
	defer ipc2.Close()
	client, err := ipc2.Connect(path, 0)
	if err != nil {
		t.Fatal(err)
	}

	var result int
	err = client.Call("BreakMe", []interface{}{"oh", "no"}, &result)
	if err == nil {
		t.Fatalf("Expected error, got %d", result)
	}
	if !strings.HasSuffix(err.Error(), "oh no") {
		t.Fatalf("Expected 'oh no', got %q", err.Error())
	}
}

func TestBidi(t *testing.T) {
	s := &tunnel{}
	ipc1, path, cleanup := newServer(t, s)
	defer cleanup()
	s.ipc = ipc1

	ipc2 := NewIPC()
	defer ipc2.Close()

	var e echo
	ipc2.Serve(e)
	client, err := ipc2.Connect(path, 0)
	if err != nil {
		t.Fatal(err)
	}

	var result string
	err = client.Call("Tunnel", nil, &result)
	if err != nil {
		t.Fatal(err)
	}
	if result != "Tunnel" {
		t.Fatalf("Expected Tunnel, got %q", result)
	}
}

func TestMain(m *testing.M) {
	flag.Parse()
	vlog.ConfigureLibraryLoggerFromFlags()
	os.Exit(m.Run())
}
