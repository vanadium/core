// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package testutil

import (
	"v.io/v23/security"
	vsecurity "v.io/x/ref/lib/security"
)

// NewPrincipal creates a new security.Principal.
//
// It is a convenience wrapper over utility functions available in the
// v.io/x/ref/lib/security package.
//
// If the set of blessingNames provided is non-empty, it creates self-signed
// blessings for each of those names and marks all of them as the default and
// shareable with all peers on the principal's blessing store.
//
// Errors are truly rare events and since this is a utility intended only for
// unittests, NewPrincipal will panic on any errors.
func NewPrincipal(blessingNames ...string) security.Principal {
	p, err := vsecurity.NewPrincipal()
	if err != nil {
		panic(err)
	}
	var def security.Blessings
	for _, n := range blessingNames {
		b, err := p.BlessSelf(n)
		if err != nil {
			panic(err)
		}
		if def, err = security.UnionOfBlessings(def, b); err != nil {
			panic(err)
		}
	}
	if !def.IsZero() {
		if err := vsecurity.SetDefaultBlessings(p, def); err != nil {
			panic(err)
		}
	}
	return p
}

// IDProvider is a convenience type to act as an "identity provider", i.e., it
// provides other principals with a blessing whose root certificate is signed
// by the IDProvider.
//
// Typical usage:
//
//    p1, p2 := NewPrincipal(), NewPrincipal()
//    idp := NewIDProvider("xyz")
//    idp.Bless(p1, "alpha")
//    idp.Bless(p2, "beta")
//
// Now, p1 and p2 will present "xyz/alpha" and "xyz/beta" as their blessing
// names and when communicating with each other, p1 and p2 will recognize these
// names as they both trust the root certificate (that of the IDProvider)
type IDProvider struct {
	p security.Principal
	b security.Blessings
}

// NewIDProvider creates an IDProvider that will bless other principals with
// extensions of 'name'.
//
// NewIDProvider panics on any errors.
func NewIDProvider(name string) *IDProvider {
	p, err := vsecurity.NewPrincipal()
	if err != nil {
		panic(err)
	}
	b, err := p.BlessSelf(name)
	if err != nil {
		panic(err)
	}
	return &IDProvider{p, b}
}

// IDProviderFromPrincipal creates and IDProvider for the given principal.  It
// will bless other principals with extensions of its default blessing.
func IDProviderFromPrincipal(p security.Principal) *IDProvider {
	b, _ := p.BlessingStore().Default()
	return &IDProvider{p, b}
}

// Bless sets up the provided principal to use blessings from idp as its
// default. It is shorthand for:
//    b, _ := idp.NewBlessings(who, extension, caveats...)
//    who.BlessingStore().SetDefault(b)
//    who.BlessingStore().Set(b, security.AllPrincipals)
//    security.AddToRoots(who, b)
func (idp *IDProvider) Bless(who security.Principal, extension string, caveats ...security.Caveat) error {
	b, err := idp.NewBlessings(who, extension, caveats...)
	if err != nil {
		return err
	}
	return vsecurity.SetDefaultBlessings(who, b)
}

// NewBlessings returns Blessings that extend the identity provider's blessing
// with 'extension' and binds it to 'p.PublicKey'.
//
// Unlike Bless, it does not modify p's BlessingStore or set of recognized root
// certificates.
func (idp *IDProvider) NewBlessings(p security.Principal, extension string, caveats ...security.Caveat) (security.Blessings, error) {
	if len(caveats) == 0 {
		caveats = append(caveats, security.UnconstrainedUse())
	}
	return idp.p.Bless(p.PublicKey(), idp.b, extension, caveats[0], caveats[1:]...)
}

// PublicKey returns the public key of the identity provider.
func (idp *IDProvider) PublicKey() security.PublicKey {
	return idp.p.PublicKey()
}
