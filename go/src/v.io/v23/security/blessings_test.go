// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"reflect"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/vom"
)

func newSigner() Signer {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return NewInMemoryECDSASigner(key)
}

// Log the "on-the-wire" sizes for blessings (which are shipped during the
// authentication protocol).
// As of February 27, 2015, the numbers were:
//   Marshaled P256 ECDSA key                   :   91 bytes
//   Major components of an ECDSA signature     :   64 bytes
//   VOM type information overhead for blessings:  354 bytes
//   Blessing with 1 certificates               :  536 bytes (a)
//   Blessing with 2 certificates               :  741 bytes (a:a)
//   Blessing with 3 certificates               :  945 bytes (a:a:a)
//   Blessing with 4 certificates               : 1149 bytes (a:a:a:a)
//   Marshaled caveat                           :   55 bytes (0xa64c2d0119fba3348071feeb2f308000(time.Time=0001-01-01 00:00:00 +0000 UTC))
//   Marshaled caveat                           :    6 bytes (0x54a676398137187ecdb26d2d69ba0003([]string=[m]))
func TestByteSize(t *testing.T) {
	blessingsize := func(b Blessings) int {
		buf, err := vom.Encode(b)
		if err != nil {
			t.Fatal(err)
		}
		return len(buf)
	}
	key, err := newSigner().PublicKey().MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var sigbytes int
	if sig, err := newSigner().Sign([]byte("purpose"), []byte("message")); err != nil {
		t.Fatal(err)
	} else {
		sigbytes = len(sig.R) + len(sig.S)
	}
	t.Logf("Marshaled P256 ECDSA key                   : %4d bytes", len(key))
	t.Logf("Major components of an ECDSA signature     : %4d bytes", sigbytes)
	// Byte sizes of blessings (with no caveats in any certificates).
	t.Logf("VOM type information overhead for blessings: %4d bytes", blessingsize(Blessings{}))
	for ncerts := 1; ncerts < 5; ncerts++ {
		b := makeBlessings(t, ncerts)
		t.Logf("Blessing with %d certificates               : %4d bytes (%v)", ncerts, blessingsize(b), b)
	}
	// Byte size of framework caveats.
	logCaveatSize := func(c Caveat, err error) {
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Marshaled caveat                           : %4d bytes (%v)", len(c.ParamVom), &c)
	}
	logCaveatSize(NewExpiryCaveat(time.Now()))
	logCaveatSize(NewMethodCaveat("m"))
}

