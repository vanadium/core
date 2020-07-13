// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package access_test

import (
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/internal/sectest"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/security/access/internal"
	"v.io/v23/vdl"
	"v.io/v23/verror"
)

func authorize(authorizer security.Authorizer, params *security.CallParams) error {
	ctx, cancel := context.RootContext()
	defer cancel()
	return authorizer.Authorize(ctx, security.NewCall(params))
}

func enforceable(al access.AccessList, p security.Principal) error {
	ctx, cancel := context.RootContext()
	defer cancel()
	return al.Enforceable(ctx, p)
}

func TestAccessListAuthorizerECDSA(t *testing.T) {
	testAccessListAuthorizer(t,
		sectest.NewECDSAPrincipalP256TrustAllRoots(t),
		sectest.NewECDSAPrincipalP256TrustAllRoots(t),
		sectest.NewECDSAPrincipalP256TrustAllRoots(t),
	)
}

func TestAccessListAuthorizerED25519(t *testing.T) {
	testAccessListAuthorizer(t,
		sectest.NewED25519PrincipalTrustAllRoots(t),
		sectest.NewED25519PrincipalTrustAllRoots(t),
		sectest.NewED25519PrincipalTrustAllRoots(t),
	)
}

func testAccessListAuthorizer(t *testing.T, pali, pbob, pche security.Principal) {
	var (
		expired, _ = security.NewExpiryCaveat(time.Now().Add(-1 * time.Minute))

		ali, _                = pbob.BlessSelf("ali")
		bob, _                = pbob.BlessSelf("bob")
		bobSelf, _            = pbob.BlessSelf("bob:self")
		bobDelegate, _        = pbob.Bless(pche.PublicKey(), bob, "delegate:che", security.UnconstrainedUse())
		bobDelegateBad, _     = pbob.Bless(pche.PublicKey(), bob, "delegate:badman", security.UnconstrainedUse())
		bobDelegateExpired, _ = pbob.Bless(pche.PublicKey(), bob, "delegate:che", expired)

		bobUnion, _    = security.UnionOfBlessings(bobDelegate, bobDelegateExpired)
		bobBadUnion, _ = security.UnionOfBlessings(bobDelegateBad, bobDelegateExpired)

		acl = access.AccessList{
			In:    []security.BlessingPattern{"bob:delegate", "bob:self"},
			NotIn: []string{"bob:delegate:badman"},
		}

		tests = []struct {
			B         security.Blessings
			Authorize bool
		}{
			{bob, false},
			{bobSelf, true},
			{bobDelegate, true},
			{bobDelegateBad, false},
			{bobDelegateExpired, false},
			{bobUnion, true},
			{bobBadUnion, false},
		}
	)
	for _, test := range tests {
		err := authorize(acl, &security.CallParams{
			LocalPrincipal:  pali,
			LocalBlessings:  ali,
			RemoteBlessings: test.B,
		})
		if test.Authorize && err != nil {
			t.Errorf("%v: Got error %v", test.B, err)
		} else if errid := verror.ErrorID(err); !test.Authorize && errid != access.ErrAccessListMatch.ID {
			t.Errorf("%v: Got error %v (errorid=%v) want errorid %v", test.B, err, errid, access.ErrAccessListMatch.ID)
		}
	}
}

func TestAccessListEnforceableECDSA(t *testing.T) {
	testAccessListEnforceable(t, sectest.NewECDSAPrincipalP256TrustAllRoots(t))
}

func TestAccessListEnforceableED25519(t *testing.T) {
	testAccessListEnforceable(t, sectest.NewED25519PrincipalTrustAllRoots(t))
}

