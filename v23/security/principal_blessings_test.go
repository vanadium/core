// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/internal/sectest"
	"v.io/v23/security"
	"v.io/v23/uniqueid"
	"v.io/v23/verror"
)

func TestBlessSelfECDSA(t *testing.T) {
	testBlessSelf(t, sectest.NewECDSAPrincipalP256(t), sectest.NewECDSAPrincipalP256(t))
}

func TestBlessSelfED25519(t *testing.T) {
	testBlessSelf(t, sectest.NewED25519Principal(t), sectest.NewED25519Principal(t))
}

func TestBlessSel(t *testing.T) {
	testBlessSelf(t, sectest.NewECDSAPrincipalP256(t), sectest.NewED25519Principal(t))
	testBlessSelf(t, sectest.NewED25519Principal(t), sectest.NewECDSAPrincipalP256(t))
}

func testBlessSelf(t *testing.T, tp, p security.Principal) {
	// tp is the principal where blessings are tested.
	call := func(method string) security.CallParams {
		return security.CallParams{
			LocalPrincipal: tp,
			Method:         method,
		}
	}
	alice, err := p.BlessSelf("alice", sectest.NewMethodCaveat(t, "Method"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(alice.PublicKey(), p.PublicKey()) {
		t.Errorf("Public key mismatch. Principal: %v, Blessing: %v", p.PublicKey(), alice.PublicKey())
	}
	if err := checkBlessings(alice, call("Foo")); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(alice, call("Method")); err != nil {
		t.Error(err)
	}
	sectest.AddToRoots(t, tp, alice)
	if err := checkBlessings(alice, call("Foo")); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(alice, call("Method"), "alice"); err != nil {
		t.Error(err)
	}
}

func TestBlessECDSA(t *testing.T) {
	testBless(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func TestBlessED25519(t *testing.T) {
	testBless(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func TestBless(t *testing.T) {
	testBless(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
	testBless(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func testBless(t *testing.T, tp, p1, p2, p3 security.Principal) {
	// tp is the principal where blessings are tested
	alice := sectest.BlessSelf(t, p1, "alice")
	call := func(method, suffix string) security.CallParams {
		return security.CallParams{
			LocalPrincipal: tp,
			Method:         method,
			Suffix:         suffix,
		}
	}
	sectest.AddToRoots(t, tp, alice)
	// p1 blessing p2 'with' empty Blessings should fail.
	if b, err := p1.Bless(p2.PublicKey(), security.Blessings{}, "friend", security.UnconstrainedUse()); err == nil {
		t.Errorf("p1 was able to extend a nil blessing to produce: %v", b)
	}
	// p1 blessing p2 as "alice:friend" for "Suffix.Method"
	friend, err := p1.Bless(p2.PublicKey(), alice, "friend", sectest.NewMethodCaveat(t, "Method"), sectest.NewSuffixCaveat(t, "Suffix"))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(friend.PublicKey(), p2.PublicKey()) {
		t.Errorf("Public key mismatch. Principal: %v, Blessing: %v", p2.PublicKey(), friend.PublicKey())
	}
	if err := checkBlessings(friend, call("Method", "OtherSuffix")); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, call("OtherMethod", "Suffix")); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, call("Method", "Suffix"), "alice:friend"); err != nil {
		t.Error(err)
	}
	// p1.Bless should not mess with the certificate chains of "alice" itself.
	if err := checkBlessings(alice, call("OtherMethod", "OtherSuffix"), "alice"); err != nil {
		t.Error(err)
	}

	// p2 should not be able to bless p3 as "alice:friend"
	blessings, err := p2.Bless(p3.PublicKey(), alice, "friend", security.UnconstrainedUse())
	if !blessings.IsZero() {
		t.Errorf("p2 was able to extend a blessing bound to p1 to produce: %v", blessings)
	} else if err = matchesError(err, "cannot extend blessing with public key"); err != nil {
		t.Fatal(err)
	}
}

func TestThirdPartyCaveatsECDSA(t *testing.T) {
	testThirdPartyCaveats(t, sectest.NewECDSAPrincipalP256(t), sectest.NewECDSAPrincipalP256(t))
}
func TestThirdPartyCaveatsED25519(t *testing.T) {
	testThirdPartyCaveats(t, sectest.NewED25519Principal(t), sectest.NewED25519Principal(t))
}

func TestThirdPartyCaveats(t *testing.T) {
	testThirdPartyCaveats(t, sectest.NewECDSAPrincipalP256(t), sectest.NewED25519Principal(t))
	testThirdPartyCaveats(t, sectest.NewED25519Principal(t), sectest.NewECDSAPrincipalP256(t))

}

func testThirdPartyCaveats(t *testing.T, p1, p2 security.Principal) {
	var (
		tp1 = sectest.NewPublicKeyUnconstrainedCaveat(t, p1, "peoria")
		tp2 = sectest.NewPublicKeyUnconstrainedCaveat(t, p1, "london")
		tp3 = sectest.NewPublicKeyUnconstrainedCaveat(t, p1, "delhi")
		c1  = sectest.NewMethodCaveat(t, "method")
		c2  = sectest.NewExpiryCaveat(t, time.Now())
	)

	b, err := p1.BlessSelf("alice", tp1, c1, tp2)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := b.ThirdPartyCaveats(), []security.Caveat{tp1, tp2}; !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}
	if b, err = p1.Bless(p2.PublicKey(), b, "friend", tp3, c2); err != nil {
		t.Fatal(err)
	}
	if got, want := b.ThirdPartyCaveats(), []security.Caveat{tp1, tp2, tp3}; !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}
}

func TestBlessingNamesECDSA(t *testing.T) {
	testBlessingNames(t, sectest.NewECDSAPrincipalP256(t), sectest.NewECDSAPrincipalP256(t))
}

func TestBlessingNamesED25519(t *testing.T) {
	testBlessingNames(t, sectest.NewED25519Principal(t), sectest.NewED25519Principal(t))
}

func TestBlessingNames(t *testing.T) {
	testBlessingNames(t, sectest.NewED25519Principal(t), sectest.NewECDSAPrincipalP256(t))
	testBlessingNames(t, sectest.NewECDSAPrincipalP256(t), sectest.NewED25519Principal(t))

}

func testBlessingNames(t *testing.T, p1, p2 security.Principal) {
	expiryCaveat, err := security.NewExpiryCaveat(time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	methodCaveat, err := security.NewMethodCaveat("FriendMethod")
	if err != nil {
		t.Fatal(err)
	}
	noCaveat := security.UnconstrainedUse()

	var (
		alice  = sectest.BlessSelf(t, p1, "alice", expiryCaveat)
		bob    = sectest.BlessSelf(t, p2, "bob", noCaveat)
		notBob = sectest.BlessSelf(t, p2, "bobUnrecognized", noCaveat)
	)
	sectest.AddToRoots(t, p2, bob)
	sectest.AddToRoots(t, p2, alice)
	alicefriend, err := p1.Bless(p2.PublicKey(), alice, "friend", methodCaveat)
	if err != nil {
		t.Fatal(err)
	}
	bobfriend, err := p2.Bless(p2.PublicKey(), bob, "friend", methodCaveat)
	if err != nil {
		t.Fatal(err)
	}
	aliceAndBobFriend, err := security.UnionOfBlessings(alicefriend, bobfriend)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cf := context.RootContext()
	defer cf()
	// BlessingNames, LocalBlessingNames are evaluated with p2 as the LocalPrincipal.
	tests := []struct {
		blessings security.Blessings
		names     []string
	}{
		{
			blessings: bob,
			names:     []string{"bob"},
		},
		{
			blessings: alicefriend,
			names:     []string{"alice:friend"},
		},
		{
			blessings: bobfriend,
			names:     []string{"bob:friend"},
		},
		{
			blessings: aliceAndBobFriend,
			names:     []string{"alice:friend", "bob:friend"},
		},
		{
			blessings: alice, // not a blessing for principal p2.
		},
		{
			blessings: notBob, // root not trusted.
		},
	}
	for _, test := range tests {
		want := test.names
		sort.Strings(want)

		got := security.BlessingNames(p2, test.blessings)
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("BlessingNames(%v) got:%v, want:%v", test.blessings, got, want)
		}

		call := security.NewCall(&security.CallParams{LocalPrincipal: p2, LocalBlessings: test.blessings})
		got = security.LocalBlessingNames(ctx, call)
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LocalBlessingNames(%v) got:%v, want:%v", test.blessings, got, want)
		}
	}
}

func TestBlessingsECDSA(t *testing.T) {
	testBlessings(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t).PublicKey(),
	)
}

func TestBlessingsED25519(t *testing.T) {
	testBlessings(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t).PublicKey(),
	)
}

func TestBlessings(t *testing.T) {
	testBlessings(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t).PublicKey(),
	)
	testBlessings(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t).PublicKey(),
	)
}

