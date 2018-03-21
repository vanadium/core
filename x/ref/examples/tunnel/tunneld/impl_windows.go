// +build windows

package main

import (
	"v.io/v23/context"
	"v.io/x/ref/examples/tunnel"
)

func (t *T) Shell(ctx *context.T, call tunnel.TunnelShellServerCall, command string, shellOpts tunnel.ShellOpts) (int32, string, error) {
	panic("not implemented")
	return 0, "", nil
}