func testAccessListEnforceable(t *testing.T, p security.Principal) {
	if key, err := p.PublicKey().MarshalBinary(); err != nil {
		t.Fatal(err)
	} else {
		for _, pattern := range []security.BlessingPattern{"ali:spouse:$", "bob:friend"} {
			if err := p.Roots().Add(key, pattern); err != nil {
				t.Fatal(err)
			}
		}
	}

	type (
		bp []security.BlessingPattern // shorthand
		s  []string                   // shorthand
	)

	tests := []struct {
		al       access.AccessList
		errID    verror.ID
		rejected []security.BlessingPattern
	}{
		{
			al: access.AccessList{
				In:    bp{"$", "ali:spouse:$", "bob:friend:$", "bob:friend:colleague"},
				NotIn: s{"ali:spouse:friend", "bob"}, // NotIn patterns don't matter.
			},
		},
		{
			al: access.AccessList{
				In:    bp{"bob:friend:$", "ali:$:spouse", "bob::friend", "bob:..."}, // invalid patterns are rejected
				NotIn: s{"ali:spouse:friend", "bob"},
			},
			errID:    access.ErrUnenforceablePatterns.ID,
			rejected: bp{"ali:$:spouse", "bob::friend", "bob:..."},
		},
		{
			al: access.AccessList{
				In:    bp{"ali:spouse:$", "bob:friend:$", "ali", "ali:$", "ali:spouse", "ali:spouse:friend", "bob", "bob:$", "bob:spouse"}, // unrecognized patterns are rejected
				NotIn: s{"ali:spouse:friend", "bob"},
			},
			errID:    access.ErrUnenforceablePatterns.ID,
			rejected: bp{"ali", "ali:$", "ali:spouse", "ali:spouse:friend", "bob", "bob:$", "bob:spouse"},
		},
		{
			al: access.AccessList{
				In:    bp{"..."},
				NotIn: s{"bob:friend"},
			},
			errID: access.ErrInvalidOpenAccessList.ID,
		},
		{
			al: access.AccessList{
				In: bp{"...", "bob:friend"},
			},
			errID: access.ErrInvalidOpenAccessList.ID,
		},
	}
	for _, test := range tests {
		gotErr := enforceable(test.al, p)
		if (test.errID == "" && gotErr != nil) || (verror.ErrorID(gotErr) != test.errID) {
			t.Errorf("%v.Enforceable(...): got error %v, want error with ID %v", test.al, gotErr, test.errID)
		}
		if test.errID != access.ErrUnenforceablePatterns.ID {
			continue
		}

		gotRejected := access.IsUnenforceablePatterns(gotErr)
		sort.Sort(byPattern(gotRejected))
		sort.Sort(byPattern(test.rejected))
		if !reflect.DeepEqual(gotRejected, test.rejected) {
			t.Errorf("IsUnenforceablePatterns(%v): got rejected pattern %v, want %v", gotErr, gotRejected, test.rejected)
		}
	}
}

type byPattern []security.BlessingPattern

func (a byPattern) Len() int           { return len(a) }
func (a byPattern) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byPattern) Less(i, j int) bool { return a[i] < a[j] }

