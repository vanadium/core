// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run v.io/x/lib/cmdline/gendoc . -help

package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"v.io/x/lib/cmdline"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/verror"

	"v.io/x/ref/examples/tunnel"
	"v.io/x/ref/examples/tunnel/internal"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/lib/v23cmd"
	_ "v.io/x/ref/runtime/factories/generic"
)

var (
	disablePty, forcePty, noShell   bool
	localProtocol, remoteProtocol   string
	portForward, reversePortForward stringList
)

type stringList []string

func (sl *stringList) String() string {
	return fmt.Sprint(*sl)
}

func (sl *stringList) Set(value string) error {
	for _, v := range *sl {
		if v == value {
			return nil
		}
	}
	*sl = append(*sl, value)
	return nil
}

func main() {
	cmdVsh.Flags.BoolVar(&disablePty, "T", false, "Disable pseudo-terminal allocation.")
	cmdVsh.Flags.BoolVar(&forcePty, "t", false, "Force allocation of pseudo-terminal.")
	cmdVsh.Flags.BoolVar(&noShell, "N", false, "Do not execute a shell.  Only do port forwarding.")
	cmdVsh.Flags.Var(&portForward, "L", `Forward local to remote, format is "localAddress,remoteAddress". May be repeated.`)
	cmdVsh.Flags.Var(&reversePortForward, "R", `Forward remote to local, format is "remoteAddress,localAddress". May be repeated.`)
	cmdVsh.Flags.StringVar(&localProtocol, "local-protocol", "tcp", "Local network protocol for port forwarding.")
	cmdVsh.Flags.StringVar(&remoteProtocol, "remote-protocol", "tcp", "Remote network protocol for port forwarding.")
	cmdline.HideGlobalFlagsExcept()
	cmdline.Main(cmdVsh)
}

var cmdVsh = &cmdline.Command{
	Runner: v23cmd.RunnerFunc(runVsh),
	Name:   "vsh",
	Short:  "Vanadium shell",
	Long: `
Command vsh runs the Vanadium shell, a Tunnel client that can be used to run
shell commands or start an interactive shell on a remote tunneld server.

To open an interactive shell, use:
  vsh <object name>

To run a shell command, use:
  vsh <object name> <command to run>

The -L flag will forward connections from a local port to a remote address
through the tunneld service. The flag value is localAddress,remoteAddress. E.g.
  -L :14141,www.google.com:80

The -R flag will forward connections from a remote port on the tunneld service
to a local address. The flag value is remoteAddress,localAddress. E.g.
  -R :14141,www.google.com:80

vsh can't be used directly with tools like rsync because vanadium object names
don't look like traditional hostnames, which rsync doesn't understand. For
compatibility with such tools, vsh has a special feature that allows passing the
vanadium object name via the VSH_NAME environment variable.

  $ VSH_NAME=<object name> rsync -avh -e vsh /foo/* v23:/foo/

In this example, the "v23" host will be substituted with $VSH_NAME by vsh and
rsync will work as expected.
`,
	ArgsName: "<object name> [command]",
	ArgsLong: `
<object name> is the Vanadium object name of the server to connect to.

[command] is the shell command and args to run, for non-interactive vsh.
`,
}

