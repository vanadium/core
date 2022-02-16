// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package sectest contains support for security related tests
package sectest

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/security"
	"v.io/v23/uniqueid"
	"v.io/v23/vdl"
	"v.io/v23/vom"
)

// TrustAllRoots is an implementation of security.BlessingRoots that
// trusts all roots, regardless of whether they have been added to it.
type TrustAllRoots struct {
	dump map[security.BlessingPattern][]security.PublicKey
}

func (r *TrustAllRoots) Add(root []byte, pattern security.BlessingPattern) error {
	if r.dump == nil {
		r.dump = make(map[security.BlessingPattern][]security.PublicKey)
	}
	key, err := security.UnmarshalPublicKey(root)
	if err != nil {
		return err
	}
	r.dump[pattern] = append(r.dump[pattern], key)
	return nil
}

func (r *TrustAllRoots) Recognized(cert []byte, blessing string) error {
	return nil
}

func (r *TrustAllRoots) RecognizedCert(root *security.Certificate, blessing string) error {
	return r.Recognized(root.PublicKey, blessing)
}

func (r *TrustAllRoots) Dump() map[security.BlessingPattern][]security.PublicKey {
	if r.dump == nil {
		r.dump = make(map[security.BlessingPattern][]security.PublicKey)
	}
	return r.dump
}

func (r *TrustAllRoots) DebugString() string {
	return fmt.Sprintf("%v", r)
}

type markedRoot struct {
	root    []byte
	pattern security.BlessingPattern
}

// Roots is an implementation of security.BlessingRoots that trusts the roots
// that have been added to it.
type Roots struct {
	data []markedRoot
}

func (r *Roots) Add(root []byte, pattern security.BlessingPattern) error {
	if !pattern.IsValid() {
		return fmt.Errorf("pattern %q is invalid", pattern)
	}
	r.data = append(r.data, markedRoot{root, pattern})
	return nil
}

func (r *Roots) Recognized(root []byte, blessing string) error {
	for _, mr := range r.data {
		if bytes.Equal(root, mr.root) && mr.pattern.MatchedBy(blessing) {
			return nil
		}
	}
	key, err := security.UnmarshalPublicKey(root)
	if err != nil {
		return err
	}
	return security.ErrorfUnrecognizedRoot(nil, "unrecognized public key %v in root certificate: %v", key.String(), nil)
}

func (r *Roots) RecognizedCert(root *security.Certificate, blessing string) error {
	return r.Recognized(root.PublicKey, blessing)
}

func (r *Roots) Dump() map[security.BlessingPattern][]security.PublicKey {
	ret := make(map[security.BlessingPattern][]security.PublicKey)
	for _, mr := range r.data {
		key, err := security.UnmarshalPublicKey(mr.root)
		if err != nil {
			ret[mr.pattern] = append(ret[mr.pattern], key)
		}
	}
	return ret
}

func (*Roots) DebugString() string {
	return "BlessingRoots implementation for testing purposes only"
}

// NewPrincipal creates a new security.Principal using the supplied signer,
// blessings store and roots.
func NewPrincipal(t testing.TB, signer security.Signer, store security.BlessingStore, roots security.BlessingRoots) security.Principal {
	p, err := security.CreatePrincipal(signer, store, roots)
	if err != nil {
		t.Fatalf("CreatePrincipal using ECDSA signer failed: %v", err)
	}
	return p
}

// NewPrincipalRootsOnly creates a new security.Principal using the supplied signer,
// no blessing store and &Roots{}.
func NewPrincipalRootsOnly(t testing.TB, signer security.Signer) security.Principal {
	return NewPrincipal(t, signer, nil, &Roots{})
}

// BlessSelf returns a named blessing for the supplied principal.
func BlessSelf(t *testing.T, p security.Principal, name string, caveats ...security.Caveat) security.Blessings {
	b, err := p.BlessSelf(name, caveats...)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// RoundTrip simulates a network round trip by encoding/decoding from
// to/from vom.
func RoundTrip(in, out interface{}) error {
	data, err := vom.Encode(in)
	if err != nil {
		return err
	}
	return vom.Decode(data, out)
}

// NewPublicKeyUnconstrainedCaveat creates a named, unconstrained caveat using the
// supplied principal and with no third party caveats.
func NewPublicKeyUnconstrainedCaveat(t testing.TB, p security.Principal, name string) security.Caveat {
	c, err := security.NewPublicKeyCaveat(p.PublicKey(),
		name,
		security.ThirdPartyRequirements{},
		security.UnconstrainedUse())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// NewExpiryCaveat is like security.NewNewExpiryCaveat except that it fails
// on error.
func NewExpiryCaveat(t testing.TB, until time.Time) security.Caveat {
	c, err := security.NewCaveat(security.ExpiryCaveat, until)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// NewMethodCaveat is like security.NewNewMethodCaveat except that it fails
// on error.
func NewMethodCaveat(t testing.TB, method string, additionalMethods ...string) security.Caveat {
	c, err := security.NewCaveat(security.MethodCaveat, append(additionalMethods, method))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// SuffixCaveat is a Caveat that validates iff Call.Suffix matches the string.
//
// Since at the time of this writing, it was not clear that we want to make caveats on
// suffixes generally available, this type is implemented in this test file.
// If there is a general need for such a caveat, it should be defined similar to
// other caveats (like methodCaveat) in caveat.vdl and removed from this test file.
var SuffixCaveat = security.CaveatDescriptor{
	Id:        uniqueid.Id{0xce, 0xc4, 0xd0, 0x98, 0x94, 0x53, 0x90, 0xdb, 0x15, 0x7c, 0xa8, 0x10, 0xae, 0x62, 0x80, 0x0},
	ParamType: vdl.TypeOf(string("")),
}

// NewSuffixCaveat returns a caveat for SuffixCaveat.
func NewSuffixCaveat(t *testing.T, suffix string) security.Caveat {
	c, err := security.NewCaveat(SuffixCaveat, suffix)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// AddToRoots calls security.AddAddToRoots.
func AddToRoots(t *testing.T, p security.Principal, b security.Blessings) {
	if err := security.AddToRoots(p, b); err != nil {
		t.Fatal(err)
	}
}

// GenPurposeAndMessage generates a random purpose and message of the
// the requested size.
func GenPurposeAndMessage(psize, msize int) (purpose, message []byte) {
	purpose = make([]byte, 5)
	message = make([]byte, 100)
	if _, err := rand.Reader.Read(purpose); err != nil {
		panic(err)
	}
	if _, err := rand.Reader.Read(message); err != nil {
		panic(err)
	}
	purpose = []byte("test")
	message = []byte("hello there")
	return
}

func init() {
	security.RegisterCaveatValidator(SuffixCaveat, func(ctx *context.T, call security.Call, suffix string) error {
		if suffix != call.Suffix() {
			return fmt.Errorf("suffixCaveat not met")
		}
		return nil
	})
}