// TestPermissionsAuthorizer is both a test and a demonstration of the use of
// the access.PermissionsAuthorizer and interaction with interface specification
// in VDL.
func TestPermissionsAuthorizer(t *testing.T) {
	type P []security.BlessingPattern
	type S []string
	// access.Permissions to test against.
	perms := access.Permissions{
		"R": {
			In: P{security.AllPrincipals},
		},
		"W": {
			In:    P{"ali:family", "bob", "che:$"},
			NotIn: S{"bob:acquaintances"},
		},
		"X": {
			In: P{"ali:family:boss:$", "superman:$"},
		},
	}
	type testcase struct {
		Method string
		Client security.Blessings
	}

	authorizer, err := access.PermissionsAuthorizer(perms, vdl.TypeOf(internal.Read))
	if err != nil {
		t.Fatalf("Could not create authorizer: %v", err)
	}

	var (
		// Two principals: The "server" and the "client"
		pserver   = sectest.NewECDSAPrincipalP256TrustAllRoots(t)
		pclient   = sectest.NewED25519PrincipalTrustAllRoots(t)
		server, _ = pserver.BlessSelf("server")

		// B generates the provided blessings for the client and ensures
		// that the server will recognize them.
		B = func(names ...string) security.Blessings {
			var ret security.Blessings
			for _, name := range names {
				b, err := pclient.BlessSelf(name)
				if err != nil {
					t.Fatalf("%q: %v", name, err)
				}
				// Since this test uses trustAllRoots, no need
				// to call AddToRoots(pserver, b) to get the server
				// to recognize the client.
				if ret, err = security.UnionOfBlessings(ret, b); err != nil {
					t.Fatal(err)
				}
			}
			return ret
		}

		run = func(test testcase) error {
			return authorize(authorizer, &security.CallParams{
				LocalPrincipal:  pserver,
				LocalBlessings:  server,
				RemoteBlessings: test.Client,
				Method:          test.Method,
				MethodTags:      methodTags(test.Method),
			})
		}
	)

	// Test cases where access should be granted to methods with tags on
	// them.
	for _, test := range []testcase{
		{"Get", security.Blessings{}},
		{"Get", B("ali")},
		{"Get", B("bob:friend", "che:enemy")},

		{"Put", B("ali:family:mom")},
		{"Put", B("bob:friends")},
		{"Put", B("bob:acquaintances:carol", "che")}, // Access granted because of "che"

		{"Resolve", B("superman")},
		{"Resolve", B("ali:family:boss")},
	} {
		if err := run(test); err != nil {
			t.Errorf("Access denied to method %q to %v: %v", test.Method, test.Client, err)
		}
	}
	// Test cases where access should be denied.
	for _, test := range []testcase{
		// Nobody is denied access to "Get"
		{"Put", B("ali", "bob:acquaintances", "bob:acquaintances:dave", "che:friend", "dave")},
		{"Resolve", B("ali", "ali:friend", "ali:family", "ali:family:friend", "alice:family:boss:friend", "superman:friend")},
		// Since there are no tags on the NoTags method, it has an
		// empty AccessList.  No client will have access.
		{"NoTags", B("ali", "ali:family:boss", "bob", "che", "superman")},
	} {
		if err := run(test); err == nil {
			t.Errorf("Access to %q granted to %v", test.Method, test.Client)
		}
	}
}

func TestPermissionsAuthorizerSelfRPCsECDSA(t *testing.T) {
	testPermissionsAuthorizerSelfRPCs(t, sectest.NewECDSAPrincipalP256TrustAllRoots(t))
}

func TestPermissionsAuthorizerSelfRPCsED25519(t *testing.T) {
	testPermissionsAuthorizerSelfRPCs(t, sectest.NewED25519PrincipalTrustAllRoots(t))
}

func testPermissionsAuthorizerSelfRPCs(t *testing.T, p security.Principal) {
	var (
		// Client and server are the same principal, though have
		// different blessings.
		client, _ = p.BlessSelf("client")
		server, _ = p.BlessSelf("server")
		// Authorizer with access.Permissions that grant access to noone.
		authorizer, _ = access.PermissionsAuthorizer(access.Permissions{"R": {In: []security.BlessingPattern{"nobody:$"}}}, vdl.TypeOf(internal.Read))
	)
	for _, test := range []string{"Put", "Get", "Resolve", "NoTags"} {
		params := &security.CallParams{
			LocalPrincipal:  p,
			LocalBlessings:  server,
			RemoteBlessings: client,
			Method:          test,
			MethodTags:      methodTags(test),
		}
		// Self-RPCs should not be treated differently.
		if err := authorize(authorizer, params); err == nil {
			t.Errorf("Access to %q granted to %v", test, client)
		}
	}
}

func TestPermissionsAuthorizerWithNilAccessList(t *testing.T) {
	var (
		authorizer, _ = access.PermissionsAuthorizer(nil, vdl.TypeOf(internal.Read))
		pserver       = sectest.NewED25519PrincipalTrustAllRoots(t)
		pclient       = sectest.NewECDSAPrincipalP256TrustAllRoots(t)
		server, _     = pserver.BlessSelf("server")
		client, _     = pclient.BlessSelf("client")
	)
	for _, test := range []string{"Put", "Get", "Resolve", "NoTags"} {
		params := &security.CallParams{
			LocalPrincipal:  pserver,
			LocalBlessings:  server,
			RemoteBlessings: client,
			Method:          test,
			MethodTags:      methodTags(test),
		}
		if err := authorize(authorizer, params); err == nil {
			t.Errorf("nil access.Permissions authorized method %q, %v", test, err)
		}
	}
}

