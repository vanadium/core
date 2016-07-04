// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"crypto/elliptic"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/uniqueid"
	"v.io/v23/verror"
)

func TestBlessSelf(t *testing.T) {
	var (
		tp = newPrincipal(t) // principal where blessings are tested
		p  = newPrincipal(t)

		call = func(method string) CallParams {
			return CallParams{
				LocalPrincipal: tp,
				Method:         method,
			}
		}
	)

	alice, err := p.BlessSelf("alice", newCaveat(NewMethodCaveat("Method")))
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
	addToRoots(t, tp, alice)
	if err := checkBlessings(alice, call("Foo")); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(alice, call("Method"), "alice"); err != nil {
		t.Error(err)
	}
}

func TestBless(t *testing.T) {
	var (
		tp = newPrincipal(t) // principal where blessings are tested

		p1    = newPrincipal(t)
		p2    = newPrincipal(t)
		p3    = newPrincipal(t)
		alice = blessSelf(t, p1, "alice")
		call  = func(method, suffix string) CallParams {
			return CallParams{
				LocalPrincipal: tp,
				Method:         method,
				Suffix:         suffix,
			}
		}
	)
	addToRoots(t, tp, alice)
	// p1 blessing p2 'with' empty Blessings should fail.
	if b, err := p1.Bless(p2.PublicKey(), Blessings{}, "friend", UnconstrainedUse()); err == nil {
		t.Errorf("p1 was able to extend a nil blessing to produce: %v", b)
	}
	// p1 blessing p2 as "alice:friend" for "Suffix.Method"
	friend, err := p1.Bless(p2.PublicKey(), alice, "friend", newCaveat(NewMethodCaveat("Method")), newSuffixCaveat("Suffix"))
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
	blessings, err := p2.Bless(p3.PublicKey(), alice, "friend", UnconstrainedUse())
	if !blessings.IsZero() {
		t.Errorf("p2 was able to extend a blessing bound to p1 to produce: %v", blessings)
	} else if err = matchesError(err, "cannot extend blessing with public key"); err != nil {
		t.Fatal(err)
	}
}

func TestThirdPartyCaveats(t *testing.T) {
	var (
		p1  = newPrincipal(t)
		p2  = newPrincipal(t)
		tp1 = newCaveat(NewPublicKeyCaveat(p1.PublicKey(), "peoria", ThirdPartyRequirements{}, UnconstrainedUse()))
		tp2 = newCaveat(NewPublicKeyCaveat(p1.PublicKey(), "london", ThirdPartyRequirements{}, UnconstrainedUse()))
		tp3 = newCaveat(NewPublicKeyCaveat(p1.PublicKey(), "delhi", ThirdPartyRequirements{}, UnconstrainedUse()))
		c1  = newCaveat(NewMethodCaveat("method"))
		c2  = newCaveat(NewExpiryCaveat(time.Now()))
	)

	b, err := p1.BlessSelf("alice", tp1, c1, tp2)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := b.ThirdPartyCaveats(), []Caveat{tp1, tp2}; !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}
	if b, err = p1.Bless(p2.PublicKey(), b, "friend", tp3, c2); err != nil {
		t.Fatal(err)
	}
	if got, want := b.ThirdPartyCaveats(), []Caveat{tp1, tp2, tp3}; !reflect.DeepEqual(got, want) {
		t.Errorf("Got %v, want %v", got, want)
	}
}