func runVsh(ctx *context.T, env *cmdline.Env, args []string) error {
	serverName, cmd, err := objectNameAndCommandLine(env, args)
	if err != nil {
		return env.UsageErrorf("%v", err)
	}

	for _, pf := range portForward {
		go runPortForwarding(ctx, serverName, pf)
	}
	for _, rpf := range reversePortForward {
		go runReversePortForwarding(ctx, serverName, rpf)
	}

	if noShell {
		<-signals.ShutdownOnSignals(ctx)
		return nil
	}

	opts := shellOptions(ctx, env, cmd)

	call, err := tunnel.TunnelClient(serverName).Shell(ctx, cmd, opts)
	if err != nil {
		return err
	}
	if opts.UsePty {
		saved := internal.EnterRawTerminalMode(ctx)
		defer internal.RestoreTerminalSettings(ctx, saved)
	}

	var (
		sendMutex sync.Mutex
		send      = func(packet tunnel.ClientShellPacket) error {
			sendMutex.Lock()
			defer sendMutex.Unlock()
			return call.SendStream().Send(packet)
		}
	)

	// Read loop for STDIN
	// This loop reads from STDIN and sends the data on the RPC stream.
	go func() {
		buf := make([]byte, 2048)
		for {
			n, err := env.Stdin.Read(buf[:])
			if err == io.EOF {
				if err := send(tunnel.ClientShellPacketEndOfFile{}); err != nil {
					ctx.VI(3).Infof("send failed: %v", err)
				}
				return
			}
			if err != nil {
				return
			}
			if err := send(tunnel.ClientShellPacketStdin{buf[:n]}); err != nil {
				ctx.VI(3).Infof("send failed: %v", err)
				return
			}
		}
	}()

	// Signal loop for WINCH
	// This loops sends WinSize packets on the RPC stream whenever the local
	// window size changes.
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	go func() {
		for range winch {
			ws, err := internal.GetWindowSize()
			if err != nil {
				ctx.Infof("GetWindowSize failed: %v", err)
				continue
			}
			if err := send(tunnel.ClientShellPacketWinSize{tunnel.WindowSize{Rows: ws.Row, Cols: ws.Col}}); err != nil {
				ctx.VI(2).Infof("send failed: %v", err)
				return
			}
		}
	}()

	// Read loop for STDOUT / STDERR
	// This loop receives output from the remote shell and writes it to
	// STDOUT or STDERR.
	for call.RecvStream().Advance() {
		packet := call.RecvStream().Value()
		switch v := packet.(type) {
		case tunnel.ServerShellPacketStdout:
			if n, err := env.Stdout.Write(v.Value); n != len(v.Value) || err != nil {
				break
			}
		case tunnel.ServerShellPacketStderr:
			if n, err := env.Stderr.Write(v.Value); n != len(v.Value) || err != nil {
				break
			}
		default:
			ctx.Infof("unexpected message type: %T", packet)
		}
	}

	exitMsg := fmt.Sprintf("Connection to %s closed.", serverName)
	exitStatus, m, err := call.Finish()
	if err != nil {
		return err
	}
	if m != "" {
		exitMsg += fmt.Sprintf(" (%v)", m)
	}
	ctx.VI(1).Info(exitMsg)
	// Only show the exit message on stdout for interactive shells.
	// Otherwise, the exit message might get confused with the output
	// of the command that was run.
	if err != nil {
		fmt.Fprintln(env.Stderr, exitMsg)
	} else if len(cmd) == 0 {
		fmt.Println(exitMsg)
	}
	return cmdline.ErrExitCode(exitStatus)
}

func shellOptions(ctx *context.T, env *cmdline.Env, cmd string) (opts tunnel.ShellOpts) {
	opts.UsePty = (len(cmd) == 0 || forcePty) && !disablePty
	opts.Environment = environment(env.Vars)
	ws, err := internal.GetWindowSize()
	if err != nil {
		ctx.VI(1).Infof("GetWindowSize failed: %v", err)
	} else {
		opts.WinSize.Rows = ws.Row
		opts.WinSize.Cols = ws.Col
	}
	return
}

func environment(vars map[string]string) []string {
	env := []string{}
	for _, name := range []string{"TERM", "COLORTERM"} {
		if value := vars[name]; value != "" {
			env = append(env, name+"="+value)
		}
	}
	return env
}

