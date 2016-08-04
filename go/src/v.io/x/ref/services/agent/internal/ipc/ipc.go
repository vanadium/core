// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ipc implements a simple IPC system based on VOM.
package ipc

import (
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"sync"
	"syscall"
	"time"

	"v.io/v23/vom"
	"v.io/x/lib/vlog"
	"v.io/x/ref/services/agent"
)

const currentVersion = 1

var zeroTime time.Time

type decoderFunc func(n uint32, dec *vom.Decoder) ([]reflect.Value, error)

type rpcInfo struct {
	id      uint64
	results []interface{}
	done    chan error
}

type rpcHandler struct {
	decode decoderFunc
	f      reflect.Value
}

// IPC provides interprocess rpcs.
// One process must act as the "server" by listening for connections.
// However once connected rpcs can flow in either direction.
type IPC struct {
	handlers      map[string]rpcHandler
	mu            sync.Mutex
	listener      *net.UnixListener
	conns         []*IPCConn
	idleStartTime time.Time
}

// IPCConn represents a connection to a process.
// It can be used to make rpcs to the other process.  It also
// dispatches rpcs from that process to handlers registered with the
// parent IPC object.
type IPCConn struct {
	enc    *vom.Encoder
	dec    *vom.Decoder
	conn   net.Conn
	mu     sync.Mutex
	nextId uint64
	ipc    *IPC
	rpcs   map[uint64]rpcInfo
}

// Close the connection to the process.
// All outstanding rpcs will return errors.
func (c *IPCConn) Close() {
	c.ipc.closeConn(c)
	c.mu.Lock()
	for _, rpc := range c.rpcs {
		rpc.done <- io.EOF
	}
	c.rpcs = nil
	c.mu.Unlock()
}

// Perform an rpc with the given name and arguments.
// 'args' must correspond with the method registered in the other process,
// and results must contain pointers to the return types of the method.
// All rpc methods must have a final error result, which is not included in 'results'.
// It is the return value of Call().
func (c *IPCConn) Call(method string, args []interface{}, results ...interface{}) error {
	ch := c.ipc.startCall(c, method, args, results)
	return <-ch
}

// Create a new IPC object.
// After calling new you can register handlers with Serve().
// Then call Listen() or Connect().
func NewIPC() *IPC {
	ipc := &IPC{
		handlers:      make(map[string]rpcHandler),
		idleStartTime: time.Now(),
	}
	return ipc
}

// Listen for connections by creating a UNIX domain socket at 'path'.
func (ipc *IPC) Listen(path string) error {
	addr, err := net.ResolveUnixAddr("unix", path)
	if err != nil {
		return err
	}
	ipc.mu.Lock()
	ipc.listener, err = net.ListenUnix("unix", addr)
	ipc.mu.Unlock()
	if err == nil {
		go ipc.listenLoop()
	}
	return err
}

// Connect to a listening socket at 'path'.  If timeout is non-zero and the
// initial connection fails because the socket is not ready, Connect will retry
// once per second until the timeout expires.
func (ipc *IPC) Connect(path string, timeout time.Duration) (*IPCConn, error) {
	timeoutExpired := time.After(timeout)
	// It's possible the agent isn't ready yet, in which case, the
	// underlying connect() call returns ENOENT.
	retryable := func(err error) bool {
		opErr, ok := err.(*net.OpError)
		if !ok {
			return false
		}
		sysErr, ok := opErr.Err.(*os.SyscallError)
		return ok && sysErr.Err == syscall.ENOENT
	}
	for {
		conn, err := net.Dial("unix", path)
		switch {
		case err == nil:
			return ipc.newConn(conn), nil
		case !retryable(err):
			return nil, err
		}
		// Retry.
		select {
		case <-time.After(time.Second):
		case <-timeoutExpired:
			return nil, err
		}
	}
}

// Connections returns all the current connections in this IPC.
func (ipc *IPC) Connections() []*IPCConn {
	defer ipc.mu.Unlock()
	ipc.mu.Lock()
	conns := make([]*IPCConn, len(ipc.conns))
	copy(conns, ipc.conns)
	return conns
}

// Must be called while not holding ipc.mu
func (ipc *IPC) newConn(conn net.Conn) *IPCConn {
	ipc.mu.Lock()
	defer ipc.mu.Unlock()
	ic := ipc.newConnLocked(conn)
	go ipc.readLoop(ic)
	return ic
}

// Must be called while holding ipc.mu
func (ipc *IPC) newConnLocked(conn net.Conn) *IPCConn {
	result := &IPCConn{enc: vom.NewEncoder(conn), dec: vom.NewDecoder(conn), conn: conn, ipc: ipc, rpcs: make(map[uint64]rpcInfo)}
	// Don't allow any rpcs to be sent until negotiateVersion unlocks this.
	result.mu.Lock()
	ipc.conns = append(ipc.conns, result)
	ipc.idleStartTime = zeroTime
	return result
}