func TestBlessingNames(t *testing.T) {
	expiryCaveat, err := NewExpiryCaveat(time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	methodCaveat, err := NewMethodCaveat("FriendMethod")
	if err != nil {
		t.Fatal(err)
	}
	noCaveat := UnconstrainedUse()

	var (
		p1     = newPrincipal(t)
		alice  = blessSelf(t, p1, "alice", expiryCaveat)
		p2     = newPrincipal(t)
		bob    = blessSelf(t, p2, "bob", noCaveat)
		notBob = blessSelf(t, p2, "bobUnrecognized", noCaveat)
	)
	addToRoots(t, p2, bob)
	addToRoots(t, p2, alice)
	alicefriend, err := p1.Bless(p2.PublicKey(), alice, "friend", methodCaveat)
	if err != nil {
		t.Fatal(err)
	}
	bobfriend, err := p2.Bless(p2.PublicKey(), bob, "friend", methodCaveat)
	if err != nil {
		t.Fatal(err)
	}
	aliceAndBobFriend, err := UnionOfBlessings(alicefriend, bobfriend)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cf := context.RootContext()
	defer cf()
	// BlessingNames, LocalBlessingNames are evaluated with p2 as the LocalPrincipal.
	tests := []struct {
		blessings Blessings
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

		got := BlessingNames(p2, test.blessings)
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("BlessingNames(%v) got:%v, want:%v", test.blessings, got, want)
		}

		call := NewCall(&CallParams{LocalPrincipal: p2, LocalBlessings: test.blessings})
		got = LocalBlessingNames(ctx, call)
		sort.Strings(got)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("LocalBlessingNames(%v) got:%v, want:%v", test.blessings, got, want)
		}
	}
}

