// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/kr/pty"

	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

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

func (t *T) Shell(ctx *context.T, call tunnel.TunnelShellServerCall, command string, shellOpts tunnel.ShellOpts) (int32, string, error) {
	b, _ := security.RemoteBlessingNames(ctx, call.Security())
	ctx.Infof("SHELL START for %v: %q", b, command)
	shell, err := findShell()
	if err != nil {
		return nonShellErrorCode, "", err
	}
	var c *exec.Cmd
	// An empty command means that we need an interactive shell.
	if len(command) == 0 {
		c = exec.Command(shell, "-i")
		sendMotd(ctx, call)
	} else {
		c = exec.Command(shell, "-c", command)
	}

	c.SysProcAttr = &syscall.SysProcAttr{
		// Create a new process group for the child process. After the
		// child exits, we'll use it to clean up processes left behind.
		Setpgid: true,
	}

	c.Env = []string{
		fmt.Sprintf("HOME=%s", os.Getenv("HOME")),
		fmt.Sprintf("PATH=%s", os.Getenv("PATH")),
	}
	c.Env = append(c.Env, shellOpts.Environment...)
	ctx.Infof("Shell environment: %v", c.Env)

	c.Dir = os.Getenv("HOME")
	ctx.Infof("Shell CWD: %v", c.Dir)

	var (
		stdin          io.WriteCloser // We write to stdin.
		stdout, stderr io.ReadCloser  // We read from stdout and stderr.
		ptyFd          uintptr        // File descriptor for pty.
	)

	if shellOpts.UsePty {
		f, err := pty.Start(c)
		if err != nil {
			return nonShellErrorCode, "", err
		}
		defer f.Close()

		stdin, stdout, stderr = f, f, nil
		ptyFd = f.Fd()

		setWindowSize(ctx, ptyFd, shellOpts.WinSize.Rows, shellOpts.WinSize.Cols)
	} else {
		var err error
		if stdin, err = c.StdinPipe(); err != nil {
			return nonShellErrorCode, "", err
		}
		defer stdin.Close()

		if stdout, err = c.StdoutPipe(); err != nil {
			return nonShellErrorCode, "", err
		}
		defer stdout.Close()

		if stderr, err = c.StderrPipe(); err != nil {
			return nonShellErrorCode, "", err
		}
		defer stderr.Close()

		if err = c.Start(); err != nil {
			ctx.Infof("Cmd.Start failed: %v", err)
			return nonShellErrorCode, "", err
		}
	}

	// Read loop for STDIN
	// This loop reads from the RPC stream and writes to STDIN of the child
	// process.
	go func() {
		stream := call.RecvStream()
		for stream.Advance() {
			packet := stream.Value()
			switch v := packet.(type) {
			case tunnel.ClientShellPacketStdin:
				if n, err := stdin.Write(v.Value); n != len(v.Value) || err != nil {
					ctx.VI(3).Infof("Write failed: %v", err)
				}
			case tunnel.ClientShellPacketEndOfFile:
				if err := stdin.Close(); err != nil {
					ctx.VI(3).Infof("Close failed: %v", err)
				}
			case tunnel.ClientShellPacketWinSize:
				size := v.Value
				if size.Rows > 0 && size.Cols > 0 && ptyFd != 0 {
					setWindowSize(ctx, ptyFd, size.Rows, size.Cols)
				}
			default:
				ctx.Infof("unexpected message type: %T", packet)
			}
		}

		ctx.VI(3).Infof("Read loop STDIN: %v", stream.Err())
		if err := stdin.Close(); err != nil {
			ctx.VI(3).Infof("stdin.Close failed: %v", err)
		}

	}()

	var (
		// wg is used to make sure the STDOUT and STDERR read loops exit
		// before the RPC ends.
		wg sync.WaitGroup

		// STDOUT and STDERR are sent on the same stream, which is not
		// thread-safe.
		sendMutex sync.Mutex

		// The read loop reads from STDOUT or STDOUT of the child
		// process and writes to the RPC stream.
		readLoop = func(r io.Reader, stderr bool) {
			defer wg.Done()
			buf := make([]byte, 2048)
			var pkt tunnel.ServerShellPacket
			for {
				n, err := r.Read(buf[:])
				if err != nil {
					ctx.VI(3).Infof("Read failed: %v", err)
					return
				}
				if stderr {
					pkt = tunnel.ServerShellPacketStderr{buf[:n]}
				} else {
					pkt = tunnel.ServerShellPacketStdout{buf[:n]}
				}
				sendMutex.Lock()
				err = call.SendStream().Send(pkt)
				sendMutex.Unlock()
				if err != nil {
					ctx.VI(3).Infof("send failed: %v", err)
					return
				}
			}
		}
	)
	wg.Add(1)
	go readLoop(stdout, false)
	if stderr != nil {
		wg.Add(1)
		go readLoop(stderr, true)
	}
	defer wg.Wait()

	// When the RPC is terminated, we need to kill the child process, if
	// it is still running, and any other processes left behind in the same
	// process group.
	go func(p *os.Process) {
		if pid := p.Pid; pid > 0 {
			defer syscall.Kill(-pid, syscall.SIGKILL)
		}
		<-ctx.Done()
		if err := p.Signal(syscall.SIGHUP); err != nil {
			// The process has already exited, or we somehow don't
			// have permission to send a signal to that process.
			ctx.Infof("[%d].Signal(SIGHUP): %v", p.Pid, err)
			return
		}
		time.Sleep(time.Second)
		if err := p.Kill(); err != nil {
			ctx.Infof("[%d].Kill(): %v", p.Pid, err)
		}
	}(c.Process)

	// Wait for the child process to exit.
	// If the exit status is 0, return nil error. Otherwise, return an error
	// that describes how the process exited.
	state, err := c.Process.Wait()
	if err != nil {
		ctx.Infof("Process.Wait failed: %v", err)
		return nonShellErrorCode, "", err
	}
	ctx.Infof("SHELL END for %v: %s", b, state)

	if state.Success() {
		return 0, "", nil
	}
	status, ok := state.Sys().(syscall.WaitStatus)
	if !ok {
		return nonShellErrorCode, state.String(), nil
	}
	if status.Signaled() {
		return nonShellErrorCode, fmt.Sprintf("process killed by signal %d (%v)", status.Signal(), status.Signal()), nil
	}
	return int32(status.ExitStatus()), fmt.Sprintf("process exited with exit status %d", status.ExitStatus()), nil
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