// IdleStartTime returns the time when this IPC became idle (no connections).
// If there are connections currently, returns the zero time instant.
func (ipc *IPC) IdleStartTime() time.Time {
	ipc.mu.Lock()
	defer ipc.mu.Unlock()
	return ipc.idleStartTime
}

func (ipc *IPC) closeConn(c *IPCConn) {
	ipc.mu.Lock()
	for i := range ipc.conns {
		if ipc.conns[i] == c {
			last := len(ipc.conns) - 1
			ipc.conns[i], ipc.conns[last], ipc.conns = ipc.conns[last], nil, ipc.conns[:last]
			if len(ipc.conns) == 0 {
				ipc.idleStartTime = time.Now()
			}
			break
		}
	}
	ipc.mu.Unlock()
	c.conn.Close()
}

func (ipc *IPC) listenLoop() {
	ipc.mu.Lock()
	l := ipc.listener
	ipc.mu.Unlock()
	for l != nil {
		conn, err := l.AcceptUnix()
		if err != nil {
			vlog.VI(3).Infof("Accept error: %v", err)
			return
		}
		ipc.mu.Lock()
		l = ipc.listener
		if l == nil {
			// We're already closed!
			conn.Close()
			return
		}
		ic := ipc.newConnLocked(conn)
		ipc.mu.Unlock()
		go ipc.readLoop(ic)
	}
}

func (ipc *IPC) readLoop(c *IPCConn) {
	v := ipc.negotiateVersion(c)
	if v != currentVersion {
		ipc.closeConn(c)
		return
	}
	for {
		if f, err := ipc.readMessage(c); err != nil {
			vlog.VI(3).Infof("ipc read error: %v", err)
			c.Close()
			return
		} else if f != nil {
			go f()
		}
	}
}

func (ipc *IPC) negotiateVersion(c *IPCConn) int32 {
	go func() {
		myInfo := agent.ConnInfo{MinVersion: currentVersion, MaxVersion: currentVersion}
		// c.mu.Lock() already called in newConnLocked
		vlog.VI(4).Infof("negotiateVersion: sending %v", myInfo)
		err := c.enc.Encode(myInfo)
		c.mu.Unlock()
		if err != nil {
			vlog.VI(3).Infof("ipc.negotiateVersion encode: %v", err)
			ipc.closeConn(c)
		}
	}()
	var theirInfo agent.ConnInfo
	err := c.dec.Decode(&theirInfo)
	if err != nil {
		vlog.VI(3).Infof("ipc.negotiateVersion decode: %v", err)
	}
	if theirInfo.MinVersion <= currentVersion && theirInfo.MaxVersion >= currentVersion {
		return currentVersion
	}
	vlog.VI(3).Infof("ipc.negotiateVersion %d not in remote range: %d-%d", currentVersion, theirInfo.MinVersion, theirInfo.MaxVersion)
	return -1
}

func makeDecoder(t reflect.Type) decoderFunc {
	numArgs := t.NumIn() - 1
	inTypes := make([]reflect.Type, numArgs)
	for i := 1; i < numArgs+1; i++ {
		inTypes[i-1] = t.In(i)
	}
	return func(n uint32, dec *vom.Decoder) (result []reflect.Value, err error) {
		if n != uint32(numArgs) {
			for i := uint32(0); i < n; i++ {
				if err = dec.Decoder().SkipValue(); err != nil {
					return nil, err
				}
			}
			return nil, fmt.Errorf("wrong number of args: expected %d, got %d", numArgs, n)
		}

		result = make([]reflect.Value, numArgs)
		for i := 0; i < numArgs; i++ {
			v := reflect.New(inTypes[i])
			result[i] = reflect.Indirect(v)
			if err = dec.Decode(v.Interface()); err != nil {
				vlog.VI(3).Infof("decode error for %v: %v", inTypes[i], err)
				return nil, err
			}
		}
		return
	}
}

func noDecoder(n uint32, dec *vom.Decoder) (result []reflect.Value, err error) {
	for i := uint32(0); i < n; i++ {
		if err = dec.Decoder().SkipValue(); err != nil {
			return nil, err
		}
	}
	return nil, fmt.Errorf("unknown method")
}

// Serve registers rpc handlers.
// All exported methods in 'x' are registered for rpc.
// All arguments and results for these methods must be VOM serializable.
// Additionally each method must have at least one return value, and
// the final return value must be an 'error'.
// Serve must be called before Listen() or Connect().
func (ipc *IPC) Serve(x interface{}) error {
	v := reflect.ValueOf(x)
	t := reflect.TypeOf(x)
	for i, numMethods := 0, v.NumMethod(); i < numMethods; i++ {
		m := t.Method(i)
		if _, exists := ipc.handlers[m.Name]; exists {
			return fmt.Errorf("Method %s already registered", m.Name)
		}
		ipc.handlers[m.Name] = rpcHandler{makeDecoder(m.Type), v.Method(i)}
		vlog.VI(4).Infof("Registered method %q", m.Name)
	}
	return nil
}

