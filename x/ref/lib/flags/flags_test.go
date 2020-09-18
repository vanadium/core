// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags_test

import (
	"flag"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"v.io/x/ref"
	"v.io/x/ref/lib/flags"
)

func TestFlags(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	if fl, err := flags.CreateAndRegister(fs); fl != nil || err != nil {
		t.Fatalf("should have returned nil and no error")
	}
	fl, err := flags.CreateAndRegister(fs, flags.Runtime)
	if err != nil {
		t.Errorf("should have succeeded")
	}
	creds := "creddir"
	roots := []string{"ab:cd:ef"}
	args := []string{"--v23.credentials=" + creds, "--v23.namespace.root=" + roots[0]}
	fl.Parse(args, nil) //nolint:errcheck
	rtf := fl.RuntimeFlags()
	if got, want := rtf.NamespaceRoots.Roots, roots; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := rtf.Credentials, creds; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := fl.HasGroup(flags.Listen), false; got != want {
		t.Errorf("got %t, want %t", got, want)
	}
	// Make sure we have a deep copy.
	rtf.NamespaceRoots.Roots[0] = "oooh"
	rtf = fl.RuntimeFlags()
	if got, want := rtf.NamespaceRoots.Roots, roots; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestPermissionsFlags(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fl, err := flags.CreateAndRegister(fs, flags.Runtime, flags.Permissions)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"--v23.permissions.file=runtime:foo.json", "--v23.permissions.file=bar:bar.json", "--v23.permissions.file=baz:bar:baz.json"}
	fl.Parse(args, nil) //nolint:errcheck
	permsf := fl.PermissionsFlags()

	if got, want := permsf.PermissionsFile("runtime"), "foo.json"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := permsf.PermissionsFile("bar"), "bar.json"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := permsf.PermissionsFile("wombat"), ""; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := permsf.PermissionsFile("baz"), "bar:baz.json"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPermissionsLiteralFlags(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fl, err := flags.CreateAndRegister(fs, flags.Runtime, flags.Permissions)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{`--v23.permissions.literal={"x":"hedgehog"}`,
		`--v23.permissions.literal={"y":"badger"}`}
	fl.Parse(args, nil) //nolint:errcheck
	permsf := fl.PermissionsFlags()

	if got, want := permsf.PermissionsFile("runtime"), ""; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := permsf.PermissionsLiteral(), `{"x":"hedgehog"}{"y":"badger"}`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPermissionsLiteralBoth(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fl, err := flags.CreateAndRegister(fs, flags.Runtime, flags.Permissions)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"--v23.permissions.file=runtime:foo.json", `--v23.permissions.literal={"x":"hedgehog"}`}
	fl.Parse(args, nil) //nolint:errcheck
	permsf := fl.PermissionsFlags()

	if got, want := permsf.PermissionsFile("runtime"), "foo.json"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := permsf.PermissionsLiteral(), `{"x":"hedgehog"}`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	for _, tc := range []struct {
		args []string
		set  bool
	}{
		{[]string{""}, false},
		{[]string{"--v23.permissions.file=runtime:foo.json", `--v23.permissions.literal={"x":"hedgehog"}`}, true},
		{[]string{"--v23.permissions.file=runtime:foo.json"}, true},
		{[]string{`--v23.permissions.literal={"x":"hedgehog"}`}, true},
	} {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		fl, err := flags.CreateAndRegister(fs, flags.Runtime, flags.Permissions)
		if err != nil {
			t.Fatal(err)
		}
		if err := fl.Parse(tc.args, nil); err != nil {
			t.Fatal(err)
		}
		permsf = fl.PermissionsFlags()
		if got, want := permsf.ExplicitlySpecified(), tc.set; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestFlagError(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.SetOutput(ioutil.Discard)
	fl, err := flags.CreateAndRegister(fs, flags.Runtime)
	if err != nil {
		t.Fatal(err)
	}
	addr := "192.168.10.1:0"
	args := []string{"--xxxv23.tcp.address=" + addr, "not an arg"}
	if err := fl.Parse(args, nil); err == nil {
		t.Fatalf("expected this to fail!")
	}
	if got, want := len(fl.Args()), 1; got != want {
		t.Errorf("got %d, want %d [args: %v]", got, want, fl.Args())
	}

	fs = flag.NewFlagSet("test", flag.ContinueOnError)
	fl, err = flags.CreateAndRegister(fs, flags.Permissions)
	if err != nil {
		t.Fatal(err)
	}
	args = []string{"--v23.permissions.file=noname"}
	err = fl.Parse(args, nil)
	if err == nil {
		t.Fatalf("expected this to fail!")
	}
}

func TestFlagsGroups(t *testing.T) {
	fl, err := flags.CreateAndRegister(flag.NewFlagSet("test", flag.ContinueOnError), flags.Runtime, flags.Listen)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := fl.HasGroup(flags.Listen), true; got != want {
		t.Errorf("got %t, want %t", got, want)
	}
	addr := "192.168.10.1:0"
	roots := []string{"ab:cd:ef"}
	args := []string{"--v23.tcp.address=" + addr, "--v23.namespace.root=" + roots[0]}
	fl.Parse(args, nil) //nolint:errcheck
	lf := fl.ListenFlags()
	if got, want := fl.RuntimeFlags().NamespaceRoots.Roots, roots; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := lf.Addrs[0].Address, addr; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

const (
	rootEnvVar  = ref.EnvNamespacePrefix
	rootEnvVar0 = ref.EnvNamespacePrefix + "0"
)

func TestEnvVars(t *testing.T) {
	oldcreds := os.Getenv(ref.EnvCredentials)
	defer os.Setenv(ref.EnvCredentials, oldcreds)

	oldroot := os.Getenv(rootEnvVar)
	oldroot0 := os.Getenv(rootEnvVar0)
	defer os.Setenv(rootEnvVar, oldroot)
	defer os.Setenv(rootEnvVar0, oldroot0)

	os.Setenv(ref.EnvCredentials, "bar")
	fl, err := flags.CreateAndRegister(flag.NewFlagSet("test", flag.ContinueOnError), flags.Runtime)
	if err != nil {
		t.Fatal(err)
	}
	if err := fl.Parse([]string{}, nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	rtf := fl.RuntimeFlags()
	if got, want := rtf.Credentials, "bar"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	if err := fl.Parse([]string{"--v23.credentials=baz"}, nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	rtf = fl.RuntimeFlags()
	if got, want := rtf.Credentials, "baz"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	os.Setenv(rootEnvVar, "a:1")
	os.Setenv(rootEnvVar0, "a:2")
	fl, err = flags.CreateAndRegister(flag.NewFlagSet("test", flag.ContinueOnError), flags.Runtime)
	if err != nil {
		t.Fatal(err)
	}
	if err := fl.Parse([]string{}, nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	rtf = fl.RuntimeFlags()
	if got, want := rtf.NamespaceRoots.Roots, []string{"a:1", "a:2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if err := fl.Parse([]string{"--v23.namespace.root=b:1", "--v23.namespace.root=b:2", "--v23.namespace.root=b:3", "--v23.credentials=b:4"}, nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	rtf = fl.RuntimeFlags()
	if got, want := rtf.NamespaceRoots.Roots, []string{"b:1", "b:2", "b:3"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := rtf.Credentials, "b:4"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDefaults(t *testing.T) {
	oldroot := os.Getenv(rootEnvVar)
	oldroot0 := os.Getenv(rootEnvVar0)
	defer os.Setenv(rootEnvVar, oldroot)
	defer os.Setenv(rootEnvVar0, oldroot0)

	os.Setenv(rootEnvVar, "")
	os.Setenv(rootEnvVar0, "")

	fl, err := flags.CreateAndRegister(flag.NewFlagSet("test", flag.ContinueOnError), flags.Runtime, flags.Permissions)
	if err != nil {
		t.Fatal(err)
	}
	if err := fl.Parse([]string{}, nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	rtf := fl.RuntimeFlags()
	if got, want := rtf.NamespaceRoots.Roots, []string{"/(dev.v.io:r:vprod:service:mounttabled)@ns.dev.v.io:8101"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	permsf := fl.PermissionsFlags()
	if got, want := permsf.PermissionsFile(""), ""; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	merged := flags.MergedForTest()
	if got, want := merged["test-default-flag-not-for-real-use"].(string), "test-value"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestDuplicateFlags(t *testing.T) {
	fl, err := flags.CreateAndRegister(flag.NewFlagSet("test", flag.ContinueOnError), flags.Listen)
	if err != nil {
		t.Fatal(err)
	}
	if err := fl.Parse([]string{
		"--v23.tcp.address=172.0.0.1:10", "--v23.tcp.address=172.0.0.1:10", "--v23.tcp.address=172.0.0.1:34",
		"--v23.tcp.protocol=tcp", "--v23.tcp.address=172.0.0.1:10", "--v23.tcp.address=172.0.0.1:10", "--v23.tcp.address=172.0.0.1:34",
		"--v23.tcp.protocol=ws", "--v23.tcp.address=172.0.0.1:10", "--v23.tcp.address=172.0.0.1:34", "--v23.tcp.address=172.0.0.1:34"}, nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	lf := fl.ListenFlags()
	if got, want := len(lf.Addrs), 6; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	expected := flags.ListenAddrs{
		{"wsh", "172.0.0.1:10"},
		{"wsh", "172.0.0.1:34"},
		{"tcp", "172.0.0.1:10"},
		{"tcp", "172.0.0.1:34"},
		{"ws", "172.0.0.1:10"},
		{"ws", "172.0.0.1:34"},
	}
	if got, want := lf.Addrs, expected; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if err := fl.Parse([]string{
		"--v23.tcp.address=172.0.0.1:10", "--v23.tcp.address=172.0.0.1:10", "--v23.tcp.address=172.0.0.1:34",
		"--v23.tcp.protocol=tcp", "--v23.tcp.address=172.0.0.1:10", "--v23.tcp.address=127.0.0.1:34", "--v23.tcp.address=127.0.0.1:34",
		"--v23.tcp.protocol=ws", "--v23.tcp.address=172.0.0.1:10", "--v23.tcp.address=127.0.0.1:34", "--v23.tcp.address=127.0.0.1:34"}, nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got, want := len(lf.Addrs), 6; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	if got, want := lf.Addrs, expected; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}

	fl, err = flags.CreateAndRegister(flag.NewFlagSet("test", flag.ContinueOnError), flags.Runtime)
	if err != nil {
		t.Fatal(err)
	}

	if err := fl.Parse([]string{"--v23.namespace.root=ab", "--v23.namespace.root=xy", "--v23.namespace.root=ab"}, nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}

	rf := fl.RuntimeFlags()
	if got, want := len(rf.NamespaceRoots.Roots), 2; got != want {
		t.Errorf("got %d, want %d", got, want)
	}
	if got, want := rf.NamespaceRoots.Roots, []string{"ab", "xy"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestConfig(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var testFlag1, testFlag2 string
	fs.StringVar(&testFlag1, "test_flag1", "default1", "")
	fs.StringVar(&testFlag2, "test_flag2", "default2", "")
	fl, err := flags.CreateAndRegister(fs, flags.Runtime)
	if err != nil {
		t.Fatal(err)
	}
	args := []string{
		"--v23.namespace.root=argRoot1",
		"--v23.namespace.root=argRoot2",
		"--v23.vtrace.cache-size=1234",
	}
	config := map[string]string{
		"v23.namespace.root":       "configRoot",
		"v23.credentials":          "configCreds",
		"v23.vtrace.cache-size":    "4321",
		"test_flag1":               "test value",
		"flag.that.does.not.exist": "some value",
	}
	if err := fl.Parse(args, config); err != nil {
		t.Errorf("Parse(%v, %v) failed: %v", args, config, err)
	}
	rtf := fl.RuntimeFlags()
	if got, want := rtf.NamespaceRoots.Roots, []string{"argRoot1", "argRoot2", "configRoot"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Namespace roots: got %v, want %v", got, want)
	}
	if got, want := rtf.Credentials, "configCreds"; got != want {
		t.Errorf("Credentials: got %v, want %v", got, want)
	}
	if got, want := testFlag1, "test value"; got != want {
		t.Errorf("Test flag 1: got %v, want %v", got, want)
	}
	if got, want := testFlag2, "default2"; got != want {
		t.Errorf("Test flag 2: got %v, want %v", got, want)
	}
	if got, want := rtf.CacheSize, 4321; got != want {
		t.Errorf("Test flag 2: got %v, want %v", got, want)
	}
}

func TestRefreshDefaults(t *testing.T) {
	orig := flags.DefaultNamespaceRoots()
	defer flags.SetDefaultNamespaceRoots(orig...)
	defer flags.SetDefaultHostPort(":0")
	defer flags.SetDefaultProtocol("wsh")

	nsRoot := "/127.0.0.1:8101"
	hostPort := "128.0.0.1:11"
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fl, err := flags.CreateAndRegister(fs, flags.Runtime, flags.Listen)
	if err != nil {
		t.Fatal(err)
	}
	// It's possible to set defaults after CreateAndRegister, but before Parse.
	flags.SetDefaultNamespaceRoots(nsRoot)
	flags.SetDefaultHostPort(hostPort)
	flags.SetDefaultProtocol("tcp6")
	if err := fl.Parse([]string{}, nil); err != nil {
		t.Fatal(err)
	}

	rtf := fl.RuntimeFlags()
	if got, want := rtf.NamespaceRoots.Roots, []string{nsRoot}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	lf := fl.ListenFlags()
	want := flags.ListenAddrs{struct{ Protocol, Address string }{"tcp6", hostPort}}
	if got := lf.Addrs; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	changed := "/128.1.1.1:1"
	flags.SetDefaultNamespaceRoots(changed)
	if err := fl.Parse([]string{}, nil); err != nil {
		t.Fatal(err)
	}
	rtf = fl.RuntimeFlags()
	if got, want := rtf.NamespaceRoots.Roots, []string{changed}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestRefreshAlreadySetDefaults(t *testing.T) {
	orig := flags.DefaultNamespaceRoots()
	defer flags.SetDefaultNamespaceRoots(orig...)
	defer flags.SetDefaultHostPort(":0")
	defer flags.SetDefaultProtocol("wsh")

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fl, err := flags.CreateAndRegister(fs, flags.Runtime, flags.Listen)
	if err != nil {
		t.Fatal(err)
	}
	nsRoot := "/127.0.1.1:10"
	hostPort := "127.0.0.1:10"
	fl.Parse([]string{"--v23.namespace.root", nsRoot, "--v23.tcp.address", hostPort}, nil) //nolint:errcheck
	rtf := fl.RuntimeFlags()
	if got, want := rtf.NamespaceRoots.Roots, []string{nsRoot}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	flags.SetDefaultNamespaceRoots("/128.1.1.1:2")
	flags.SetDefaultHostPort("128.0.0.1:11")
	if err := fl.Parse([]string{}, nil); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	rtf = fl.RuntimeFlags()
	if got, want := rtf.NamespaceRoots.Roots, []string{nsRoot}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	lf := fl.ListenFlags()
	want := flags.ListenAddrs{struct{ Protocol, Address string }{"wsh", hostPort}}
	if got := lf.Addrs; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