func TestBlessings(t *testing.T) {
	type s []string

	var (
		tp = newPrincipal(t) // principal where blessings are tested

		p     = newPrincipal(t)
		p2    = newPrincipal(t).PublicKey()
		alice = blessSelf(t, p, "alice")
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
	addToRoots(t, tp, alice)
	for _, test := range valid {
		self, err := p.BlessSelf(test)
		if err != nil {
			t.Errorf("BlessSelf(%q) failed: %v", test, err)
			continue
		}
		addToRoots(t, tp, self)
		if err := checkBlessings(self, CallParams{LocalPrincipal: tp}, test); err != nil {
			t.Errorf("BlessSelf(%q): %v)", test, err)
		}
		other, err := p.Bless(p2, alice, test, UnconstrainedUse())
		if err != nil {
			t.Errorf("Bless(%q) failed: %v", test, err)
			continue
		}
		if err := checkBlessings(other, CallParams{LocalPrincipal: tp}, fmt.Sprintf("alice%v%v", ChainSeparator, test)); err != nil {
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
		other, err := p.Bless(p2, alice, test, UnconstrainedUse())
		if merr := matchesError(err, "invalid blessing extension"); merr != nil {
			t.Errorf("Bless(%q): %v", test, merr)
		} else if !other.IsZero() {
			t.Errorf("Bless(%q) returned %q", test, other)
		}
	}
}

func TestCreatePrincipalWithNilStoreAndRoots(t *testing.T) {
	p, err := CreatePrincipal(newECDSASigner(t, elliptic.P256()), nil, nil)
	if err != nil {
		t.Fatalf("CreatePrincipal failed: %v", err)
	}
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
	var empty Blessings
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
	if err := checkBlessings(blessSelf(t, p, "alice"), CallParams{LocalPrincipal: p}); err != nil {
		t.Error(err)
	}
}

func TestAddToRoots(t *testing.T) {
	type s []string
	var (
		p1          = newPrincipal(t)
		aliceFriend = blessSelf(t, p1, "alice:friend")

		p2      = newPrincipal(t)
		charlie = blessSelf(t, p2, "charlie")

		p3 = newPrincipal(t).PublicKey()
	)
	aliceFriendSpouse, err := p1.Bless(p3, aliceFriend, "spouse", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	charlieFamilyDaughter, err := p2.Bless(p3, charlie, "family:daughter", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		add           Blessings
		root          PublicKey
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
		tp := newPrincipal(t) // principal where roots are tested.
		if err := AddToRoots(tp, test.add); err != nil {
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

func TestPrincipalSign(t *testing.T) {
	var (
		p       = newPrincipal(t)
		message = make([]byte, 10)
	)
	if sig, err := p.Sign(message); err != nil {
		t.Error(err)
	} else if !sig.Verify(p.PublicKey(), message) {
		t.Errorf("Signature is not valid for message that was signed")
	}
}

func TestPrincipalSignaturePurpose(t *testing.T) {
	// Ensure that logically different private key operations result in different purposes in the signatures.
	p := newPrincipal(t)

	// signPurpose for Sign
	if sig, err := p.Sign(make([]byte, 1)); err != nil {
		t.Error(err)
	} else if !bytes.Equal(sig.Purpose, signPurpose) {
		t.Errorf("Sign returned signature with purpose %q, want %q", sig.Purpose, signPurpose)
	}

	// blessPurpose for Bless (and BlessSelf)
	selfBlessing, err := p.BlessSelf("foo")
	if err != nil {
		t.Fatal(err)
	}
	if sig := selfBlessing.chains[0][0].Signature; !bytes.Equal(sig.Purpose, blessPurpose) {
		t.Errorf("BlessSelf used signature with purpose %q, want %q", sig.Purpose, blessPurpose)
	}
	otherBlessing, err := p.Bless(newPrincipal(t).PublicKey(), selfBlessing, "bar", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ { // Should be precisely 2 certificates in "otherBlessing"
		cert := otherBlessing.chains[0][i]
		if !bytes.Equal(cert.Signature.Purpose, blessPurpose) {
			t.Errorf("Certificate with purpose %q, want %q", cert.Signature.Purpose, blessPurpose)
		}
	}
}

func TestUnionOfBlessings(t *testing.T) {
	// A bunch of principals bless p
	var (
		p1    = newPrincipal(t)
		p2    = newPrincipal(t)
		alice = blessSelf(t, p1, "alice")
		bob   = blessSelf(t, p2, "bob")
		p     = newPrincipal(t)
		carol = blessSelf(t, p, "carol")
		empty Blessings

		// call returns CallParams where the LocalPrincipal recognizes
		// all the blessings presented in 'recognized'.
		call = func(method, suffix string, recognized ...Blessings) CallParams {
			params := CallParams{
				Method:         method,
				Suffix:         suffix,
				LocalPrincipal: newPrincipal(t),
			}
			for _, r := range recognized {
				addToRoots(t, params.LocalPrincipal, r)
			}
			return params
		}
	)
	alicefriend, err := p1.Bless(p.PublicKey(), alice, "friend", newCaveat(NewMethodCaveat("Method", "AliceMethod")))
	if err != nil {
		t.Fatal(err)
	}

	bobfriend, err := p2.Bless(p.PublicKey(), bob, "friend", newCaveat(NewMethodCaveat("Method", "BobMethod")))
	if err != nil {
		t.Fatal(err)
	}
	friend, err := UnionOfBlessings(empty, alicefriend, empty, bobfriend, empty, carol, empty)
	if err != nil {
		t.Fatal(err)
	}

	if err := checkBlessings(friend, call("Method", "Suffix")); err != nil {
		// The authorizing principal does not recognize either alice or bob
		// and thus does not recognize "friend".
		t.Error(err)
	}
	if err := checkBlessings(friend, call("OtherMethod", "Suffix", alice, bob)); err != nil {
		// Caveats not satisfied.
		t.Error(err)
	}
	if err := checkBlessings(friend, call("Method", "Suffix", carol), "carol"); err != nil {
		// No caveats on the recognized "carol" blessing.
		t.Error(err)
	}
	if err := checkBlessings(friend, call("Method", "Suffix", alice), "alice:friend"); err != nil {
		// Caveats on the recognized blessing are satisfied.
		t.Error(err)
	}
	if err := checkBlessings(friend, call("Method", "Suffix", alice, carol), "carol", "alice:friend"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, call("Method", "Suffix", bob), "bob:friend"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, call("Method", "Suffix", bob, carol), "carol", "bob:friend"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, call("Method", "Suffix", alice, bob, carol), "carol", "alice:friend", "bob:friend"); err != nil {
		t.Error(err)
	}
	if err := checkBlessings(friend, call("AliceMethod", "Suffix", alice, bob), "alice:friend"); err != nil {
		// Caveats on only one of the two recognized blessings is satisfied.
		t.Error(err)
	}
	if err := checkBlessings(friend, call("BobMethod", "Suffix", alice, bob), "bob:friend"); err != nil {
		// Caveats on only one of the two recognized blessings is satisfied.
		t.Error(err)
	}

	// p can bless p3 further, allowing only method calls on 'Suffix'.
	spouse, err := p.Bless(newPrincipal(t).PublicKey(), friend, "spouse", newSuffixCaveat("Suffix"))
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
	if mixed, err := UnionOfBlessings(alice, bob); verror.ErrorID(err) != errInvalidUnion.ID || !mixed.IsZero() {
		t.Errorf("Got (%v, %v(errorid=%v)), want errorid=%v", mixed, err, verror.ErrorID(err), errInvalidUnion.ID)
	}
}

func TestCertificateCompositionAttack(t *testing.T) {
	var (
		tp = newPrincipal(t) // principal for testing blessings.

		p1    = newPrincipal(t)
		alice = blessSelf(t, p1, "alice")
		p2    = newPrincipal(t)
		bob   = blessSelf(t, p2, "bob")
		p3    = newPrincipal(t)
		p4    = newPrincipal(t)
		cp    = CallParams{Method: "Foo", LocalPrincipal: tp}
	)
	addToRoots(t, tp, alice)
	addToRoots(t, tp, bob)
	// p3 has the blessings "alice:friend" and "bob:family" (from p1 and p2 respectively).
	// It then blesses p4 as "alice:friend:spouse" with no caveat and as "bob:family:spouse"
	// with a caveat.
	alicefriend, err := p1.Bless(p3.PublicKey(), alice, "friend", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	bobfamily, err := p2.Bless(p3.PublicKey(), bob, "family", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}

	alicefriendspouse, err := p3.Bless(p4.PublicKey(), alicefriend, "spouse", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	bobfamilyspouse, err := p3.Bless(p4.PublicKey(), bobfamily, "spouse", newCaveat(NewMethodCaveat("Foo")))
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
	spousecert := alicefriendspouse.chains[0][2]
	// sanity check
	if spousecert.Extension != "spouse" || len(spousecert.Caveats) != 1 || spousecert.Caveats[0].Id != ConstCaveat.Id {
		t.Fatalf("Invalid test data. Certificate: %+v", spousecert)
	}
	// Replace the certificate in bobfamilyspouse
	bobfamilyspouse.chains[0][2] = spousecert
	if err := matchesError(checkBlessings(bobfamilyspouse, cp), "invalid Signature in certificate(for \"spouse\")"); err != nil {
		t.Fatal(err)
	}
}

func TestCertificateTamperingAttack(t *testing.T) {
	var (
		tp = newPrincipal(t) // principal for testing blessings.

		p1 = newPrincipal(t)
		p2 = newPrincipal(t)
		p3 = newPrincipal(t)

		alice = blessSelf(t, p1, "alice")
	)
	addToRoots(t, tp, alice)

	alicefriend, err := p1.Bless(p2.PublicKey(), alice, "friend", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	if err := checkBlessings(alicefriend, CallParams{LocalPrincipal: tp}, "alice:friend"); err != nil {
		t.Fatal(err)
	}
	// p3 attempts to "steal" the blessing by constructing his own certificate.
	cert := &alicefriend.chains[0][1]
	if cert.PublicKey, err = p3.PublicKey().MarshalBinary(); err != nil {
		t.Fatal(err)
	}
	if err := matchesError(checkBlessings(alicefriend, CallParams{LocalPrincipal: tp}, "alice:friend"), "invalid Signature in certificate(for \"friend\")"); err != nil {
		t.Error(err)
	}
}

func TestCertificateChainsTamperingAttack(t *testing.T) {
	var (
		tp = newPrincipal(t) // principal for testing blessings.

		p1    = newPrincipal(t)
		p2    = newPrincipal(t)
		alice = blessSelf(t, p1, "alice")
		bob   = blessSelf(t, p2, "bob")
	)
	addToRoots(t, tp, alice)
	addToRoots(t, tp, bob)

	if err := checkBlessings(alice, CallParams{LocalPrincipal: tp}, "alice"); err != nil {
		t.Fatal(err)
	}
	// Act as if alice tried to package bob's chain with her existing chains and ship it over the network.
	alice.chains = append(alice.chains, bob.chains...)
	if err := matchesError(checkBlessings(alice, CallParams{LocalPrincipal: tp}, "alice", "bob"), "two certificate chains that bind to different public keys"); err != nil {
		t.Error(err)
	}
}

func TestBlessingToAndFromWire(t *testing.T) {
	// WireBlessings and Blessings should be basically interchangeable.
	native, err := newPrincipal(t).BlessSelf("self")
	if err != nil {
		t.Fatal(err)
	}
	var wire WireBlessings
	var dup Blessings
	// native -> wire
	if err := roundTrip(native, &wire); err != nil {
		t.Fatal(err)
	}
	// wire -> native
	if err := roundTrip(wire, &dup); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(dup, native) {
		t.Errorf("native->wire->native changed value from %#v to %#v", native, dup)
	}
}

func TestBlessingsRoundTrip(t *testing.T) {
	// Test that the blessing obtained after roundtripping is identical to the original.
	b, err := newPrincipal(t).BlessSelf("self")
	if err != nil {
		t.Fatal(err)
	}
	var got Blessings
	if err := roundTrip(b, &got); err != nil || !reflect.DeepEqual(got, b) {
		t.Fatalf("Got (%#v, %v), want (%#v, nil)", got, err, b)
	}
	// Putzing around with the wire representation should break the decoding.
	otherkey, err := newPrincipal(t).PublicKey().MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var wire WireBlessings
	if err := roundTrip(b, &wire); err != nil {
		t.Fatal(err)
	}
	wire.CertificateChains[0][len(wire.CertificateChains[0])-1].PublicKey = otherkey
	err = roundTrip(wire, &got)
	if merr := matchesError(err, "invalid Signature in certificate"); merr != nil {
		t.Error(merr)
	}
	// It should be fine to send/recv empty blessings
	got = Blessings{}
	if err := roundTrip(Blessings{}, &got); err != nil || !got.IsZero() {
		t.Errorf("Got (%#v, %v) want (<zero value>, nil)", got, err)
	}
}

func TestBlessingsOnWireWithMissingCertificates(t *testing.T) {
	var (
		B = func(b Blessings, err error) Blessings {
			if err != nil {
				t.Fatal(err)
			}
			return b
		}

		// Create "leaf", a blessing involving three certificates that bind the name
		// root:middleman:leaf to the leaf principal.
		rootP      = newPrincipal(t)
		middlemanP = newPrincipal(t)
		leafP      = newPrincipal(t)
		root       = B(rootP.BlessSelf("root"))
		middleman  = B(rootP.Bless(middlemanP.PublicKey(), root, "middleman", UnconstrainedUse()))
		leaf       = B(middlemanP.Bless(leafP.PublicKey(), middleman, "leaf", UnconstrainedUse()))

		wire WireBlessings
		tmp  Blessings
		err  error
	)
	if err := roundTrip(leaf, &wire); err != nil {
		t.Fatal(err)
	}
	// We should have a certificate chain of size 3.
	chain := wire.CertificateChains[0]
	if len(chain) != 3 {
		t.Fatalf("Got a chain of %d certificates, want 3", len(chain))
	}

	C1, C2, C3 := chain[0], chain[1], chain[2]
	var CX Certificate
	// The following combinations should fail because a certificate is missing
	type C []Certificate
	tests := []struct {
		Chain []Certificate
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
		err := roundTrip(wire, &tmp)
		if merr := matchesError(err, test.Err); merr != nil {
			t.Errorf("(%d) %v [%v]", idx, merr, test.Chain)
		}
	}

	// Mulitple chains, certifying different keys should fail
	wire.CertificateChains = [][]Certificate{
		C{C1},
		C{C1, C2},
		C{C1, C2, C3},
	}
	err = roundTrip(wire, &tmp)
	if merr := matchesError(err, "bind to different public keys"); merr != nil {
		t.Error(err)
	}

	// Multiple chains certifying the same key are okay
	wire.CertificateChains = [][]Certificate{chain, chain, chain}
	if err := roundTrip(wire, &tmp); err != nil {
		t.Error(err)
	}
	// But leaving any empty chains is not okay
	for idx := 0; idx < len(wire.CertificateChains); idx++ {
		wire.CertificateChains[idx] = []Certificate{}
		err := roundTrip(wire, &tmp)
		if merr := matchesError(err, "empty certificate chain"); merr != nil {
			t.Errorf("%d: %v", idx, merr)
		}
		wire.CertificateChains[idx] = chain
	}
}

func TestOverrideCaveatValidation(t *testing.T) {
	falseReturningCav := Caveat{
		Id: uniqueid.Id{0x99, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}
	trueReturningCav := Caveat{
		Id: uniqueid.Id{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
	}
	falseResultErr := fmt.Errorf("False caveat result")

	validator := func(_ *context.T, _ Call, cavs [][]Caveat) []error {
		results := make([]error, len(cavs))
		for i, chain := range cavs {
			for _, cav := range chain {
				switch cav.Id {
				case falseReturningCav.Id:
					results[i] = falseResultErr
				case trueReturningCav.Id:
				case ConstCaveat.Id:
					if !reflect.DeepEqual(cav, UnconstrainedUse()) {
						t.Fatalf("Unexpected const caveat")
					}
				default:
					t.Fatalf("Unexpected caveat: %#v", cav)
				}
			}
		}
		return results
	}
	setCaveatValidationForTest(validator)
	defer setCaveatValidationForTest(defaultCaveatValidation)

	p := newPrincipal(t)
	ctx, cf := context.RootContext()
	defer cf()

	bu, err := p.BlessSelf("unrestricted", UnconstrainedUse())
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
	bunion, err := UnionOfBlessings(bu, bft, btt, bsepft, bseptt)
	if err != nil {
		t.Fatal(err)
	}
	if err := AddToRoots(p, bunion); err != nil {
		t.Fatal(err)
	}

	results, infos := RemoteBlessingNames(ctx, NewCall(&CallParams{
		LocalPrincipal:  p,
		RemoteBlessings: bunion,
	}))
	expectedFailInfos := map[string]error{
		"falsetrue":  falseResultErr,
		"true:false": falseResultErr,
	}
	failInfos := map[string]error{}
	for _, info := range infos {
		failInfos[info.Blessing] = info.Err
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

func TestRemoteBlessingNames(t *testing.T) {
	var (
		p          = newPrincipal(t)
		mkBlessing = func(name string, cav ...Caveat) Blessings {
			ret, err := p.BlessSelf(name, cav...)
			if err != nil {
				t.Fatalf("%q: %v", name, err)
			}
			return ret
		}

		b1 = mkBlessing("alice", newCaveat(NewMethodCaveat("Method")))
		b2 = mkBlessing("bob")

		bnames = func(b Blessings, method string) ([]string, []RejectedBlessing) {
			ctx, cancel := context.RootContext()
			defer cancel()
			return RemoteBlessingNames(ctx, NewCall(&CallParams{
				LocalPrincipal:  p,
				RemoteBlessings: b,
				Method:          method}))
		}
	)
	if err := AddToRoots(p, b1); err != nil {
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
		verror.ErrorID(rejected[0].Err) != ErrCaveatValidation.ID {
		t.Errorf("Got (%v, %v), want ([], [alice: <caveat validation error>])", accepted, rejected)
	}
	// b2 is not recognized because the roots aren't recognized.
	if accepted, rejected := bnames(b2, "Method"); len(accepted) > 0 ||
		len(rejected) != 1 ||
		rejected[0].Blessing != "bob" ||
		verror.ErrorID(rejected[0].Err) != ErrUnrecognizedRoot.ID {
		t.Errorf("Got (%v, %v), want ([], [bob: <untrusted root>])", accepted, rejected)
	}
}

func BenchmarkRemoteBlessingNames(b *testing.B) {
	p := newPrincipal(b)
	local, err := p.BlessSelf("local")
	if err != nil {
		b.Fatal(err)
	}
	remote, err := p.Bless(newPrincipal(b).PublicKey(), local, "delegate", UnconstrainedUse())
	if err != nil {
		b.Fatal(err)
	}
	AddToRoots(p, remote)
	ctx, cancel := context.RootContext()
	defer cancel()
	call := NewCall(&CallParams{
		LocalPrincipal:  p,
		RemoteBlessings: remote})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RemoteBlessingNames(ctx, call)
	}
	b.StopTimer() // So the cancel() call isn't included.
}

func TestSigningBlessings(t *testing.T) {
	var (
		google = newPrincipal(t)
		alice  = newPrincipal(t)
		bob    = newPrincipal(t)

		googleB, _ = google.BlessSelf("google")

		peerCav, _  = NewCaveat(PeerBlessingsCaveat, []BlessingPattern{"youtube"})
		trueCav, _  = NewCaveat(ConstCaveat, true)
		falseCav, _ = NewCaveat(ConstCaveat, false)

		aliceSelf, _          = alice.BlessSelf("alice")
		googleYoutubeUser, _  = google.Bless(alice.PublicKey(), googleB, "youtube:user", peerCav)
		googleAliceExpired, _ = google.Bless(alice.PublicKey(), googleB, "alice:expired", newCaveat(NewExpiryCaveat(time.Now().Add(-time.Second))))
		googleAliceFalse, _   = google.Bless(alice.PublicKey(), googleB, "alice:false", falseCav)
		googleAlice, _        = google.Bless(alice.PublicKey(), googleB, "alice", newCaveat(NewExpiryCaveat(time.Now().Add(time.Hour))), trueCav)
		aliceB, _             = UnionOfBlessings(aliceSelf, googleYoutubeUser, googleAliceExpired, googleAliceFalse, googleAlice)

		// rawNames returns the blessing names encapsulated in the provided blessings, without
		// validating any caveats and blessing roots.
		rawNames = func(b Blessings) []string {
			var ret []string
			for _, chain := range b.chains {
				if len(chain) == 0 {
					continue
				}
				ret = append(ret, claimedName(chain))
			}
			return ret
		}
	)
	if err := AddToRoots(bob, googleB); err != nil {
		t.Fatal(err)
	}

	// The blessing "google:youtube:user" should be dropped when calling
	// SigningBlessings on 'aliceB'
	aliceSigning := SigningBlessings(aliceB)

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

	names, rejected := SigningBlessingNames(ctx, bob, aliceB)
	if want := []string{"google:alice"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("SigningBlessingNames(%v): got names %v, want %v", aliceB, names, want)
	}
	if got := len(rejected); got != 4 {
		t.Fatalf("SigningBlessingNames(%v): got %d rejected blessing names, want 4", aliceB, got)
	}
	for _, r := range rejected {
		switch r.Blessing {
		case "alice":
			if got, want := verror.ErrorID(r.Err), ErrUnrecognizedRoot.ID; got != want {
				t.Errorf("SigningBlessingNames(%v): rejected blessing %v with errorID %v, want errorID %v", aliceB, r.Blessing, got, want)
			}
		case "google:alice:false":
			if got, want := verror.ErrorID(r.Err), ErrCaveatValidation.ID; got != want {
				t.Errorf("SigningBlessingNames(%v): rejected blessing %v with errorID %v, want errorID %v", aliceB, r.Blessing, got, want)
			}
		case "google:alice:expired":
			if got, want := verror.ErrorID(r.Err), ErrCaveatValidation.ID; got != want {
				t.Errorf("SigningBlessingNames(%v): rejected blessing %v with errorID %v, want errorID %v", aliceB, r.Blessing, got, want)
			}
		case "google:youtube:user":
			if got, want := verror.ErrorID(r.Err), ErrInvalidSigningBlessingCaveat.ID; got != want {
				t.Errorf("SigningBlessingNames(%v): rejected blessing %v with errorID %v, want errorID %v", aliceB, r.Blessing, got, want)
			}
		default:
			t.Errorf("SigningBlessingNames(%v): invalid rejected blessing name %v", aliceB, r.Blessing)

		}
	}
}