func (ipc *IPC) dispatch(req agent.RpcRequest) rpcHandler {
	handler, ok := ipc.handlers[req.Method]
	if !ok {
		handler = rpcHandler{noDecoder, reflect.Value{}}
	}
	return handler
}

// Close the IPC and all it's connections.
func (ipc *IPC) Close() {
	ipc.mu.Lock()
	if ipc.listener != nil {
		ipc.listener.Close()
		ipc.listener = nil
	}
	for _, c := range ipc.conns {
		c.conn.Close()
	}
	ipc.conns = nil
	ipc.mu.Unlock()
}

func (ipc *IPC) startCall(c *IPCConn, method string, args []interface{}, results []interface{}) (ch chan error) {
	ch = make(chan error, 1)
	header := agent.RpcMessageReq{Value: agent.RpcRequest{Method: method, NumArgs: uint32(len(args))}}
	shouldClose := false
	defer func() {
		// Only close c after we unlock it.
		if shouldClose {
			c.Close()
		}
	}()
	defer c.mu.Unlock()
	c.mu.Lock()
	header.Value.Id = c.nextId
	c.nextId++
	vlog.VI(4).Infof("startCall sending %v", header)
	if err := c.enc.Encode(header); err != nil {
		// TODO(ribrdb): Close?
		vlog.VI(4).Infof("startCall encode error %v", err)
		ch <- err
		return
	}
	for _, arg := range args {
		if err := c.enc.Encode(arg); err != nil {
			vlog.VI(4).Infof("startCall arg error %v", err)
			shouldClose = true
			ch <- err
			return
		}
	}
	c.rpcs[header.Value.Id] = rpcInfo{header.Value.Id, results, ch}
	return
}

func (ipc *IPC) readMessage(c *IPCConn) (func(), error) {
	var m agent.RpcMessage
	if err := c.dec.Decode(&m); err != nil {
		return nil, err
	}
	vlog.VI(4).Infof("readMessage decoded %v", m)
	switch msg := m.Interface().(type) {
	case agent.RpcRequest:
		handler := ipc.dispatch(msg)
		args, err := handler.decode(msg.NumArgs, c.dec)

		if err != nil {
			err = fmt.Errorf("%s: %v", msg.Method, err)
		}
		return func() { ipc.handleReq(c, msg.Id, handler.f, args, err) }, nil
	case agent.RpcResponse:
		return nil, ipc.handleResp(c, msg)
	default:
		return nil, fmt.Errorf("unsupported type %t", msg)
	}
}

func (ipc *IPC) handleReq(c *IPCConn, id uint64, method reflect.Value, args []reflect.Value, err error) {
	var results []reflect.Value
	if err == nil {
		results = method.Call(args)
		last := len(results) - 1
		if !results[last].IsNil() {
			err = results[last].Interface().(error)
		}
		results = results[:last]
	}
	header := agent.RpcMessageResp{Value: agent.RpcResponse{Id: id, NumArgs: uint32(len(results)), Err: err}}
	shouldClose := false
	defer func() {
		if shouldClose {
			c.Close()
		}
	}()
	defer c.mu.Unlock()
	c.mu.Lock()
	vlog.VI(4).Infof("handleReq sending %v", header)
	if err = c.enc.Encode(header); err != nil {
		vlog.VI(3).Infof("handleReq error %v", err)
		shouldClose = true
		return
	}
	for _, result := range results {
		if err = c.enc.Encode(result.Interface()); err != nil {
			vlog.VI(3).Infof("handleReq result error %v", err)
			shouldClose = true
			return
		}
	}
}

func (ipc *IPC) handleResp(c *IPCConn, resp agent.RpcResponse) error {
	c.mu.Lock()
	info, ok := c.rpcs[resp.Id]
	if ok {
		delete(c.rpcs, resp.Id)
	}
	c.mu.Unlock()
	if ok && resp.NumArgs == uint32(len(info.results)) {
		for i := range info.results {
			if err := c.dec.Decode(info.results[i]); err != nil {
				vlog.VI(3).Infof("result decode error for %T: %v", info.results[i], err)
				info.done <- err
				return err
			}
		}
		info.done <- resp.Err
	} else {
		for i := uint32(0); i < resp.NumArgs; i++ {
			c.dec.Decoder().SkipValue()
		}
		err := resp.Err
		if err == nil {
			err = fmt.Errorf("invalid results")
		}
		info.done <- err
	}
	return nil
}
