// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package securityflag

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"v.io/x/ref/lib/flags"

	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/x/lib/gosh"
)

var (
	perms1 = access.Permissions{}
	perms2 = access.Permissions{
		string(access.Read): access.AccessList{
			In: []security.BlessingPattern{"v23:alice:$", "v23:bob"},
		},
		string(access.Write): access.AccessList{
			In: []security.BlessingPattern{"v23:alice:$"},
		},
	}

	expectedAuthorizer = map[string]security.Authorizer{
		"empty":  access.TypicalTagTypePermissionsAuthorizer(perms1),
		"perms2": access.TypicalTagTypePermissionsAuthorizer(perms2),
	}
)

var permFromFlag = gosh.RegisterFunc("permFromFlag", func() {
	pf := flags.PermissionsFlags{}
	flags.RegisterPermissionsFlags(flag.CommandLine, &pf)
	flag.Parse()
	nfargs := flag.CommandLine.Args()
	perms, err := PermissionsFromSpec(access.PermissionsSpec{
		Files:   pf.PermissionsNamesAndFiles(),
		Literal: pf.PermissionsLiteral(),
	}, "")
	if err != nil {
		fmt.Printf("PermissionsFromFlag() failed: %v", err)
		return
	}
	got := access.TypicalTagTypePermissionsAuthorizer(perms)
	want := expectedAuthorizer[nfargs[0]]
	if !reflect.DeepEqual(got, want) {
		fmt.Printf("args %#v\n", os.Args)
		fmt.Printf("AuthorizerFromFlags() got Authorizer: %v, want: %v", got, want)
	}
})

func writePermissionsToFile(perms access.Permissions) (string, error) {
	f, err := ioutil.TempFile("", "permissions")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := access.WritePermissions(f, perms); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func TestNewAuthorizerOrDie(t *testing.T) {
	sh := gosh.NewShell(t)
	defer sh.Cleanup()

	// Create a file.
	filename, err := writePermissionsToFile(perms2)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(filename)

	testdata := []struct {
		flags []string
		auth  string
	}{
		{
			flags: []string{"--v23.permissions.file", "runtime:" + filename},
			auth:  "perms2",
		},
		{
			flags: []string{"--v23.permissions.literal", "{}"},
			auth:  "empty",
		},
		{
			flags: []string{"--v23.permissions.literal", `{"Read": {"In":["v23:alice:$", "v23:bob"]}, "Write": {"In":["v23:alice:$"]}}`},
			auth:  "perms2",
		},
	}
	for _, td := range testdata {
		fp := append(td.flags, td.auth)
		c := sh.FuncCmd(permFromFlag)
		c.Args = append(c.Args, fp...)
		if stdout := c.Stdout(); stdout != "" {
			t.Errorf(stdout)
		}
	}
}

func TestMain(m *testing.M) {
	gosh.InitMain()
	os.Exit(m.Run())
}