func TestPermissionsAuthorizerFromFile(t *testing.T) {
	file, err := ioutil.TempFile("", "TestPermissionsAuthorizerFromFile")
	if err != nil {
		t.Fatal(err)
	}
	filename := file.Name()
	file.Close()
	defer os.Remove(filename)

	var (
		authorizer, _  = access.PermissionsAuthorizerFromFile(filename, vdl.TypeOf(internal.Read))
		pserver        = sectest.NewED25519PrincipalTrustAllRoots(t)
		pclient        = sectest.NewECDSAPrincipalP256TrustAllRoots(t)
		server, _      = pserver.BlessSelf("alice")
		alicefriend, _ = pserver.Bless(pclient.PublicKey(), server, "friend:bob", security.UnconstrainedUse())
		params         = &security.CallParams{
			LocalPrincipal:  pserver,
			LocalBlessings:  server,
			RemoteBlessings: alicefriend,
			Method:          "Get",
			MethodTags:      methodTags("Get"),
		}
	)
	// Since this test is using trustAllRoots{}, do not need
	// AddToRoots(pserver, server) to make pserver recognize itself as an
	// authority on blessings matching "alice".

	// "alice:friend:bob" should not have access to internal.Read methods like Get.
	if err := authorize(authorizer, params); err == nil {
		t.Fatalf("Expected authorization error as %v is not on the AccessList for Read operations", alicefriend)
	}
	// Rewrite the file giving access
	if err := ioutil.WriteFile(filename, []byte(`{"R": { "In":["alice:friend"] }}`), 0600); err != nil {
		t.Fatal(err)
	}
	// Now should have access
	if err := authorize(authorizer, params); err != nil {
		t.Fatal(err)
	}
}

func TestTagTypeMustBeString(t *testing.T) {
	type I int
	if auth, err := access.PermissionsAuthorizer(access.Permissions{}, vdl.TypeOf(I(0))); err == nil || auth != nil {
		t.Errorf("Got (%v, %v), wanted error since tag type is not a string", auth, err)
	}
	if auth, err := access.PermissionsAuthorizerFromFile("does_not_matter", vdl.TypeOf(I(0))); err == nil || auth != nil {
		t.Errorf("Got (%v, %v), wanted error since tag type is not a string", auth, err)
	}
}

// TestMultipleTags will fail if PermissionsAuthorizer authorizes a request to
// a method with multiple tags. Alternatives considered were "must be present
// in access list for *all* tags", "must be present in access list for *any*
// tag" and "must be present in access list for *first* tag". Cases can be made
// for any of them and until there is more mileage on the use of multiple tags,
// take the conservative approach of considering multiple tags an error.
func TestMultipleTags(t *testing.T) {
	var (
		allowAll = access.AccessList{In: []security.BlessingPattern{security.AllPrincipals}}
		perms    = access.Permissions{
			"R": allowAll,
			"W": allowAll,
		}
		authorizer, _ = access.PermissionsAuthorizer(perms, vdl.TypeOf(internal.Read))
		pserver       = sectest.NewECDSAPrincipalP256TrustAllRoots(t)
		pclient       = sectest.NewED25519PrincipalTrustAllRoots(t)
		server, _     = pserver.BlessSelf("server")
		client, _     = pclient.BlessSelf("client")
		call          = &security.CallParams{
			LocalPrincipal:  pserver,
			LocalBlessings:  server,
			RemoteBlessings: client,
			Method:          "Get",
		}
		tags = []*vdl.Value{vdl.ValueOf(internal.Read), vdl.ValueOf(internal.Write)}
	)
	// Access should be granted if any of the two tags are present.
	for _, tag := range tags {
		call.MethodTags = []*vdl.Value{tag}
		if err := authorize(authorizer, call); err != nil {
			t.Errorf("%v: %v", tag, err)
		}
	}
	// But not if both the tags are present.
	call.MethodTags = tags
	if err := authorize(authorizer, call); err == nil {
		t.Errorf("Expected error since there are multiple tags on the method")
	}
}

func methodTags(name string) []*vdl.Value {
	server := internal.MyObjectServer(nil)
	for _, iface := range server.Describe__() {
		for _, method := range iface.Methods {
			if method.Name == name {
				return method.Tags
			}
		}
	}
	return nil
}