// objectNameAndCommandLine extracts the object name and the remote command to
// send to the server. The object name is the first non-flag argument.
// The command line is the concatenation of all non-flag arguments excluding
// the object name.
func objectNameAndCommandLine(env *cmdline.Env, args []string) (string, string, error) {
	if len(args) < 1 {
		return "", "", errors.New("object name missing")
	}
	name := args[0]
	args = args[1:]
	// For compatibility with tools like rsync. Because object names
	// don't look like traditional hostnames, tools that work with rsh and
	// ssh can't work directly with vsh. This trick makes the following
	// possible:
	//   $ VSH_NAME=<object name> rsync -avh -e vsh /foo/* v23:/foo/
	// The "v23" host will be substituted with <object name>.
	if envName := env.Vars["VSH_NAME"]; envName != "" && name == "v23" {
		name = envName
	}
	cmd := strings.Join(args, " ")
	return name, cmd, nil
}

func runPortForwarding(ctx *context.T, serverName, portForward string) {
	// portForward is localAddress,remoteAddress
	parts := strings.Split(portForward, ",")
	if len(parts) != 2 {
		ctx.Fatalf("-L flag expects 2 values separated by a comma")
	}
	localAddress, remoteAddress := parts[0], parts[1]

	ln, err := net.Listen(localProtocol, localAddress)
	if err != nil {
		ctx.Fatalf("net.Listen(%q, %q) failed: %v", localProtocol, localAddress, err)
	}
	defer ln.Close()
	ctx.VI(1).Infof("Listening on %q", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			ctx.Infof("Accept failed: %v", err)
			continue
		}
		stream, err := tunnel.TunnelClient(serverName).Forward(ctx, remoteProtocol, remoteAddress)
		if err != nil {
			ctx.Infof("Tunnel(%q, %q) failed: %v", remoteProtocol, remoteAddress, err)
			conn.Close()
			continue
		}
		name := fmt.Sprintf("%v-->%v-->(%v)-->%v", conn.RemoteAddr(), conn.LocalAddr(), serverName, remoteAddress)
		go func() {
			ctx.VI(1).Infof("TUNNEL START: %v", name)
			errf := internal.Forward(conn, stream.SendStream(), stream.RecvStream())
			err := stream.Finish()
			ctx.VI(1).Infof("TUNNEL END  : %v (%v, %v)", name, errf, err)
		}()
	}
}

type forwarder struct {
	network, address string
}

func (f *forwarder) Forward(ctx *context.T, call tunnel.ForwarderForwardServerCall) error {
	conn, err := net.Dial(f.network, f.address)
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

// This is a combined Authorizer and Granter. It is used as Authorizer for our
// local forwarder server, and as a security callback (via the Granter option)
// for the ReverserForward RPC.
type authorizerGranterHack struct {
	mu   sync.Mutex
	auth security.Authorizer
	rpc.CallOpt
}

func (a *authorizerGranterHack) Authorize(ctx *context.T, call security.Call) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.auth == nil {
		return verror.ErrNoAccess.Errorf(ctx, "access denied")
	}
	return a.auth.Authorize(ctx, call)
}

func (a *authorizerGranterHack) Grant(_ *context.T, call security.Call) (security.Blessings, error) {
	a.mu.Lock()
	a.auth = security.PublicKeyAuthorizer(call.RemoteBlessings().PublicKey())
	a.mu.Unlock()
	// Don't actually want to grant anything, just using the Granter as a
	// callback to obtain the remote end's private key before the request
	// is sent to it.
	return security.Blessings{}, nil
}

func runReversePortForwarding(ctx *context.T, serverName, reversePortForward string) {
	// reversePortForward is remoteAddress,localAddress
	parts := strings.Split(reversePortForward, ",")
	if len(parts) != 2 {
		ctx.Fatalf("-R flag expects 2 values separated by a comma")
	}
	remoteAddress, localAddress := parts[0], parts[1]

	ctx = v23.WithListenSpec(ctx, rpc.ListenSpec{})
	auth := &authorizerGranterHack{}
	ctx, _, err := v23.WithNewServer(ctx, "", tunnel.ForwarderServer(&forwarder{localProtocol, localAddress}), auth)
	if err != nil {
		ctx.Fatalf("Failed to create server: %v", err)
	}
	tunnel.TunnelClient(serverName).ReverseForward(ctx, remoteProtocol, remoteAddress, auth)
}
