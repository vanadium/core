// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rt_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"testing"

	"v.io/v23"
	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/x/lib/envvar"
	"v.io/x/lib/gosh"
	"v.io/x/ref"
	"v.io/x/ref/internal/logger"
	vsecurity "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/signals"
	"v.io/x/ref/test"
	"v.io/x/ref/test/v23test"
)

func TestInit(t *testing.T) {
	ref.EnvClearCredentials()
	ctx, shutdown := test.V23Init()
	defer shutdown()

	mgr := logger.Manager(ctx)
	fmt.Println(mgr)
	args := fmt.Sprintf("%s", mgr)
	expected := regexp.MustCompile("name=vanadium logdirs=\\[/tmp\\] logtostderr=true|false alsologtostderr=false|true max_stack_buf_size=4292608 v=[0-9] stderrthreshold=2 vmodule= vfilepath= log_backtrace_at=:0")

	if !expected.MatchString(args) {
		t.Errorf("unexpected default args: %s, want %s", args, expected)
	}
	p := v23.GetPrincipal(ctx)
	if p == nil {
		t.Fatalf("A new principal should have been created")
	}
	if p.BlessingStore() == nil {
		t.Fatalf("The principal must have a BlessingStore")
	}
	if got, _ := p.BlessingStore().Default(); got.IsZero() {
		t.Errorf("Principal().BlessingStore().Default() should not be the zero value")
	}
	if p.BlessingStore().ForPeer().IsZero() {
		t.Errorf("Principal().BlessingStore().ForPeer() should not be the zero value")
	}
}

var child = gosh.RegisterFunc("child", func() {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	mgr := logger.Manager(ctx)
	ctx.Infof("%s\n", mgr)
	fmt.Printf("%s\n", mgr)
	<-signals.ShutdownOnSignals(ctx)
})

func TestInitArgs(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	c := sh.FuncCmd(child)
	c.Args = append(c.Args, "--logtostderr=true", "--vmodule=*=3", "--", "foobar")
	c.PropagateOutput = true
	c.Start()
	c.S.Expect(fmt.Sprintf("name=vlog "+
		"logdirs=[%s] "+
		"logtostderr=true "+
		"alsologtostderr=true "+
		"max_stack_buf_size=4292608 "+
		"v=0 "+
		"stderrthreshold=2 "+
		"vmodule=*=3 "+
		"vfilepath= "+
		"log_backtrace_at=:0",
		os.TempDir()))
	c.Terminate(os.Interrupt)
	c.S.ExpectEOF()
}

func validatePrincipal(p security.Principal) error {
	if p == nil {
		return fmt.Errorf("nil principal")
	}
	remote, _ := p.BlessingStore().Default()
	call := security.NewCall(&security.CallParams{LocalPrincipal: p, RemoteBlessings: remote})
	ctx, cancel := context.RootContext()
	defer cancel()
	blessings, rejected := security.RemoteBlessingNames(ctx, call)
	if n := len(blessings); n != 1 {
		return fmt.Errorf("rt.Principal().BlessingStore().Default() return blessings:%v (rejected:%v), want exactly one recognized blessing", blessings, rejected)
	}
	return nil
}

func defaultBlessing(p security.Principal) string {
	remote, _ := p.BlessingStore().Default()
	call := security.NewCall(&security.CallParams{LocalPrincipal: p, RemoteBlessings: remote})
	ctx, cancel := context.RootContext()
	defer cancel()
	b, _ := security.RemoteBlessingNames(ctx, call)
	return b[0]
}

func tmpDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "rt_test_dir")
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	return dir
}

var principal = gosh.RegisterFunc("principal", func() error {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	p := v23.GetPrincipal(ctx)
	if err := validatePrincipal(p); err != nil {
		return err
	}
	fmt.Printf("DEFAULT_BLESSING=%s\n", defaultBlessing(p))
	return nil
})

