// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"bytes"
	gocontext "context"
	"reflect"
	"runtime"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/internal/sectest"
	"v.io/v23/security"
	"v.io/v23/vom"
	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/test/sectestdata"
)

type useOrCreateSigners struct {
	used      int
	available []security.Signer
	keyType   keys.CryptoAlgo
}

func (uc *useOrCreateSigners) nextSigner(t testing.TB) security.Signer {
	if uc.used < len(uc.available) {
		uc.used++
		return uc.available[uc.used-1]
	}
	signer, err := seclib.NewSigner(gocontext.Background(), uc.keyType)
	if err != nil {
		t.Fatal(err)
	}
	return signer
}

func newUseOrCreateSigners(keyType keys.CryptoAlgo, signers ...security.Signer) func(t testing.TB) security.Signer {
	uc := &useOrCreateSigners{keyType: keyType, available: signers}
	return uc.nextSigner
}

func TestByteSize(t *testing.T) {
	for _, kt := range testCryptoAlgos {
		testByteSize(t, kt.String(),
			sectestdata.V23Signer(kt, sectestdata.V23KeySetA),
			sectestdata.V23Signer(kt, sectestdata.V23KeySetB),
			newUseOrCreateSigners(kt,
				sectestdata.V23Signer(kt, sectestdata.V23KeySetC),
				sectestdata.V23Signer(kt, sectestdata.V23KeySetD),
				sectestdata.V23Signer(kt, sectestdata.V23KeySetE),
			),
		)
	}
}

func verifyBlessingSignatures(t *testing.T, blessings ...security.Blessings) {
	for _, b := range blessings {
		if err := security.ExposeVerifySignature(&b); err != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Fatalf("line %v: invalid signature for blessing %v: %v", line, b.String(), err)
		}
	}

}

// Log the "on-the-wire" sizes for blessings (which are shipped during the
// authentication protocol).
// As of February 27, 2015, the numbers were:
//
//	Marshaled P256 ECDSA key                   :   91 bytes
//	Major components of an ECDSA signature     :   64 bytes
//	VOM type information overhead for blessings:  354 bytes
//	Blessing with 1 certificates               :  536 bytes (a)
//	Blessing with 2 certificates               :  741 bytes (a:a)
//	Blessing with 3 certificates               :  945 bytes (a:a:a)
//	Blessing with 4 certificates               : 1149 bytes (a:a:a:a)
//	Marshaled caveat                           :   55 bytes (0xa64c2d0119fba3348071feeb2f308000(time.Time=0001-01-01 00:00:00 +0000 UTC))
//	Marshaled caveat                           :    6 bytes (0x54a676398137187ecdb26d2d69ba0003([]string=[m]))
func testByteSize(t *testing.T, algo string, s1, s2 security.Signer, sfn func(testing.TB) security.Signer) {
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
	t.Logf("Marshaled % 10s key                   : %4d bytes", algo, len(key))
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

func TestBlessingCouldHaveNames(t *testing.T) {
	twoPrincipalTest(t, "testBlessingCouldHaveNames", testBlessingCouldHaveNames)
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

		balice1 = sectest.BlessSelf(t, alice, "alice")
		balice2 = sectest.BlessSelf(t, alice, "alice:phone:youtube", falseCaveat)
		balice3 = bless(bob, alice.PublicKey(), bbob, "friend")
	)
	balice, err := security.UnionOfBlessings(balice1, balice2, balice3)
	if err != nil {
		t.Fatal(err)
	}

	verifyBlessingSignatures(t, bbob, balice1, balice2, balice3, balice)
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
	for _, kt := range testCryptoAlgos {
		testBlessingsExpiry(t, sectestdata.V23Signer(kt, sectestdata.V23KeySetA))
	}
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

func TestBlessingsUniqueID(t *testing.T) {
	twoPrincipalTest(t, "testBlessingsUniqueID", testBlessingsUniqueID)
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

	verifyBlessingSignatures(t, alice, bob, bobfriend, bobspouse, u1, u2)

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
			if err != nil {
				t.Errorf("VOM decode error: %v", err)
			} else {
				t.Errorf("%q: UniqueID mismatch after VOM round-tripping.", b1)
			}
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

func TestRootBlessings(t *testing.T) {
	fourPrincipalTest(t, "testRootBlessings", testRootBlessings)
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

	verifyBlessingSignatures(t, balpha, bbeta, bgamma, balice, bAlphaFriend, bBetaEnemy, bGammaAcquaintanceFriend)
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

func TestNamelessBlessing(t *testing.T) {
	twoPrincipalTest(t, "testNamelessBlessing", testNamelessBlessing)
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
	// Union of zero and nameless blessings should be nameless.
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
	benchmarkBless(b, ecdsa256SignerA, ecdsa256SignerB)
}

func BenchmarkBlessED25519(b *testing.B) {
	benchmarkBless(b, ed25519SignerA, ed25519SignerB)
}

func BenchmarkBlessRSA2048(b *testing.B) {
	benchmarkBless(b, rsa2048SignerA, rsa2048SignerB)
}

func BenchmarkBlessRSA4096(b *testing.B) {
	benchmarkBless(b, rsa4096SignerA, rsa4096SignerB)
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
	benchmarkVerifyCertificateIntegrity(b, keys.ECDSA256)
}

func BenchmarkVerifyCertificateIntegrityED25519(b *testing.B) {
	benchmarkVerifyCertificateIntegrity(b, keys.ED25519)
}

func BenchmarkVerifyCertificateIntegrityRSA(b *testing.B) {
	benchmarkVerifyCertificateIntegrity(b, keys.RSA2048)
}

func benchmarkVerifyCertificateIntegrity(b *testing.B, kt keys.CryptoAlgo) {
	sfn := newUseOrCreateSigners(
		kt,
		sectestdata.V23Signer(kt, sectestdata.V23KeySetA),
		sectestdata.V23Signer(kt, sectestdata.V23KeySetB))
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
	benchmarkVerifyCertificateIntegrityNoCaching(b, keys.ECDSA256)
}

func BenchmarkVerifyCertificateIntegrityNoCachingED25519(b *testing.B) {
	benchmarkVerifyCertificateIntegrityNoCaching(b, keys.ED25519)
}

func BenchmarkVerifyCertificateIntegrityNoCachingRSA2048(b *testing.B) {
	benchmarkVerifyCertificateIntegrityNoCaching(b, keys.RSA2048)
}

func benchmarkVerifyCertificateIntegrityNoCaching(b *testing.B, kt keys.CryptoAlgo) {
	security.DisableSignatureCache()
	defer security.EnableSignatureCache()
	benchmarkVerifyCertificateIntegrity(b, kt)
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
