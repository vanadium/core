// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"bytes"
	"crypto/x509"
	"fmt"
	"reflect"
	"time"
)

// CreatePrincipal returns a Principal that uses 'signer' for all
// private key operations, 'store' for storing blessings bound
// to the Principal and 'roots' for the set of authoritative public
// keys on blessings recognized by this Principal.
//
// If provided 'roots' is nil then the Principal does not trust any
// public keys and all subsequent 'AddToRoots' operations fail.
//
// It returns an error if store.PublicKey does not match signer.PublicKey.
//
// NOTE: v.io/x/ref/lib/security provides implementations
// NOTE: v.io/x/ref/lib/testutil/security provides utility methods for creating
// principals for testing purposes.
func CreatePrincipal(signer Signer, store BlessingStore, roots BlessingRoots) (Principal, error) {
	if store == nil {
		store = errStore{signer.PublicKey()}
	}
	if roots == nil {
		roots = errRoots{}
	}
	if got, want := store.PublicKey(), signer.PublicKey(); !reflect.DeepEqual(got, want) {
		return nil, fmt.Errorf("store's public key: %v does not match signer's public key: %v", got, want)
	}
	return &principal{signer: signer, publicKey: signer.PublicKey(), store: store, roots: roots}, nil
}

// CreatePrincipalPublicKeyOnly returns a Principal that cannot sign new blessings
// and has only the public key portion of the original key pair used to create
// it. Such a principal is intended for use in situations where only signature
// verification is required and with no need/ability for RPC based communication
// since that requires signing capability.
func CreatePrincipalPublicKeyOnly(publicKey PublicKey, store BlessingStore, roots BlessingRoots) (Principal, error) {
	if store == nil {
		store = errStore{publicKey}
	}
	if roots == nil {
		roots = errRoots{}
	}
	return &principal{signer: nil, publicKey: publicKey, store: store, roots: roots}, nil
}

var (
	// Every Signer.Sign operation conducted by the principal fills in a
	// "purpose" before signing to prevent "type attacks" so that a signature obtained
	// for one operation (e.g. Principal.Sign) cannot be re-purposed for another
	// operation (e.g. Principal.Bless). If the "Principal" object lives in an
	// external "agent" process, this works out well since this other process
	// can confidently audit all private key operations.
	// For this to work, ALL private key operations by the Principal must
	// use a distinct "purpose" and no two "purpose"s should share a prefix
	// or suffix.
	blessPurpose     = []byte(SignatureForBlessingCertificates)
	signPurpose      = []byte(SignatureForMessageSigning)
	dischargePurpose = []byte(SignatureForDischarge)
)

type errStore struct {
	key PublicKey
}

func (errStore) Set(Blessings, BlessingPattern) (Blessings, error) {
	return Blessings{}, fmt.Errorf("underlying BlessingStore object is nil")
}
func (errStore) ForPeer(peerBlessings ...string) Blessings { return Blessings{} }
func (errStore) SetDefault(blessings Blessings) error {
	return fmt.Errorf("underlying BlessingStore object is nil")
}
func (errStore) Default() (Blessings, <-chan struct{})                         { return Blessings{}, nil }
func (errStore) PeerBlessings() map[BlessingPattern]Blessings                  { return nil }
func (errStore) CacheDischarge(Discharge, Caveat, DischargeImpetus) error      { return nil }
func (errStore) ClearDischarges(...Discharge)                                  {}
func (errStore) Discharge(Caveat, DischargeImpetus) (d Discharge, t time.Time) { return d, t }
func (errStore) DebugString() string {
	return fmt.Errorf("underlying BlessingStore object is nil").Error()
}
func (s errStore) PublicKey() PublicKey { return s.key }

type errRoots struct{}

func (errRoots) Add([]byte, BlessingPattern) error {
	return fmt.Errorf("underlying BlessingRoots object is nil")
}

func (errRoots) Recognized([]byte, string) error {
	return fmt.Errorf("underlying BlessingRoots object is nil")
}

func (errRoots) RecognizedCert(*Certificate, string) error {
	return fmt.Errorf("underlying BlessingRoots object is nil")
}

func (errRoots) Dump() map[BlessingPattern][]PublicKey { return nil }

func (errRoots) DebugString() string {
	return fmt.Errorf("underlying BlessingRoots object is nil").Error()
}

type principal struct {
	signer    Signer
	publicKey PublicKey
	roots     BlessingRoots
	store     BlessingStore
}

