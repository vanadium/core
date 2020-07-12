// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"bytes"
	"crypto/elliptic"
	"reflect"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/internal/sectest"
	"v.io/v23/security"
	"v.io/v23/vom"
)

func TestByteSizeECDSA(t *testing.T) {
	testByteSize(t,
		sectest.NewECDSASigner(t, elliptic.P256()),
		sectest.NewECDSASigner(t, elliptic.P256()),
		func(t testing.TB) security.Signer {
			return sectest.NewECDSASigner(t, elliptic.P256())
		},
	)
}

func TestByteSizeED25519(t *testing.T) {
	testByteSize(t,
		sectest.NewED25519Signer(t),
		sectest.NewED25519Signer(t),
		sectest.NewED25519Signer,
	)
}

func TestByteSize(t *testing.T) {
	testByteSize(t,
		sectest.NewED25519Signer(t),
		sectest.NewECDSASigner(t, elliptic.P256()),
		func(t testing.TB) security.Signer {
			return sectest.NewECDSASigner(t, elliptic.P256())
		},
	)
	testByteSize(t,
		sectest.NewECDSASigner(t, elliptic.P256()),
		sectest.NewED25519Signer(t),
		sectest.NewED25519Signer,
	)
	testByteSize(t,
		sectest.NewED25519Signer(t),
		sectest.NewECDSASigner(t, elliptic.P256()),
		func(t testing.TB) security.Signer {
			return sectest.NewECDSASigner(t, elliptic.P256())
		},
	)

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
func testByteSize(t *testing.T, s1, s2 security.Signer, sfn func(testing.TB) security.Signer) {
	blessingsize := func(b security.Blessings) int {
		buf, err := vom.Encode(b)
		if err != nil {
			t.Fatal(err)
		}
		return len(buf)
	}
	key, err := s1.PublicKey().MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var sigbytes int
	if sig, err := s2.Sign([]byte("purpose"), []byte("message")); err != nil {
		t.Fatal(err)
	} else {
		sigbytes = len(sig.R) + len(sig.S)
	}
	t.Logf("Marshaled P256 ECDSA key                   : %4d bytes", len(key))
	t.Logf("Major components of an ECDSA signature     : %4d bytes", sigbytes)
	// Byte sizes of blessings (with no caveats in any certificates).
	t.Logf("VOM type information overhead for blessings: %4d bytes", blessingsize(security.Blessings{}))
	for ncerts := 1; ncerts < 5; ncerts++ {
		b := makeBlessings(t, sfn, ncerts)
		t.Logf("Blessing with %d certificates               : %4d bytes (%v)", ncerts, blessingsize(b), b)
	}
	// Byte size of framework caveats.
	logCaveatSize := func(c security.Caveat, err error) {
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("Marshaled caveat                           : %4d bytes (%v)", len(c.ParamVom), &c)
	}
	logCaveatSize(security.NewExpiryCaveat(time.Now()))
	logCaveatSize(security.NewMethodCaveat("m"))
}

func TestBlessingCouldHaveNamesECDSA(t *testing.T) {
	testBlessingCouldHaveNames(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func TestBlessingCouldHaveNamesED25519(t *testing.T) {
	testBlessingCouldHaveNames(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func TestBlessingCouldHaveNames(t *testing.T) {
	testBlessingCouldHaveNames(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewED25519Principal(t),
	)
	testBlessingCouldHaveNames(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func testBlessingCouldHaveNames(t *testing.T, alice, bob security.Principal) {
	falseCaveat, err := security.NewCaveat(security.ConstCaveat, false)
	if err != nil {
		t.Fatal(err)
	}

	bless := func(p security.Principal, key security.PublicKey, with security.Blessings, extension string) security.Blessings {
		b, err := p.Bless(key, with, extension, falseCaveat)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	var (
		bbob = sectest.BlessSelf(t, bob, "bob:tablet")

		balice1   = sectest.BlessSelf(t, alice, "alice")
		balice2   = sectest.BlessSelf(t, alice, "alice:phone:youtube", falseCaveat)
		balice3   = bless(bob, alice.PublicKey(), bbob, "friend")
		balice, _ = security.UnionOfBlessings(balice1, balice2, balice3)
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

func TestBlessingsExpiryECDSA(t *testing.T) {
	testBlessingsExpiry(t, sectest.NewECDSASignerP256(t))
}

func TestBlessingsExpiryED25519(t *testing.T) {
	testBlessingsExpiry(t, sectest.NewED25519Signer(t))
}

func testBlessingsExpiry(t *testing.T, signer security.Signer) {
	p, err := security.CreatePrincipal(signer, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	oneHour := now.Add(time.Hour)
	twoHour := now.Add(2 * time.Hour)
	oneHourCav, err := security.NewExpiryCaveat(oneHour)
	if err != nil {
		t.Fatal(err)
	}
	twoHourCav, err := security.NewExpiryCaveat(twoHour)
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
	noExpiryB, err := p.BlessSelf("self", security.UnconstrainedUse())
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
func TestBlessingsUniqueIDECDSA(t *testing.T) {
	testBlessingsUniqueID(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)

}

func TestBlessingsUniqueIDED25519(t *testing.T) {
	testBlessingsUniqueID(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func TestBlessingsUniqueID(t *testing.T) {
	testBlessingsUniqueID(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
	)
	testBlessingsUniqueID(t,
		sectest.NewED25519Principal(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func testBlessingsUniqueID(t *testing.T, palice, pbob security.Principal) {
	var (

		// Create blessings using all the methods available to create
		// them: Bless, BlessSelf, UnionOfBlessings, NamelessBlessings.
		nameless, _  = security.NamelessBlessing(palice.PublicKey())
		alice        = sectest.BlessSelf(t, palice, "alice")
		bob          = sectest.BlessSelf(t, pbob, "bob")
		bobfriend, _ = pbob.Bless(alice.PublicKey(), bob, "friend", security.UnconstrainedUse())
		bobspouse, _ = pbob.Bless(alice.PublicKey(), bob, "spouse", security.UnconstrainedUse())

		u1, _ = security.UnionOfBlessings(alice, bobfriend, bobspouse)
		u2, _ = security.UnionOfBlessings(bobfriend, bobspouse, alice)

		all = []security.Blessings{nameless, alice, bob, bobfriend, bobspouse, u1}
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
		var deserialized security.Blessings
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
		alice1 = sectest.BlessSelf(t, palice, commonName)
		alice2 = sectest.BlessSelf(t, pbob, commonName)
	)
	if bytes.Equal(alice1.UniqueID(), alice2.UniqueID()) {
		t.Errorf("Blessings for different public keys but the same name have the same unique id!")
	}
}

func TestRootBlessingsECDSA(t *testing.T) {
	testRootBlessings(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func TestRootBlessingsED25519(t *testing.T) {
	testRootBlessings(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func testRootBlessings(t *testing.T, alpha, beta, gamma, alice security.Principal) {
	falseCaveat, err := security.NewCaveat(security.ConstCaveat, false)
	if err != nil {
		t.Fatal(err)
	}

	bless := func(p security.Principal, key security.PublicKey, with security.Blessings, extension string) security.Blessings {
		b, err := p.Bless(key, with, extension, falseCaveat)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}

	union := func(b ...security.Blessings) security.Blessings {
		ret, err := security.UnionOfBlessings(b...)
		if err != nil {
			t.Fatal(err)
		}
		return ret
	}

	var (
		balpha = sectest.BlessSelf(t, alpha, "alpha")
		bbeta  = sectest.BlessSelf(t, beta, "beta")
		bgamma = sectest.BlessSelf(t, gamma, "gamma")
		balice = sectest.BlessSelf(t, alice, "alice")

		bAlphaFriend             = bless(alpha, alice.PublicKey(), balpha, "friend")
		bBetaEnemy               = bless(beta, alice.PublicKey(), bbeta, "enemy")
		bGammaAcquaintanceFriend = bless(alpha, alice.PublicKey(), bless(gamma, alpha.PublicKey(), bgamma, "acquaintance"), "friend")

		tests = []struct {
			b security.Blessings
			r []security.Blessings
		}{
			{balice, []security.Blessings{balice}},
			{bAlphaFriend, []security.Blessings{balpha}},
			{union(balice, bAlphaFriend), []security.Blessings{balice, balpha}},
			{union(bAlphaFriend, bBetaEnemy, bGammaAcquaintanceFriend), []security.Blessings{balpha, bbeta, bgamma}},
		}
	)
	for _, test := range tests {
		roots := security.RootBlessings(test.b)
		if got, want := roots, test.r; !reflect.DeepEqual(got, want) {
			t.Errorf("%v: Got %#v, want %#v", test.b, got, want)
		}
		// Since these RootBlessings are carefully constructed, use a
		// VOM-roundtrip to ensure they are properly so.
		serialized, err := vom.Encode(roots)
		if err != nil {
			t.Errorf("%v: vom encoding error: %v", test.b, err)
		}
		var deserialized []security.Blessings
		if err := vom.Decode(serialized, &deserialized); err != nil || !reflect.DeepEqual(roots, deserialized) {
			t.Errorf("%v: Failed roundtripping: Got %#v want %#v", test.b, roots, deserialized)
		}
	}
}

func TestNamelessBlessingECDSA(t *testing.T) {
	testNamelessBlessing(t,
		sectest.NewECDSAPrincipalP256(t),
		sectest.NewECDSAPrincipalP256(t),
	)
}

func TestNamelessBlessingED25519(t *testing.T) {
	testNamelessBlessing(t,
		sectest.NewED25519Principal(t),
		sectest.NewED25519Principal(t),
	)
}

func testNamelessBlessing(t *testing.T, alice, bob security.Principal) {
	var (
		balice, _      = security.NamelessBlessing(alice.PublicKey())
		baliceagain, _ = security.NamelessBlessing(alice.PublicKey())
		bbob, _        = security.NamelessBlessing(bob.PublicKey())
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
	if got, want := len(security.RootBlessings(balice)), 0; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got := security.SigningBlessings(balice); !got.IsZero() {
		t.Errorf("got %v, want zero blessings", got)
	}
	ctx, cancel := context.RootContext()
	defer cancel()
	if gotname, gotrejected := security.SigningBlessingNames(ctx, alice, balice); len(gotname)+len(gotrejected) > 0 {
		t.Errorf("got %v, %v, want nil, nil", gotname, gotrejected)
	}
	testNamelessBlessingCall(t, ctx, alice, bob, balice, baliceagain, bbob)
}

func testNamelessBlessingCall(t *testing.T, ctx *context.T, alice, bob security.Principal, balice, baliceagain, bbob security.Blessings) {

	call := security.NewCall(&security.CallParams{
		LocalPrincipal:  alice,
		RemoteBlessings: bbob,
	})
	if gotname, gotrejected := security.RemoteBlessingNames(ctx, call); len(gotname)+len(gotrejected) > 0 {
		t.Errorf("got %v, %v, want nil, nil", gotname, gotrejected)
	}
	call = security.NewCall(&security.CallParams{
		LocalPrincipal: alice,
		LocalBlessings: balice,
	})
	if got := security.LocalBlessingNames(ctx, call); len(got) > 0 {
		t.Errorf("got %v, want nil", got)
	}
	if got := security.BlessingNames(alice, balice); len(got) > 0 {
		t.Errorf("got %v, want nil", got)
	}
	var b security.Blessings
	if err := sectest.RoundTrip(balice, &b); err != nil {
		t.Fatal(err)
	}
	if !balice.Equivalent(b) {
		t.Errorf("got %#v, want %#v", b, balice)
	}
	if _, err := alice.Bless(bob.PublicKey(), balice, "friend", security.UnconstrainedUse()); err == nil {
		t.Errorf("bless operation should have failed")
	}
	// Union of two nameless blessings should be nameless.
	if got, err := security.UnionOfBlessings(balice, baliceagain); err != nil || !got.Equivalent(balice) {
		t.Errorf("got %#v want %#v, (err = %v)", got, balice, err)
	}
	// Union of zero and nameless blessings hsoudl be nameless.
	if got, err := security.UnionOfBlessings(balice, security.Blessings{}); err != nil || !got.Equivalent(balice) {
		t.Errorf("got %#v want %#v, (err = %v)", got, balice, err)
	}
	// Union of zero, nameless, and named blessings should be the named blessings.
	named, err := alice.BlessSelf("named")
	if err != nil {
		t.Fatal(err)
	}
	if got, err := security.UnionOfBlessings(balice, security.Blessings{}, named); err != nil || !got.Equivalent(named) {
		t.Errorf("got %#v want %#v, (err = %v)", got, named, err)
	}
}

func BenchmarkBlessECDSA(b *testing.B) {
	benchmarkBless(b,
		sectest.NewECDSASigner(b, elliptic.P256()),
		sectest.NewECDSASigner(b, elliptic.P256()),
	)
}

func BenchmarkBlessED25519(b *testing.B) {
	benchmarkBless(b,
		sectest.NewED25519Signer(b),
		sectest.NewED25519Signer(b))
}

func benchmarkBless(b *testing.B, s1, s2 security.Signer) {
	p, err := security.CreatePrincipal(s1, nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	self, err := p.BlessSelf("self")
	if err != nil {
		b.Fatal(err)
	}
	// Include at least one caveat as having caveats should be the common case.
	caveat, err := security.NewExpiryCaveat(time.Now().Add(time.Hour))
	if err != nil {
		b.Fatal(err)
	}
	blessee := s2.PublicKey()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Bless(blessee, self, "friend", caveat); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkVerifyCertificateIntegrityECDSA(b *testing.B) {
	benchmarkVerifyCertificateIntegrity(b, func(t testing.TB) security.Signer {
		return sectest.NewECDSASigner(t, elliptic.P256())
	})
}

func BenchmarkVerifyCertificateIntegrityED25519(b *testing.B) {
	benchmarkVerifyCertificateIntegrity(b, sectest.NewED25519Signer)
}

func benchmarkVerifyCertificateIntegrity(b *testing.B, sfn func(testing.TB) security.Signer) {
	native := makeBlessings(b, sfn, 1)
	var wire security.WireBlessings
	if err := security.WireBlessingsFromNative(&wire, native); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := security.WireBlessingsToNative(wire, &native); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkVerifyCertificateIntegrityNoCachingECDSA(b *testing.B) {
	benchmarkVerifyCertificateIntegrityNoCaching(b, func(t testing.TB) security.Signer {
		return sectest.NewECDSASigner(t, elliptic.P256())
	})
}

func BenchmarkVerifyCertificateIntegrityNoCachingED25519(b *testing.B) {
	benchmarkVerifyCertificateIntegrityNoCaching(b, sectest.NewED25519Signer)
}

func benchmarkVerifyCertificateIntegrityNoCaching(b *testing.B, sfn func(testing.TB) security.Signer) {
	security.DisableSignatureCache()
	defer security.EnableSignatureCache()
	benchmarkVerifyCertificateIntegrity(b, sfn)
}

func makeBlessings(t testing.TB, sfn func(testing.TB) security.Signer, ncerts int) security.Blessings {
	p, err := security.CreatePrincipal(sfn(t), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := p.BlessSelf("a")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < ncerts; i++ {
		p2, err := security.CreatePrincipal(sfn(t), nil, nil)
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		b2, err := p.Bless(p2.PublicKey(), b, "a", security.UnconstrainedUse())
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		p = p2
		b = b2
	}
	return b
}