func TestBlessingCouldHaveNames(t *testing.T) {
	falseCaveat, err := NewCaveat(ConstCaveat, false)
	if err != nil {
		t.Fatal(err)
	}

	bless := func(p Principal, key PublicKey, with Blessings, extension string) Blessings {
		b, err := p.Bless(key, with, extension, falseCaveat)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	var (
		alice = newPrincipal(t)
		bob   = newPrincipal(t)

		bbob = blessSelf(t, bob, "bob:tablet")

		balice1   = blessSelf(t, alice, "alice")
		balice2   = blessSelf(t, alice, "alice:phone:youtube", falseCaveat)
		balice3   = bless(bob, alice.PublicKey(), bbob, "friend")
		balice, _ = UnionOfBlessings(balice1, balice2, balice3)
	)

	tests := []struct {
		names  []string
		result bool
	}{
		{[]string{"alice", "alice:phone:youtube", "bob:tablet:friend"}, true},
		{[]string{"alice", "alice:phone:youtube"}, true},
		{[]string{"alice:phone:youtube", "bob:tablet:friend"}, true},
		{[]string{"alice", "bob:tablet:friend"}, true},
		{[]string{"alice"}, true},
		{[]string{"alice:phone:youtube"}, true},
		{[]string{"bob:tablet:friend"}, true},
		{[]string{"alice:tablet"}, false},
		{[]string{"alice:phone"}, false},
		{[]string{"bob:tablet"}, false},
		{[]string{"bob:tablet:friend:spouse"}, false},
		{[]string{"carol:phone"}, false},
	}
	for _, test := range tests {
		if got, want := balice.CouldHaveNames(test.names), test.result; got != want {
			t.Errorf("%v.CouldHaveNames(%v): got %v, want %v", balice, test.names, got, want)
		}
	}
}

func TestBlessingsExpiry(t *testing.T) {
	p, err := CreatePrincipal(newSigner(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	oneHour := now.Add(time.Hour)
	twoHour := now.Add(2 * time.Hour)
	oneHourCav, err := NewExpiryCaveat(oneHour)
	if err != nil {
		t.Fatal(err)
	}
	twoHourCav, err := NewExpiryCaveat(twoHour)
	if err != nil {
		t.Fatal(err)
	}
	// twoHourB should expiry in two hours.
	twoHourB, err := p.BlessSelf("self", twoHourCav)
	if err != nil {
		t.Fatal(err)
	}
	// oneHourB should expiry in one hour.
	oneHourB, err := p.BlessSelf("self", oneHourCav, twoHourCav)
	if err != nil {
		t.Fatal(err)
	}
	// noExpiryB should never expiry.
	noExpiryB, err := p.BlessSelf("self", UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	if exp := noExpiryB.Expiry(); !exp.IsZero() {
		t.Errorf("got %v, want %v", exp, time.Time{})
	}
	if got, want := oneHourB.Expiry().UTC(), oneHour.UTC(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := twoHourB.Expiry().UTC(), twoHour.UTC(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBlessingsUniqueID(t *testing.T) {
	var (
		palice = newPrincipal(t)
		pbob   = newPrincipal(t)

		// Create blessings using all the methods available to create
		// them: Bless, BlessSelf, UnionOfBlessings, NamelessBlessings.
		nameless, _  = NamelessBlessing(palice.PublicKey())
		alice        = blessSelf(t, palice, "alice")
		bob          = blessSelf(t, pbob, "bob")
		bobfriend, _ = pbob.Bless(alice.PublicKey(), bob, "friend", UnconstrainedUse())
		bobspouse, _ = pbob.Bless(alice.PublicKey(), bob, "spouse", UnconstrainedUse())

		u1, _ = UnionOfBlessings(alice, bobfriend, bobspouse)
		u2, _ = UnionOfBlessings(bobfriend, bobspouse, alice)

		all = []Blessings{nameless, alice, bob, bobfriend, bobspouse, u1}
	)
	// Each individual blessing should have a different UniqueID, and different from u1
	for i := 0; i < len(all); i++ {
		b1 := all[i]
		for j := i + 1; j < len(all); j++ {
			if b2 := all[j]; bytes.Equal(b1.UniqueID(), b2.UniqueID()) {
				t.Errorf("%q and %q have the same UniqueID!", b1, b2)
			}
		}
		// Each blessings object must have a unique ID (whether created
		// by blessing self, blessed by another principal, or
		// roundtripped through VOM)
		if len(b1.UniqueID()) == 0 {
			t.Errorf("%q has no UniqueID", b1)
		}
		serialized, err := vom.Encode(b1)
		if err != nil {
			t.Errorf("%q failed VOM encoding: %v", b1, err)
		}
		var deserialized Blessings
		if err := vom.Decode(serialized, &deserialized); err != nil || !bytes.Equal(b1.UniqueID(), deserialized.UniqueID()) {
			t.Errorf("%q: UniqueID mismatch after VOM round-tripping. VOM decode error: %v", b1, err)
		}
	}
	// u1 and u2 should have the same UniqueID
	if !bytes.Equal(u1.UniqueID(), u2.UniqueID()) {
		t.Errorf("%q and %q have different UniqueIDs", u1, u2)
	}

	// Finally, two blessings with the same name but different public keys
	// should not have the same unique id.
	const commonName = "alice"
	var (
		alice1 = blessSelf(t, palice, commonName)
		alice2 = blessSelf(t, pbob, commonName)
	)
	if bytes.Equal(alice1.UniqueID(), alice2.UniqueID()) {
		t.Errorf("Blessings for different public keys but the same name have the same unique id!")
	}
}

func TestRootBlessings(t *testing.T) {
	falseCaveat, err := NewCaveat(ConstCaveat, false)
	if err != nil {
		t.Fatal(err)
	}

	bless := func(p Principal, key PublicKey, with Blessings, extension string) Blessings {
		b, err := p.Bless(key, with, extension, falseCaveat)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	union := func(b ...Blessings) Blessings {
		ret, err := UnionOfBlessings(b...)
		if err != nil {
			t.Fatal(err)
		}
		return ret
	}

	var (
		alpha = newPrincipal(t)
		beta  = newPrincipal(t)
		gamma = newPrincipal(t)
		alice = newPrincipal(t)

		balpha = blessSelf(t, alpha, "alpha")
		bbeta  = blessSelf(t, beta, "beta")
		bgamma = blessSelf(t, gamma, "gamma")
		balice = blessSelf(t, alice, "alice")

		bAlphaFriend             = bless(alpha, alice.PublicKey(), balpha, "friend")
		bBetaEnemy               = bless(beta, alice.PublicKey(), bbeta, "enemy")
		bGammaAcquaintanceFriend = bless(alpha, alice.PublicKey(), bless(gamma, alpha.PublicKey(), bgamma, "acquaintance"), "friend")

		tests = []struct {
			b Blessings
			r []Blessings
		}{
			{balice, []Blessings{balice}},
			{bAlphaFriend, []Blessings{balpha}},
			{union(balice, bAlphaFriend), []Blessings{balice, balpha}},
			{union(bAlphaFriend, bBetaEnemy, bGammaAcquaintanceFriend), []Blessings{balpha, bbeta, bgamma}},
		}
	)
	for _, test := range tests {
		roots := RootBlessings(test.b)
		if got, want := roots, test.r; !reflect.DeepEqual(got, want) {
			t.Errorf("%v: Got %#v, want %#v", test.b, got, want)
		}
		// Since these RootBlessings are carefully constructed, use a
		// VOM-roundtrip to ensure they are properly so.
		serialized, err := vom.Encode(roots)
		if err != nil {
			t.Errorf("%v: vom encoding error: %v", test.b, err)
		}
		var deserialized []Blessings
		if err := vom.Decode(serialized, &deserialized); err != nil || !reflect.DeepEqual(roots, deserialized) {
			t.Errorf("%v: Failed roundtripping: Got %#v want %#v", test.b, roots, deserialized)
		}
	}
}

func TestNamelessBlessing(t *testing.T) {
	var (
		alice          = newPrincipal(t)
		bob            = newPrincipal(t)
		balice, _      = NamelessBlessing(alice.PublicKey())
		baliceagain, _ = NamelessBlessing(alice.PublicKey())
		bbob, _        = NamelessBlessing(bob.PublicKey())
	)
	if got, want := balice.PublicKey(), alice.PublicKey(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := len(balice.ThirdPartyCaveats()), 0; got != want {
		t.Errorf("got %v tpcavs, want %v", got, want)
	}
	if !balice.Equivalent(baliceagain) {
		t.Errorf("expected blessings to be equivalent")
	}
	if balice.Equivalent(bbob) {
		t.Errorf("expected blessings to differ")
	}
	if ua, ub := balice.UniqueID(), bbob.UniqueID(); bytes.Equal(ua, ub) {
		t.Errorf("%q and %q should not be equal", ua, ub)
	}
	if ua, uaa := balice.UniqueID(), baliceagain.UniqueID(); !bytes.Equal(ua, uaa) {
		t.Errorf("balice and baliceagain uniqueid should be equal")
	}
	if balice.CouldHaveNames([]string{"alice"}) {
		t.Errorf("blessing should not have names")
	}
	if !balice.Expiry().IsZero() {
		t.Errorf("blessing should not expire")
	}
	if got, want := balice.String(), ""; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := len(RootBlessings(balice)), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := SigningBlessings(balice); !got.IsZero() {
		t.Errorf("got %v, want zero blessings", got)
	}
	ctx, cancel := context.RootContext()
	defer cancel()
	if gotname, gotrejected := SigningBlessingNames(ctx, alice, balice); len(gotname)+len(gotrejected) > 0 {
		t.Errorf("got %v, %v, want nil, nil", gotname, gotrejected)
	}
	call := NewCall(&CallParams{
		LocalPrincipal:  alice,
		RemoteBlessings: bbob,
	})
	if gotname, gotrejected := RemoteBlessingNames(ctx, call); len(gotname)+len(gotrejected) > 0 {
		t.Errorf("got %v, %v, want nil, nil", gotname, gotrejected)
	}
	call = NewCall(&CallParams{
		LocalPrincipal: alice,
		LocalBlessings: balice,
	})
	if got := LocalBlessingNames(ctx, call); len(got) > 0 {
		t.Errorf("got %v, want nil", got)
	}
	if got := BlessingNames(alice, balice); len(got) > 0 {
		t.Errorf("got %v, want nil", got)
	}
	var b Blessings
	if err := roundTrip(balice, &b); err != nil {
		t.Fatal(err)
	}
	if !balice.Equivalent(b) {
		t.Errorf("got %#v, want %#v", b, balice)
	}
	if _, err := alice.Bless(bob.PublicKey(), balice, "friend", UnconstrainedUse()); err == nil {
		t.Errorf("bless operation should have failed")
	}
	// Union of two nameless blessings should be nameless.
	if got, err := UnionOfBlessings(balice, baliceagain); err != nil || !got.Equivalent(balice) {
		t.Errorf("got %#v want %#v, (err = %v)", got, balice, err)
	}
	// Union of zero and nameless blessings hsoudl be nameless.
	if got, err := UnionOfBlessings(balice, Blessings{}); err != nil || !got.Equivalent(balice) {
		t.Errorf("got %#v want %#v, (err = %v)", got, balice, err)
	}
	// Union of zero, nameless, and named blessings should be the named blessings.
	named, err := alice.BlessSelf("named")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := UnionOfBlessings(balice, Blessings{}, named); err != nil || !got.Equivalent(named) {
		t.Errorf("got %#v want %#v, (err = %v)", got, named, err)
	}
}

func BenchmarkBless(b *testing.B) {
	p, err := CreatePrincipal(newSigner(), nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	self, err := p.BlessSelf("self")
	if err != nil {
		b.Fatal(err)
	}
	// Include at least one caveat as having caveats should be the common case.
	caveat, err := NewExpiryCaveat(time.Now().Add(time.Hour))
	if err != nil {
		b.Fatal(err)
	}
	blessee := newSigner().PublicKey()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Bless(blessee, self, "friend", caveat); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifyCertificateIntegrity(b *testing.B) {
	native := makeBlessings(b, 1)
	var wire WireBlessings
	if err := WireBlessingsFromNative(&wire, native); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := WireBlessingsToNative(wire, &native); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifyCertificateIntegrity_NoCaching(b *testing.B) {
	signatureCache.disable()
	defer signatureCache.enable()
	BenchmarkVerifyCertificateIntegrity(b)
}

func makeBlessings(t testing.TB, ncerts int) Blessings {
	p, err := CreatePrincipal(newSigner(), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.BlessSelf("a")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < ncerts; i++ {
		p2, err := CreatePrincipal(newSigner(), nil, nil)
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		b2, err := p.Bless(p2.PublicKey(), b, "a", UnconstrainedUse())
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		p = p2
		b = b2
	}
	return b
}