func testBlessings(t *testing.T, tp, p security.Principal, p2 security.PublicKey) {
	type s []string

	var (
		alice = sectest.BlessSelf(t, p, "alice")
		valid = s{
			"a",
			"john.doe",
			"bruce@wayne.com",
			"bugs..bunny",
			"trusted:friends",
			"friends:colleagues:work",
		}
		invalid = s{
			"",
			"...",
			":",
			"bugs...bunny",
			":bruce",
			"bruce:",
			"trusted::friends",
		}
	)
	sectest.AddToRoots(t, tp, alice)
	for _, test := range valid {
		self, err := p.BlessSelf(test)
		if err != nil {
			t.Errorf("BlessSelf(%q) failed: %v", test, err)
			continue
		}
		sectest.AddToRoots(t, tp, self)
		if err := checkBlessings(self, security.CallParams{LocalPrincipal: tp}, test); err != nil {
			t.Errorf("BlessSelf(%q): %v)", test, err)
		}
		other, err := p.Bless(p2, alice, test, security.UnconstrainedUse())
		if err != nil {
			t.Errorf("Bless(%q) failed: %v", test, err)
			continue
		}
		if err := checkBlessings(other, security.CallParams{LocalPrincipal: tp}, fmt.Sprintf("alice%v%v", security.ChainSeparator, test)); err != nil {
			t.Errorf("Bless(%q): %v", test, err)
		}
	}

	for _, test := range invalid {
		self, err := p.BlessSelf(test)
		if merr := matchesError(err, "invalid blessing extension"); merr != nil {
			t.Errorf("BlessSelf(%q): %v", test, merr)
		} else if !self.IsZero() {
			t.Errorf("BlessSelf(%q) returned %q", test, self)
		}
		other, err := p.Bless(p2, alice, test, security.UnconstrainedUse())
		if merr := matchesError(err, "invalid blessing extension"); merr != nil {
			t.Errorf("Bless(%q): %v", test, merr)
		} else if !other.IsZero() {
			t.Errorf("Bless(%q) returned %q", test, other)
		}
	}
}
func TestCreatePrincipalWithNilStoreAndRootsECDSA(t *testing.T) {
	p, err := security.CreatePrincipal(sectest.NewECDSASignerP256(t), nil, nil)
	if err != nil {
		t.Fatalf("CreatePrincipal using ECDSA signer failed: %v", err)
	}
	testCreatePrincipalWithNilStoreAndRoots(t, p)
}

func TestCreatePrincipalWithNilStoreAndRootsED25519(t *testing.T) {
	p, err := security.CreatePrincipal(sectest.NewED25519Signer(t), nil, nil)
	if err != nil {
		t.Fatalf("CreatePrincipal using ECDSA signer failed: %v", err)
	}
	testCreatePrincipalWithNilStoreAndRoots(t, p)
}