func (p *principal) Bless(key PublicKey, with Blessings, extension string, caveat Caveat, additionalCaveats ...Caveat) (Blessings, error) {
	if with.IsZero() || with.isNamelessBlessing() {
		return Blessings{}, fmt.Errorf("the Blessings to bless 'with' must have at least one certificate")
	}
	if !reflect.DeepEqual(with.PublicKey(), p.PublicKey()) {
		return Blessings{}, fmt.Errorf("Principal with public key %v cannot extend blessing with public key %v", p.PublicKey(), with.PublicKey())
	}
	caveats := make([]Caveat, len(additionalCaveats)+1)
	copy(caveats, additionalCaveats)
	caveats[len(additionalCaveats)] = caveat
	cert, err := newUnsignedCertificate(extension, key, caveats...)
	if err != nil {
		return Blessings{}, err
	}
	chains := with.chains
	newchains := make([][]Certificate, len(chains))
	newdigests := make([][]byte, len(chains))
	for idx, chain := range chains {
		if p.signer == nil {
			return Blessings{}, fmt.Errorf("underlying signer is nil")
		}
		if newchains[idx], newdigests[idx], err = chainCertificate(p.signer, chain, *cert); err != nil {
			return Blessings{}, err
		}
	}
	ret := Blessings{
		chains:    newchains,
		publicKey: key,
		digests:   newdigests,
	}
	ret.init()
	return ret, nil
}

func (p *principal) BlessSelf(name string, caveats ...Caveat) (Blessings, error) {
	if cert := p.publicKey.X509Certificate(); cert != nil {
		return p.blessSelfX509(name, cert, caveats)
	}
	return p.blessSelf(name, caveats)
}

func (p *principal) blessSelf(name string, caveats []Caveat) (Blessings, error) {
	if p.signer == nil {
		return Blessings{}, fmt.Errorf("underlying signer is nil")
	}
	cert, err := newUnsignedCertificate(name, p.PublicKey(), caveats...)
	if err != nil {
		return Blessings{}, err
	}
	chain, digest, err := chainCertificate(p.signer, nil, *cert)
	if err != nil {
		return Blessings{}, err
	}
	ret := Blessings{
		chains:    [][]Certificate{chain},
		publicKey: p.PublicKey(),
		digests:   [][]byte{digest},
	}
	ret.init()
	return ret, nil
}

func (p *principal) BlessSelfX509(host string, x509Cert *x509.Certificate, caveats ...Caveat) (Blessings, error) {
	return Blessings{}, fmt.Errorf("oops")
}

func (p *principal) blessSelfX509(host string, x509Cert *x509.Certificate, caveats []Caveat) (Blessings, error) {
	if p.signer == nil {
		return Blessings{}, fmt.Errorf("underlying signer is nil")
	}
	pkBytes, err := x509.MarshalPKIXPublicKey(x509Cert.PublicKey)
	if err != nil {
		return Blessings{}, err
	}
	if !bytes.Equal(p.publicKey.bytes(), pkBytes) {
		return Blessings{}, fmt.Errorf("public key associated with this principal and the x509 certificate differ")
	}
	certs, err := newUnsignedCertificateFromX509(host, x509Cert, pkBytes, caveats)
	if err != nil {
		return Blessings{}, err
	}
	if len(certs) == 0 {
		return Blessings{}, fmt.Errorf("failed to create any certificates based on the supplied x509 certificate")
	}
	chains := make([][]Certificate, len(certs))
	digests := make([][]byte, len(certs))
	for i := range certs {
		chains[i], digests[i], err = chainCertificate(p.signer, nil, certs[i])
		if err != nil {
			return Blessings{}, err
		}
	}
	ret := Blessings{
		chains:    chains,
		publicKey: p.PublicKey(),
		digests:   digests,
	}
	ret.init()
	return ret, nil
}

func (p *principal) Sign(message []byte) (Signature, error) {
	if p.signer == nil {
		return Signature{}, fmt.Errorf("signing not supported, only public key available")
	}
	return p.signer.Sign(signPurpose, message)
}

func (p *principal) MintDischarge(forCaveat, caveatOnDischarge Caveat, additionalCaveatsOnDischarge ...Caveat) (Discharge, error) {
	if forCaveat.Id != PublicKeyThirdPartyCaveat.Id {
		return Discharge{}, fmt.Errorf("cannot mint discharges for %v", forCaveat)
	}
	id := forCaveat.ThirdPartyDetails().ID()
	dischargeCaveats := make([]Caveat, len(additionalCaveatsOnDischarge)+1)
	copy(dischargeCaveats, additionalCaveatsOnDischarge)
	dischargeCaveats[len(additionalCaveatsOnDischarge)] = caveatOnDischarge
	d := PublicKeyDischarge{ThirdPartyCaveatId: id, Caveats: dischargeCaveats}
	if err := d.sign(p.signer); err != nil {
		return Discharge{}, fmt.Errorf("failed to sign discharge: %v", err)
	}
	return Discharge{WireDischargePublicKey{d}}, nil
}

func (p *principal) PublicKey() PublicKey {
	if p.signer == nil {
		return p.publicKey
	}
	return p.signer.PublicKey()
}

func (p *principal) BlessingStore() BlessingStore {
	return p.store
}

func (p *principal) Roots() BlessingRoots {
	return p.roots
}