// Runner runs 'principal' in a subprocess, then reports back with its own
// security info as well as its child's.
var runner = gosh.RegisterFunc("runner", func() error {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	sh := v23test.NewShell(nil, ctx)
	defer sh.Cleanup()

	p := v23.GetPrincipal(sh.Ctx)
	if err := validatePrincipal(p); err != nil {
		return err
	}
	fmt.Printf("RUNNER_DEFAULT_BLESSING=%v\n", defaultBlessing(p))
	c := sh.FuncCmd(principal)
	// Make sure the child gets its credentials from this shell (via agent), not
	// from the original EnvCredentials env var.
	delete(c.Vars, ref.EnvCredentials)
	c.PropagateOutput = true
	c.Run()
	return nil
})

func createCredentialsInDir(t *testing.T, dir string, blessing string) {
	principal, err := vsecurity.CreatePersistentPrincipal(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if err := vsecurity.InitDefaultBlessings(principal, blessing); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestPrincipalInheritance(t *testing.T) {
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	// Test that the child inherits from the parent's credentials correctly.
	// The running test process may or may not have a credentials directory set
	// up, so we have to use a 'runner' process to ensure the correct setup.
	cdir := tmpDir(t)
	defer os.RemoveAll(cdir)

	createCredentialsInDir(t, cdir, "test")

	// Directory supplied by the environment.
	c := sh.FuncCmd(runner)
	c.Vars[ref.EnvCredentials] = cdir
	c.Start()

	runnerBlessing := c.S.ExpectVar("RUNNER_DEFAULT_BLESSING")
	principalBlessing := c.S.ExpectVar("DEFAULT_BLESSING")
	if err := c.S.Error(); err != nil {
		t.Fatalf("failed to read input from children: %s", err)
	}
	c.Wait()

	wantRunnerBlessing := "test"
	wantPrincipalBlessing := "test:child"
	if runnerBlessing != wantRunnerBlessing || principalBlessing != wantPrincipalBlessing {
		t.Fatalf("unexpected default blessing: got runner %s, principal %s, want runner %s, principal %s", runnerBlessing, principalBlessing, wantRunnerBlessing, wantPrincipalBlessing)
	}
}

func TestPrincipalInit(t *testing.T) {
	// Runs the principal function and returns the child's default blessing.
	collect := func(sh *v23test.Shell, extraVars map[string]string, extraArgs ...string) string {
		c := sh.FuncCmd(principal)
		c.Vars = envvar.MergeMaps(c.Vars, extraVars)
		c.Args = append(c.Args, extraArgs...)
		c.Start()
		return c.S.ExpectVar("DEFAULT_BLESSING")
	}

	// Set aside any credentials dir specified via env.
	origEnvCredentials := os.Getenv(ref.EnvCredentials)
	defer os.Setenv(ref.EnvCredentials, origEnvCredentials)
	if err := os.Unsetenv(ref.EnvCredentials); err != nil {
		t.Fatal(err)
	}

	// Note, v23test.Shell uses the agent to pass credentials to its children.
	sh := v23test.NewShell(t, nil)
	defer sh.Cleanup()

	// Test that the shell uses the EnvAgentPath, not EnvCredentials, to pass
	// credentials to its children.
	c := sh.FuncCmd(principal)
	if _, ok := c.Vars[ref.EnvCredentials]; ok {
		t.Fatal("Did not expect child to have EnvCredentials set")
	}
	if _, ok := c.Vars[ref.EnvAgentPath]; !ok {
		t.Fatal("Expected child to have EnvAgentPath set")
	}

	// Test that with ref.EnvCredentials unset, the child runtime's principal is
	// initialized as expected.
	if got, want := collect(sh, nil), "root:child"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	// Test that credentials specified via the ref.EnvCredentials environment
	// variable take precedence over credentials from ref.EnvAgentPath.
	cdir1 := tmpDir(t)
	defer os.RemoveAll(cdir1)
	createCredentialsInDir(t, cdir1, "test_env")
	credEnv := map[string]string{ref.EnvCredentials: cdir1}

	if got, want := collect(sh, credEnv), "test_env"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Test that credentials specified via the command line (--v23.credentials)
	// take precedence over the ref.EnvCredentials and ref.EnvAgentPath
	// environment variables.
	cdir2 := tmpDir(t)
	defer os.RemoveAll(cdir2)
	createCredentialsInDir(t, cdir2, "test_cmd")

	if got, want := collect(sh, credEnv, "--v23.credentials="+cdir2), "test_cmd"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