func testCreatePrincipalWithNilStoreAndRoots(t *testing.T, p security.Principal) {
	const (
		noRootsErr = "BlessingRoots object is nil"
		noStoreErr = "BlessingStore object is nil"
	)

	// Test Roots.
	r := p.Roots()
	if r == nil {
		t.Fatal("Roots() returned nil")
	}
	if err := matchesError(r.Add(nil, ""), noRootsErr); err != nil {
		t.Error(err)
	}
	if err := matchesError(r.Recognized(nil, ""), noRootsErr); err != nil {
		t.Error(err)
	}

	// Test Store.
	s := p.BlessingStore()
	var empty security.Blessings
	if s == nil {
		t.Fatal("BlessingStore() returned nil")
	}
	if _, err := s.Set(empty, ""); matchesError(err, noStoreErr) != nil {
		t.Error(matchesError(err, noStoreErr))
	}
	if err := matchesError(s.SetDefault(empty), noStoreErr); err != nil {
		t.Error(err)
	}
	if got := s.ForPeer(); !got.IsZero() {
		t.Errorf("BlessingStore.ForPeer: got %v want empty", got)
	}
	if got, _ := s.Default(); !got.IsZero() {
		t.Errorf("BlessingStore.Default: got %v want empty", got)
	}
	if got, want := s.PublicKey(), p.PublicKey(); !reflect.DeepEqual(got, want) {
		t.Errorf("BlessingStore.PublicKey: got %v want %v", got, want)
	}

	// Test that no blessings are trusted by the principal.
	if err := checkBlessings(sectest.BlessSelf(t, p, "alice"), security.CallParams{LocalPrincipal: p}); err != nil {
		t.Error(err)
	}
}

func TestAddToRootsECDSA(t *testing.T) {
	testAddToRoots(t,
		sectest.NewECDSAPrincipalP256,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t).PublicKey(),
	)
}
func TestAddToRootsED25519(t *testing.T) {
	testAddToRoots(t,
		sectest.NewED25519Principal,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t).PublicKey(),
	)
}

func TestAddToRoots(t *testing.T) {
	testAddToRoots(t,
		sectest.NewECDSAPrincipalP256,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t).PublicKey(),
	)
	testAddToRoots(t,
		sectest.NewED25519Principal,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t).PublicKey(),
	)
}

