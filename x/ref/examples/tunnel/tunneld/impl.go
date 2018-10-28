// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/x/ref/examples/tunnel"
	"v.io/x/ref/examples/tunnel/internal"
)

// T implements tunnel.TunnelServerMethods
type T struct {
}

// Use the same exit code as SSH when a non-shell error occurs.
const nonShellErrorCode int32 = 255

func (t *T) Forward(ctx *context.T, call tunnel.TunnelForwardServerCall, network, address string) error {
	conn, err := net.Dial(network, address)
	if err != nil {
		return err
	}
	b, _ := security.RemoteBlessingNames(ctx, call.Security())
	name := fmt.Sprintf("RemoteBlessings:%v LocalAddr:%v RemoteAddr:%v", b, conn.LocalAddr(), conn.RemoteAddr())
	ctx.Infof("TUNNEL START: %v", name)
	err = internal.Forward(conn, call.SendStream(), call.RecvStream())
	ctx.Infof("TUNNEL END  : %v (%v)", name, err)
	return err
}

func (t *T) ReverseForward(ctx *context.T, call rpc.ServerCall, network, address string) error {
	ln, err := net.Listen(network, address)
	if err != nil {
		return err
	}
	defer ln.Close()
	ctx.Infof("Listening on %q", ln.Addr())
	remoteEP := call.RemoteEndpoint().Name()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				ctx.Infof("Accept failed: %v", err)
				if oErr, ok := err.(*net.OpError); ok && oErr.Temporary() {
					continue
				}
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				stream, err := tunnel.ForwarderClient(remoteEP).Forward(ctx)
				if err != nil {
					ctx.Infof("Forward failed: %v", err)
					return
				}
				name := fmt.Sprintf("%v-->%v-->(%v)", c.RemoteAddr(), c.LocalAddr(), remoteEP)
				ctx.Infof("TUNNEL START: %v", name)
				errf := internal.Forward(c, stream.SendStream(), stream.RecvStream())
				err = stream.Finish()
				ctx.Infof("TUNNEL END  : %v (%v, %v)", name, errf, err)
			}(conn)
		}
	}()
	<-ctx.Done()
	return nil
}

// findShell returns the path to the first usable shell binary.
func findShell() (string, error) {
	shells := []string{"/bin/bash", "/bin/sh", "/system/bin/sh"}
	for _, s := range shells {
		if _, err := os.Stat(s); err == nil {
			return s, nil
		}
	}
	return "", errors.New("could not find any shell binary")
}

// sendMotd sends the content of the MOTD file to the stream, if it exists.
func sendMotd(ctx *context.T, s tunnel.TunnelShellServerStream) {
	data, err := ioutil.ReadFile("/etc/motd")
	if err != nil {
		// No MOTD. That's OK.
		return
	}
	packet := tunnel.ServerShellPacketStdout{[]byte(data)}
	if err = s.SendStream().Send(packet); err != nil {
		ctx.Infof("Send failed: %v", err)
	}
}

func setWindowSize(ctx *context.T, fd uintptr, row, col uint16) {
	ws := internal.Winsize{Row: row, Col: col}
	if err := internal.SetWindowSize(fd, ws); err != nil {
		ctx.Infof("Failed to set window size: %v", err)
	}
}
