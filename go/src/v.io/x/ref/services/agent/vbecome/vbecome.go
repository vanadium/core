// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The following enables go generate to generate the doc.go file.
//go:generate go run $JIRI_ROOT/release/go/src/v.io/x/lib/cmdline/testdata/gendoc.go . -help

package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/lib/cmdline"
	"v.io/x/ref"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/v23cmd"
	"v.io/x/ref/services/agent/internal/ipc"
	"v.io/x/ref/services/agent/internal/server"
	"v.io/x/ref/services/role"

	_ "v.io/x/ref/runtime/factories/generic"
)

var (
	durationFlag time.Duration
	timeoutFlag  time.Duration
	nameFlag     string
	roleFlag     string
)

var cmdVbecome = &cmdline.Command{
	Runner:   v23cmd.RunnerFunc(vbecome),
	Name:     "vbecome",
	Short:    "executes commands with a derived Vanadium principal",
	Long:     "Command vbecome executes commands with a derived Vanadium principal.",
	ArgsName: "<command> [command args...]",
}

const childAgentFd = 3
const keyServerFd = 4

func main() {
	cmdline.HideGlobalFlagsExcept()
	syscall.CloseOnExec(childAgentFd)
	syscall.CloseOnExec(keyServerFd)

	cmdVbecome.Flags.DurationVar(&durationFlag, "duration", 1*time.Hour, "Duration for the blessing.")
	cmdVbecome.Flags.DurationVar(&timeoutFlag, "timeout", 2*time.Minute, "Timeout for the RPCs.")
	cmdVbecome.Flags.StringVar(&nameFlag, "name", "", "If set, the derived principal will be given an extension of the caller's blessings with this name.  A union blessing will be created if used in conjunction with --role")
	cmdVbecome.Flags.StringVar(&roleFlag, "role", "", "If set, the derived principal will be given a blessing obtained from this role object.  A union blessing will be created if used in conjunction with --name")

	cmdline.Main(cmdVbecome)
}

func vbecome(ctx *context.T, env *cmdline.Env, args []string) error {
	if len(args) == 0 {
		if shell := env.Vars["SHELL"]; shell != "" {
			args = []string{shell}
		} else {
			return fmt.Errorf("You must specify a command to run.")
		}
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	signer := security.NewInMemoryECDSASigner(key)
	principal, err := vsecurity.NewPrincipalFromSigner(signer, nil)
	if err != nil {
		return err
	}
	if len(roleFlag) == 0 && len(nameFlag) == 0 {
		return fmt.Errorf("at least one of --name or --role must be specified")
	}

	var nameB, roleB security.Blessings
	if len(nameFlag) > 0 {
		if nameB, err = bless(ctx, principal, nameFlag); err != nil {
			return err
		}
	}
	if len(roleFlag) > 0 {
		if roleB, err = setupRole(ctx, principal, roleFlag); err != nil {
			return err
		}
	}
	union, err := security.UnionOfBlessings(roleB, nameB)
	if err != nil {
		return err
	}
	if err := principal.BlessingStore().SetDefault(union); err != nil {
		return err
	}
	if _, err := principal.BlessingStore().Set(union, security.AllPrincipals); err != nil {
		return err
	}
	if err := security.AddToRoots(principal, union); err != nil {
		return err
	}

	// Clear out the environment variable before starting the child.
	if err = ref.EnvClearCredentials(); err != nil {
		return err
	}

	// Start an agent server.
	i := ipc.NewIPC()
	if err := server.ServeAgent(i, principal); err != nil {
		return err
	}
	dir, err := ioutil.TempDir("", "vbecome")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "sock")
	if err := i.Listen(path); err != nil {
		return err
	}
	defer i.Close()
	if err = os.Setenv(ref.EnvAgentPath, path); err != nil {
		ctx.Fatalf("setenv: %v", err)
	}

	return doExec(args)
}

func bless(ctx *context.T, p security.Principal, name string) (security.Blessings, error) {
	caveat, err := security.NewExpiryCaveat(time.Now().Add(durationFlag))
	if err != nil {
		ctx.Errorf("Couldn't create caveat")
		return security.Blessings{}, err
	}
	rp := v23.GetPrincipal(ctx)
	rblessing, _ := rp.BlessingStore().Default()
	return rp.Bless(p.PublicKey(), rblessing, name, caveat)
}

func doExec(args []string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func setupRole(ctx *context.T, principal security.Principal, roleStr string) (security.Blessings, error) {
	// The role server expects the client's blessing name to end with
	// RoleSuffix. This is to avoid accidentally granting role access to
	// anything else that might have been blessed by the same principal.
	roleSeekerB, err := bless(ctx, principal, role.RoleSuffix)
	if err != nil {
		return security.Blessings{}, err
	}
	pseeker, err := vsecurity.ForkPrincipal(
		principal,
		vsecurity.FixedBlessingsStore(roleSeekerB, principal.BlessingStore()),
		vsecurity.ImmutableBlessingRoots(v23.GetPrincipal(ctx).Roots()))
	if err != nil {
		return security.Blessings{}, err
	}
	if ctx, err = v23.WithPrincipal(ctx, pseeker); err != nil {
		return security.Blessings{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, timeoutFlag)
	defer cancel()
	return role.RoleClient(roleStr).SeekBlessings(ctx)
}