func testAddToRoots(t *testing.T, tpFunc func(t testing.TB) security.Principal, p1, p2 security.Principal, p3 security.PublicKey) {
	type s []string
	var (
		aliceFriend = sectest.BlessSelf(t, p1, "alice:friend")
		charlie     = sectest.BlessSelf(t, p2, "charlie")
	)
	aliceFriendSpouse, err := p1.Bless(p3, aliceFriend, "spouse", security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	charlieFamilyDaughter, err := p2.Bless(p3, charlie, "family:daughter", security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		add           security.Blessings
		root          security.PublicKey
		recognized    []string
		notRecognized []string
	}{
		{
			add:           aliceFriendSpouse,
			root:          p1.PublicKey(),
			recognized:    s{"alice:friend", "alice:friend:device", "alice:friend:device:app", "alice:friend:spouse", "alice:friend:spouse:friend"},
			notRecognized: s{"alice:device", "bob", "bob:friend", "bob:friend:spouse"},
		},
		{
			add:           charlieFamilyDaughter,
			root:          p2.PublicKey(),
			recognized:    s{"charlie", "charlie:friend", "charlie:friend:device", "charlie:family", "charlie:family:daughter", "charlie:family:friend", "charlie:family:friend:device"},
			notRecognized: s{"alice", "bob", "alice:family", "alice:family:daughter"},
		},
	}
	for _, test := range tests {
		tp := tpFunc(t) // principal where roots are tested.
		if err := security.AddToRoots(tp, test.add); err != nil {
			t.Error(err)
			continue
		}
		testroot, err := test.root.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range test.recognized {
			if tp.Roots().Recognized(testroot, b) != nil {
				t.Errorf("added roots for: %v but did not recognize blessing: %v", test.add, b)
			}
		}
		for _, b := range test.notRecognized {
			if tp.Roots().Recognized(testroot, b) == nil {
				t.Errorf("added roots for: %v but recognized blessing: %v", test.add, b)
			}
		}
	}
}

func TestPrincipalSignECDSA(t *testing.T) {
	testPrincipalSign(t, sectest.NewECDSAPrincipalP256(t))
}

func TestPrincipalSignED25519(t *testing.T) {
	testPrincipalSign(t, sectest.NewED25519Principal(t))
}

func testPrincipalSign(t *testing.T, p security.Principal) {
	var message = make([]byte, 10)
	if sig, err := p.Sign(message); err != nil {
		t.Error(err)
	} else if !sig.Verify(p.PublicKey(), message) {
		t.Errorf("Signature is not valid for message that was signed")
	}
}

func TestPrincipalSignaturePurposeECDSA(t *testing.T) {
	testPrincipalSignaturePurpose(t, sectest.NewECDSAPrincipalP256, sectest.NewECDSAPrincipalP256(t))
}

func TestPrincipalSignaturePurposeED25519(t *testing.T) {
	testPrincipalSignaturePurpose(t, sectest.NewED25519Principal, sectest.NewED25519Principal(t))
}

func testPrincipalSignaturePurpose(t *testing.T, pfn func(testing.TB) security.Principal, p security.Principal) {
	// Ensure that logically different private key operations result in different purposes in the signatures.

	// signPurpose for Sign
	signPurpose := []byte(security.SignatureForMessageSigning)
	if sig, err := p.Sign(make([]byte, 1)); err != nil {
		t.Error(err)
	} else if !bytes.Equal(sig.Purpose, signPurpose) {
		t.Errorf("Sign returned signature with purpose %q, want %q", sig.Purpose, signPurpose)
	}

	// blessPurpose for Bless (and BlessSelf)
	blessPurpose := []byte(security.SignatureForBlessingCertificates)
	selfBlessing, err := p.BlessSelf("foo")
	if err != nil {
		t.Fatal(err)
	}
	if sig := security.ExposeCertChains(selfBlessing)[0][0].Signature; !bytes.Equal(sig.Purpose, blessPurpose) {
		t.Errorf("BlessSelf used signature with purpose %q, want %q", sig.Purpose, blessPurpose)
	}
	otherBlessing, err := p.Bless(pfn(t).PublicKey(), selfBlessing, "bar", security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // Should be precisely 2 certificates in "otherBlessing"
		cert := security.ExposeCertChains(otherBlessing)[0][i]
		if !bytes.Equal(cert.Signature.Purpose, blessPurpose) {
			t.Errorf("Certificate with purpose %q, want %q", cert.Signature.Purpose, blessPurpose)
		}
	}
}

func TestUnionOfBlessingsECDSA(t *testing.T) {
	testUnionOfBlessings(t,
		sectest.NewECDSAPrincipalP256,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}
func TestUnionOfBlessingsED25519(t *testing.T) {
	testUnionOfBlessings(t,
		sectest.NewED25519Principal,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func TestUnionOfBlessings(t *testing.T) {
	testUnionOfBlessings(t,
		sectest.NewECDSAPrincipalP256,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
	testUnionOfBlessings(t,
		sectest.NewED25519Principal,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func testUnionOfBlessings(t *testing.T, pfn func(testing.TB) security.Principal, p1, p2, p security.Principal) {
	// A bunch of principals bless p
	var (
		alice = sectest.BlessSelf(t, p1, "alice")
		bob   = sectest.BlessSelf(t, p2, "bob")
		carol = sectest.BlessSelf(t, p, "carol")
		empty security.Blessings

		// call returns CallParams where the LocalPrincipal recognizes
		// all the blessings presented in 'recognized'.
		call = func(method, suffix string, recognized ...security.Blessings) security.CallParams {
			params := security.CallParams{
				Method:         method,
				Suffix:         suffix,
				LocalPrincipal: pfn(t),
			}
			for _, r := range recognized {
				sectest.AddToRoots(t, params.LocalPrincipal, r)
			}
			return params
		}
	)
	alicefriend, err := p1.Bless(p.PublicKey(), alice, "friend", sectest.NewMethodCaveat(t, "Method", "AliceMethod"))
	if err != nil {
		t.Fatal(err)
	}

	bobfriend, err := p2.Bless(p.PublicKey(), bob, "friend", sectest.NewMethodCaveat(t, "Method", "BobMethod"))
	if err != nil {
		t.Fatal(err)
	}
	friend, err := security.UnionOfBlessings(empty, alicefriend, empty, bobfriend, empty, carol, empty)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		params security.CallParams
		want   []string
	}{
		// The authorizing principal does not recognize either alice or bob
		// and thus does not recognize "friend".
		{call("Method", "Suffix"), nil},
		// Caveats not satisfied.
		{call("OtherMethod", "Suffix", alice, bob), nil},
		// No caveats on the recognized "carol" blessing.
		{call("Method", "Suffix", carol), []string{"carol"}},
		// Caveats on the recognized blessing are satisfied.
		{call("Method", "Suffix", alice), []string{"alice:friend"}},

		{call("Method", "Suffix", alice, carol), []string{"carol", "alice:friend"}},
		{call("Method", "Suffix", bob), []string{"bob:friend"}},

		{call("Method", "Suffix", bob, carol), []string{"carol", "bob:friend"}},
		{call("Method", "Suffix", alice, bob, carol), []string{"carol", "alice:friend", "bob:friend"}},
		// Caveats on only one of the two recognized blessings is satisfied.
		{call("AliceMethod", "Suffix", alice, bob), []string{"alice:friend"}},
		// Caveats on only one of the two recognized blessings is satisfied.
		{call("BobMethod", "Suffix", alice, bob), []string{"bob:friend"}},
	} {
		if err := checkBlessings(friend, tc.params, tc.want...); err != nil {
			t.Errorf("failed for call params: %v (want: %v): %v\n", tc.params, tc.want, err)
		}
	}

	// p can bless p3 further, allowing only method calls on 'Suffix'.
	spouse, err := p.Bless(pfn(t).PublicKey(), friend, "spouse", sectest.NewSuffixCaveat(t, "Suffix"))
	if err != nil {
		t.Fatal(err)
	}
	if err := checkBlessings(spouse, call("Method", "Suffix", alice, bob, carol), "carol:spouse", "alice:friend:spouse", "bob:friend:spouse"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(spouse, call("Method", "OtherSuffix", alice, bob, carol)); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(spouse, call("AliceMethod", "Suffix", alice, bob, carol), "carol:spouse", "alice:friend:spouse"); err != nil {
		t.Error(err)
	}

	// However, UnionOfBlessings must not mix up public keys
	if mixed, err := security.UnionOfBlessings(alice, bob); err == nil || !mixed.IsZero() {
		t.Errorf("Got (%v, %v(errorid=%v)), want errInvalidUnion", mixed, err, verror.ErrorID(err))
	}
}

func TestCertificateCompositionAttackECDSA(t *testing.T) {
	testCertificateCompositionAttack(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}
func TestCertificateCompositionAttackED25519(t *testing.T) {
	testCertificateCompositionAttack(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}
func TestCertificateCompositionAttack(t *testing.T) {
	testCertificateCompositionAttack(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
	testCertificateCompositionAttack(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func testCertificateCompositionAttack(t *testing.T, tp, p1, p2, p3, p4 security.Principal) {
	// tp is the principal for testing blessings.
	var (
		alice = sectest.BlessSelf(t, p1, "alice")
		bob   = sectest.BlessSelf(t, p2, "bob")
		cp    = security.CallParams{Method: "Foo", LocalPrincipal: tp}
	)
	sectest.AddToRoots(t, tp, alice)
	sectest.AddToRoots(t, tp, bob)
	// p3 has the blessings "alice:friend" and "bob:family" (from p1 and p2 respectively).
	// It then blesses p4 as "alice:friend:spouse" with no caveat and as "bob:family:spouse"
	// with a caveat.
	alicefriend, err := p1.Bless(p3.PublicKey(), alice, "friend", security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	bobfamily, err := p2.Bless(p3.PublicKey(), bob, "family", security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}

	alicefriendspouse, err := p3.Bless(p4.PublicKey(), alicefriend, "spouse", security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	bobfamilyspouse, err := p3.Bless(p4.PublicKey(), bobfamily, "spouse", sectest.NewMethodCaveat(t, "Foo"))
	if err != nil {
		t.Fatal(err)
	}
	// p4's blessings should be valid.
	if err := checkBlessings(alicefriendspouse, cp, "alice:friend:spouse"); err != nil {
		t.Fatal(err)
	}
	if err := checkBlessings(bobfamilyspouse, cp, "bob:family:spouse"); err != nil {
		t.Fatal(err)
	}

	// p4 should be not to construct a valid "bob:family:spouse" blessing by
	// using the "spouse" certificate from "alice:friend:spouse" (that has no caveats)
	// and replacing the "spouse" certificate from "bob:family:spouse".
	spousecert := security.ExposeCertChains(alicefriendspouse)[0][2]
	// sanity check
	if spousecert.Extension != "spouse" || len(spousecert.Caveats) != 1 || spousecert.Caveats[0].Id != security.ConstCaveat.Id {
		t.Fatalf("Invalid test data. Certificate: %+v", spousecert)
	}
	// Replace the certificate in bobfamilyspouse
	security.ExposeCertChains(bobfamilyspouse)[0][2] = spousecert
	if err := matchesError(checkBlessings(bobfamilyspouse, cp), "invalid Signature in certificate(for \"spouse\")"); err != nil {
		t.Fatal(err)
	}
}
func TestCertificateTamperingAttackECDSA(t *testing.T) {
	testCertificateTamperingAttack(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)

}
func TestCertificateTamperingAttackED25519(t *testing.T) {
	testCertificateTamperingAttack(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}
func TestCertificateTamperingAttack(t *testing.T) {
	testCertificateTamperingAttack(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
	testCertificateTamperingAttack(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func testCertificateTamperingAttack(t *testing.T, tp, p1, p2, p3 security.Principal) {
	// tp is principal for testing blessings.
	alice := sectest.BlessSelf(t, p1, "alice")
	sectest.AddToRoots(t, tp, alice)

	alicefriend, err := p1.Bless(p2.PublicKey(), alice, "friend", security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	if err := checkBlessings(alicefriend, security.CallParams{LocalPrincipal: tp}, "alice:friend"); err != nil {
		t.Fatal(err)
	}
	// p3 attempts to "steal" the blessing by constructing his own certificate.
	cert := &security.ExposeCertChains(alicefriend)[0][1]
	if cert.PublicKey, err = p3.PublicKey().MarshalBinary(); err != nil {
		t.Fatal(err)
	}
	if err := matchesError(checkBlessings(alicefriend, security.CallParams{LocalPrincipal: tp}, "alice:friend"), "invalid Signature in certificate(for \"friend\")"); err != nil {
		t.Error(err)
	}
}

func TestCertificateChainsTamperingAttackECDSA(t *testing.T) {
	testCertificateChainsTamperingAttack(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func TestCertificateChainsTamperingAttackED25519(t *testing.T) {
	testCertificateChainsTamperingAttack(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func TestCertificateChainsTamperingAttack(t *testing.T) {
	testCertificateChainsTamperingAttack(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
	testCertificateChainsTamperingAttack(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func testCertificateChainsTamperingAttack(t *testing.T, tp, p1, p2 security.Principal) {
	// tp is principal for testing blessings.
	var (
		// tp is principal for testing blessings.
		alice = sectest.BlessSelf(t, p1, "alice")
		bob   = sectest.BlessSelf(t, p2, "bob")
	)
	sectest.AddToRoots(t, tp, alice)
	sectest.AddToRoots(t, tp, bob)

	if err := checkBlessings(alice, security.CallParams{LocalPrincipal: tp}, "alice"); err != nil {
		t.Fatal(err)
	}
	// Act as if alice tried to package bob's chain with her existing chains and ship it over the network.
	chains := append(security.ExposeCertChains(alice), security.ExposeCertChains(bob)...)
	security.ExposeAppendCertChains(&alice, chains)
	if err := matchesError(checkBlessings(alice, security.CallParams{LocalPrincipal: tp}, "alice", "bob"), "two certificate chains that bind to different public keys"); err != nil {
		t.Error(err)
	}
}
func TestBlessingToAndFromWireECDSA(t *testing.T) {
	testBlessingToAndFromWire(t, sectest.NewECDSAPrincipalP256(t))

}

func TestBlessingToAndFromWireED25519(t *testing.T) {
	testBlessingToAndFromWire(t, sectest.NewED25519Principal(t))
}

func testBlessingToAndFromWire(t *testing.T, p security.Principal) {
	// WireBlessings and Blessings should be basically interchangeable.
	native, err := p.BlessSelf("self")
	if err != nil {
		t.Fatal(err)
	}
	var wire security.WireBlessings
	var dup security.Blessings
	// native -> wire
	if err := sectest.RoundTrip(native, &wire); err != nil {
		t.Fatal(err)
	}
	// wire -> native
	if err := sectest.RoundTrip(wire, &dup); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dup, native) {
		t.Errorf("native->wire->native changed value from %#v to %#v", native, dup)
	}
}
func TestBlessingsRoundTripECDSA(t *testing.T) {
	testBlessingsRoundTrip(t, sectest.NewECDSAPrincipalP256(t), sectest.NewECDSAPrincipalP256(t))
}
func TestBlessingsRoundTripED25519(t *testing.T) {
	testBlessingsRoundTrip(t, sectest.NewED25519Principal(t), sectest.NewED25519Principal(t))
}

func TestBlessingsRoundTrip(t *testing.T) {
	testBlessingsRoundTrip(t, sectest.NewECDSAPrincipalP256(t), sectest.NewED25519Principal(t))
	testBlessingsRoundTrip(t, sectest.NewED25519Principal(t), sectest.NewECDSAPrincipalP256(t))
}

func testBlessingsRoundTrip(t *testing.T, p1, p2 security.Principal) {
	// Test that the blessing obtained after roundtripping is identical to the original.
	b, err := p1.BlessSelf("self")
	if err != nil {
		t.Fatal(err)
	}
	var got security.Blessings
	if err := sectest.RoundTrip(b, &got); err != nil || !reflect.DeepEqual(got, b) {
		t.Fatalf("Got (%#v, %v), want (%#v, nil)", got, err, b)
	}
	// Putzing around with the wire representation should break the decoding.
	otherkey, err := p2.PublicKey().MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var wire security.WireBlessings
	if err := sectest.RoundTrip(b, &wire); err != nil {
		t.Fatal(err)
	}
	wire.CertificateChains[0][len(wire.CertificateChains[0])-1].PublicKey = otherkey
	err = sectest.RoundTrip(wire, &got)
	if merr := matchesError(err, "invalid Signature in certificate"); merr != nil {
		t.Error(merr)
	}
	// It should be fine to send/recv empty blessings
	got = security.Blessings{}
	if err := sectest.RoundTrip(security.Blessings{}, &got); err != nil || !got.IsZero() {
		t.Errorf("Got (%#v, %v) want (<zero value>, nil)", got, err)
	}
}

func TestBlessingsOnWireWithMissingCertificatesECDSA(t *testing.T) {
	testBlessingsOnWireWithMissingCertificates(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}
func TestBlessingsOnWireWithMissingCertificatesED25519(t *testing.T) {
	testBlessingsOnWireWithMissingCertificates(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t))
}

func TestBlessingsOnWireWithMissingCertificates(t *testing.T) {
	testBlessingsOnWireWithMissingCertificates(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
	testBlessingsOnWireWithMissingCertificates(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t))
}

func testBlessingsOnWireWithMissingCertificates(t *testing.T, rootP, middlemanP, leafP security.Principal) {
	var (
		B = func(b security.Blessings, err error) security.Blessings {
			if err != nil {
				t.Fatal(err)
			}
			return b
		}

		// Create "leaf", a blessing involving three certificates that bind the name
		// root:middleman:leaf to the leaf principal.
		root      = B(rootP.BlessSelf("root"))
		middleman = B(rootP.Bless(middlemanP.PublicKey(), root, "middleman", security.UnconstrainedUse()))
		leaf      = B(middlemanP.Bless(leafP.PublicKey(), middleman, "leaf", security.UnconstrainedUse()))

		wire security.WireBlessings
		tmp  security.Blessings
		err  error
	)
	if err := sectest.RoundTrip(leaf, &wire); err != nil {
		t.Fatal(err)
	}
	// We should have a certificate chain of size 3.
	chain := wire.CertificateChains[0]
	if len(chain) != 3 {
		t.Fatalf("Got a chain of %d certificates, want 3", len(chain))
	}

	C1, C2, C3 := chain[0], chain[1], chain[2]
	var CX security.Certificate
	// The following combinations should fail because a certificate is missing
	type C []security.Certificate
	tests := []struct {
		Chain []security.Certificate
		Err   string
	}{
		{C{}, "empty certificate chain"}, // Empty chain
		{C{C1, C3}, "invalid Signature"}, // Missing link in the chain
		{C{C2, C3}, "invalid Signature"},
		{C{CX, C2, C3}, "invalid Signature"},
		{C{C1, CX, C3}, "asn"},
		{C{C1, C2, CX}, "asn"},
		{C{C1, C2, C3}, ""}, // Valid chain
	}
	for idx, test := range tests {
		wire.CertificateChains[0] = test.Chain
		err := sectest.RoundTrip(wire, &tmp)
		if merr := matchesError(err, test.Err); merr != nil {
			t.Errorf("(%d) %v [%v]", idx, merr, test.Chain)
		}
	}

	// Mulitiple chains, certifying different keys should fail
	wire.CertificateChains = [][]security.Certificate{
		C{C1},
		C{C1, C2},
		C{C1, C2, C3},
	}
	err = sectest.RoundTrip(wire, &tmp)
	if merr := matchesError(err, "bind to different public keys"); merr != nil {
		t.Error(err)
	}

	// Multiple chains certifying the same key are okay
	wire.CertificateChains = [][]security.Certificate{chain, chain, chain}
	if err := sectest.RoundTrip(wire, &tmp); err != nil {
		t.Error(err)
	}
	// But leaving any empty chains is not okay
	for idx := 0; idx < len(wire.CertificateChains); idx++ {
		wire.CertificateChains[idx] = []security.Certificate{}
		err := sectest.RoundTrip(wire, &tmp)
		if merr := matchesError(err, "empty certificate chain"); merr != nil {
			t.Errorf("%d: %v", idx, merr)
		}
		wire.CertificateChains[idx] = chain
	}
}
func TestOverrideCaveatValidationECDSA(t *testing.T) {
	testOverrideCaveatValidation(t, sectest.NewECDSAPrincipalP256(t))
}

func TestOverrideCaveatValidationED25519(t *testing.T) {
	testOverrideCaveatValidation(t, sectest.NewED25519Principal(t))
}

func testOverrideCaveatValidation(t *testing.T, p security.Principal) {
	falseReturningCav := security.Caveat{
		Id: uniqueid.Id{0x99, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}
	trueReturningCav := security.Caveat{
		Id: uniqueid.Id{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	}
	falseResultErr := fmt.Errorf("False caveat result")

	validator := func(ctx *context.T, call security.Call, cavs [][]security.Caveat) []error {
		return overideCaveatValidator(t, ctx, call, trueReturningCav.Id, falseReturningCav.Id, falseResultErr, cavs)
	}
	ofn := security.SetCaveatValidationForTest(validator)
	defer security.SetCaveatValidationForTest(ofn)

	ctx, cf := context.RootContext()
	defer cf()

	bu, err := p.BlessSelf("unrestricted", security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	bft, err := p.BlessSelf("falsetrue", falseReturningCav, trueReturningCav)
	if err != nil {
		t.Fatal(err)
	}
	btt, err := p.BlessSelf("truetrue", trueReturningCav, trueReturningCav)
	if err != nil {
		t.Fatal(err)
	}
	bt, err := p.BlessSelf("true", trueReturningCav)
	if err != nil {
		t.Fatal(err)
	}
	bsepft, err := p.Bless(p.PublicKey(), bt, "false", falseReturningCav)
	if err != nil {
		t.Fatal(err)
	}
	bseptt, err := p.Bless(p.PublicKey(), bt, "true", trueReturningCav)
	if err != nil {
		t.Fatal(err)
	}
	bunion, err := security.UnionOfBlessings(bu, bft, btt, bsepft, bseptt)
	if err != nil {
		t.Fatal(err)
	}
	if err := security.AddToRoots(p, bunion); err != nil {
		t.Fatal(err)
	}

	results, infos := security.RemoteBlessingNames(ctx, security.NewCall(&security.CallParams{
		LocalPrincipal:  p,
		RemoteBlessings: bunion,
	}))
	expectedFailInfos := map[string]string{
		"falsetrue":  falseResultErr.Error(),
		"true:false": falseResultErr.Error(),
	}
	failInfos := map[string]string{}
	for _, info := range infos {
		failInfos[info.Blessing] = info.Err.Error()
	}
	if !reflect.DeepEqual(failInfos, expectedFailInfos) {
		t.Fatalf("Unexpected failinfos from RemoteBlessingNames. Got %v, want %v", failInfos, expectedFailInfos)
	}
	expectedResults := []string{"unrestricted", "truetrue", "true:true"}
	sort.Strings(results)
	sort.Strings(expectedResults)
	if !reflect.DeepEqual(results, expectedResults) {
		t.Fatalf("Unexpected results from RemoteBlessingNames. Got %v, want %v", results, expectedResults)
	}
}

func overideCaveatValidator(t *testing.T, _ *context.T, _ security.Call, trueID, falseID uniqueid.Id, falseResultErr error, cavs [][]security.Caveat) []error {
	results := make([]error, len(cavs))
	for i, chain := range cavs {
		for _, cav := range chain {
			switch cav.Id {
			case falseID:
				results[i] = falseResultErr
			case trueID:
			case security.ConstCaveat.Id:
				if !reflect.DeepEqual(cav, security.UnconstrainedUse()) {
					t.Fatalf("Unexpected const caveat")
				}
			default:
				t.Fatalf("Unexpected caveat: %#v", cav)
			}
		}
	}
	return results
}

func TestRemoteBlessingNamesECDSA(t *testing.T) {
	testRemoteBlessingNames(t, sectest.NewECDSAPrincipalP256(t))
}

func TestRemoteBlessingNamesED25519(t *testing.T) {
	testRemoteBlessingNames(t, sectest.NewED25519Principal(t))
}

func testRemoteBlessingNames(t *testing.T, p security.Principal) {
	var (
		mkBlessing = func(name string, cav ...security.Caveat) security.Blessings {
			ret, err := p.BlessSelf(name, cav...)
			if err != nil {
				t.Fatalf("%q: %v", name, err)
			}
			return ret
		}

		b1 = mkBlessing("alice", sectest.NewMethodCaveat(t, "Method"))
		b2 = mkBlessing("bob")

		bnames = func(b security.Blessings, method string) ([]string, []security.RejectedBlessing) {
			ctx, cancel := context.RootContext()
			defer cancel()
			return security.RemoteBlessingNames(ctx, security.NewCall(&security.CallParams{
				LocalPrincipal:  p,
				RemoteBlessings: b,
				Method:          method}))
		}
	)
	if err := security.AddToRoots(p, b1); err != nil {
		t.Fatal(err)
	}

	// b1's alice is recognized in the right context.
	if accepted, rejected := bnames(b1, "Method"); !reflect.DeepEqual(accepted, []string{"alice"}) || len(rejected) > 0 {
		t.Errorf("Got (%v, %v), want ([alice], nil)", accepted, rejected)
	}
	// b1's alice is rejected when caveats are not met.
	if accepted, rejected := bnames(b1, "Blah"); len(accepted) > 0 ||
		len(rejected) != 1 ||
		rejected[0].Blessing != "alice" ||
		verror.ErrorID(rejected[0].Err) != security.ErrCaveatValidation.ID {
		t.Errorf("Got (%v, %v), want ([], [alice: <caveat validation error>])", accepted, rejected)
	}
	// b2 is not recognized because the roots aren't recognized.
	if accepted, rejected := bnames(b2, "Method"); len(accepted) > 0 ||
		len(rejected) != 1 ||
		rejected[0].Blessing != "bob" ||
		verror.ErrorID(rejected[0].Err) != security.ErrUnrecognizedRoot.ID {
		t.Errorf("Got (%v, %v), want ([], [bob: <untrusted root>])", accepted, rejected)
	}
}

func BenchmarkRemoteBlessingNamesECDSA(b *testing.B) {
	benchmarkRemoteBlessingNames(b,
		sectest.NewECDSAPrincipalP256(b),
		sectest.NewECDSAPrincipalP256(b))
}

func BenchmarkRemoteBlessingNamesED25519(b *testing.B) {
	benchmarkRemoteBlessingNames(b,
		sectest.NewED25519Principal(b),
		sectest.NewED25519Principal(b))
}

func benchmarkRemoteBlessingNames(b *testing.B, p, p2 security.Principal) {
	local, err := p.BlessSelf("local")
	if err != nil {
		b.Fatal(err)
	}
	remote, err := p.Bless(p2.PublicKey(), local, "delegate", security.UnconstrainedUse())
	if err != nil {
		b.Fatal(err)
	}
	if err := security.AddToRoots(p, remote); err != nil {
		b.Fatal(err)
	}
	ctx, cancel := context.RootContext()
	defer cancel()
	call := security.NewCall(&security.CallParams{
		LocalPrincipal:  p,
		RemoteBlessings: remote})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		security.RemoteBlessingNames(ctx, call)
	}
	b.StopTimer() // So the cancel() call isn't included.
}

func TestSigningBlessingsECDSA(t *testing.T) {
	testSigningBlessings(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func TestSigningBlessingsED25519(t *testing.T) {
	testSigningBlessings(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func TestSigningBlessings(t *testing.T) {
	testSigningBlessings(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
	testSigningBlessings(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
	)
}

func testSigningBlessings(t *testing.T, google, alice, bob security.Principal) {
	var (
		googleB, _ = google.BlessSelf("google")

		peerCav, _  = security.NewCaveat(security.PeerBlessingsCaveat, []security.BlessingPattern{"youtube"})
		trueCav, _  = security.NewCaveat(security.ConstCaveat, true)
		falseCav, _ = security.NewCaveat(security.ConstCaveat, false)

		aliceSelf, _          = alice.BlessSelf("alice")
		googleYoutubeUser, _  = google.Bless(alice.PublicKey(), googleB, "youtube:user", peerCav)
		googleAliceExpired, _ = google.Bless(alice.PublicKey(), googleB, "alice:expired", sectest.NewExpiryCaveat(t, time.Now().Add(-time.Second)))
		googleAliceFalse, _   = google.Bless(alice.PublicKey(), googleB, "alice:false", falseCav)
		googleAlice, _        = google.Bless(alice.PublicKey(), googleB, "alice", sectest.NewExpiryCaveat(t, time.Now().Add(time.Hour)), trueCav)
		aliceB, _             = security.UnionOfBlessings(aliceSelf, googleYoutubeUser, googleAliceExpired, googleAliceFalse, googleAlice)
	)
	if err := security.AddToRoots(bob, googleB); err != nil {
		t.Fatal(err)
	}

	// The blessing "google:youtube:user" should be dropped when calling
	// SigningBlessings on 'aliceB'
	aliceSigning := security.SigningBlessings(aliceB)

	if !reflect.DeepEqual(aliceSigning.PublicKey(), aliceB.PublicKey()) {
		t.Fatal("SigningBlessings returned blessings with different public key")
	}

	got, want := rawNames(aliceSigning), []string{"alice", "google:alice:expired", "google:alice:false", "google:alice"}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SigningBlessings(%v): got %v, want %v", aliceB, got, want)
	}

	// Alice sends the blessings 'aliceB' to Bob as part of signed data.
	// The signing names seen by bob must correspond to blessings
	// that have recognized roots and only valid signing caveats.
	ctx, cf := context.RootContext()
	defer cf()

	names, rejected := security.SigningBlessingNames(ctx, bob, aliceB)
	if want := []string{"google:alice"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("SigningBlessingNames(%v): got names %v, want %v", aliceB, names, want)
	}
	if got := len(rejected); got != 4 {
		t.Fatalf("SigningBlessingNames(%v): got %d rejected blessing names, want 4", aliceB, got)
	}
	for _, r := range rejected {
		if err := validateAliceRejectedBlessingError(r); err != nil {
			t.Errorf("SigningBlessingNames(%v): %v", aliceB, err)
		}
	}
}

func validateAliceRejectedBlessingError(r security.RejectedBlessing) error {
	switch r.Blessing {
	case "alice":
		if got, want := verror.ErrorID(r.Err), security.ErrUnrecognizedRoot.ID; got != want {
			return fmt.Errorf("rejected blessing %v with errorID %v, want errorID %v", r.Blessing, got, want)
		}
	case "google:alice:false":
		if got, want := verror.ErrorID(r.Err), security.ErrCaveatValidation.ID; got != want {
			return fmt.Errorf("rejected blessing %v with errorID %v, want errorID %v", r.Blessing, got, want)
		}
	case "google:alice:expired":
		if got, want := verror.ErrorID(r.Err), security.ErrCaveatValidation.ID; got != want {
			return fmt.Errorf("rejected blessing %v with errorID %v, want errorID %v", r.Blessing, got, want)
		}
	case "google:youtube:user":
		if got, want := verror.ErrorID(r.Err), security.ErrInvalidSigningBlessingCaveat.ID; got != want {
			return fmt.Errorf("rejected blessing %v with errorID %v, want errorID %v", r.Blessing, got, want)
		}
	default:
		return fmt.Errorf("invalid rejected blessing name %v", r.Blessing)
	}
	return nil
}

// rawNames returns the blessing names encapsulated in the provided blessings,
// without validating any caveats and blessing roots.
func rawNames(b security.Blessings) []string {
	var ret []string
	for _, chain := range security.ExposeCertChains(b) {
		if len(chain) == 0 {
			continue
		}
		ret = append(ret, security.ExposeClaimedName(chain))
	}
	return ret
}

func equalBlessings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func checkBlessings(b security.Blessings, c security.CallParams, want ...string) error {
	// Validate the integrity of the bits.
	var decoded security.Blessings
	if err := sectest.RoundTrip(b, &decoded); err != nil {
		return err
	}
	if !reflect.DeepEqual(decoded, b) {
		return fmt.Errorf("reflect.DeepEqual of %#v and %#v failed after roundtripping", decoded, b)
	}
	// And now check them under the right call
	c.RemoteBlessings = b
	ctx, cancel := context.RootContext()
	defer cancel()
	got, _ := security.RemoteBlessingNames(ctx, security.NewCall(&c))
	if !equalBlessings(got, want) {
		return fmt.Errorf("Got blessings %v, want %v", got, want)
	}
	return nil
}

func matchesError(got error, want string) error {
	if (got == nil) && len(want) == 0 {
		return nil
	}
	if got == nil {
		return fmt.Errorf("Got nil error, wanted to match %q", want)
	}
	if !strings.Contains(got.Error(), want) {
		return fmt.Errorf("Got error %q, wanted to match %q", got, want)
	}
	return nil
}
