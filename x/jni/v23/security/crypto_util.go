// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"fmt"
	"math/big"

	"v.io/v23/security"
)

var (
	oidPublicKeyECDSA = asn1.ObjectIdentifier{1, 2, 840, 10045, 2, 1}
	oidNamedCurveP224 = asn1.ObjectIdentifier{1, 3, 132, 0, 33}
	oidNamedCurveP256 = asn1.ObjectIdentifier{1, 2, 840, 10045, 3, 1, 7}
	oidNamedCurveP384 = asn1.ObjectIdentifier{1, 3, 132, 0, 34}
	oidNamedCurveP521 = asn1.ObjectIdentifier{1, 3, 132, 0, 35}
)

func oidFromNamedCurve(curve elliptic.Curve) (asn1.ObjectIdentifier, bool) {
	switch curve {
	case elliptic.P224():
		return oidNamedCurveP224, true
	case elliptic.P256():
		return oidNamedCurveP256, true
	case elliptic.P384():
		return oidNamedCurveP384, true
	case elliptic.P521():
		return oidNamedCurveP521, true
	}
	return nil, false
}

// marshalPKCS8PrivateKey marshals the provided ECDSA private key into the
// PKCS#8 private key format.
func marshalPKCS8PrivateKey(key *ecdsa.PrivateKey) ([]byte, error) {
	oid, ok := oidFromNamedCurve(key.PublicKey.Curve)
	if !ok {
		return nil, fmt.Errorf("illegal curve")
	}
	paramBytes, err := asn1.Marshal(oid)
	if err != nil {
		return nil, err
	}
	var algo pkix.AlgorithmIdentifier
	algo.Algorithm = oidPublicKeyECDSA
	algo.Parameters.FullBytes = paramBytes

	privBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, err
	}
	pkcs8 := struct {
		Version    int
		Algo       pkix.AlgorithmIdentifier
		PrivateKey []byte
	}{
		Version:    1,
		Algo:       algo,
		PrivateKey: privBytes,
	}
	return asn1.Marshal(pkcs8)
}

// parsePKCS8PrivateKey parses the provided private key in the PKCS#8 format.
func parsePKCS8PrivateKey(data []byte) (*ecdsa.PrivateKey, error) {
	key, err := x509.ParsePKCS8PrivateKey(data)
	if err != nil {
		return nil, err
	}
	eckey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA private key")
	}
	return eckey, nil
}

// marshalPKIXPublicKey marshals the provided ECDSA public key into the
// DER-encoded PKIX format.
func marshalPKIXPublicKey(key *ecdsa.PublicKey) ([]byte, error) {
	return x509.MarshalPKIXPublicKey(key)
}

// parsePKIXPublicKey parses the provided DER encoded public key.
func parsePKIXPublicKey(data []byte) (*ecdsa.PublicKey, error) {
	key, err := x509.ParsePKIXPublicKey(data)
	if err != nil {
		return nil, err
	}
	eckey, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not an ECDSA public key")
	}
	return eckey, nil
}

// ecdsaSignature is a helper struct that is used to (de)serialize security.Signature.
type ecdsaSignature struct {
	// R, S specify the pair of integers that make up an ECDSA signature.
	R, S *big.Int
}

// marshalECDSASignature returns the ASN.1 encoding of the provided ECDSA signature.
func marshalECDSASignature(s security.Signature) ([]byte, error) {
	sig := &ecdsaSignature{
		R: new(big.Int).SetBytes(s.R),
		S: new(big.Int).SetBytes(s.S),
	}
	return asn1.Marshal(sig)
}

// parseECDSASignature parses the ASN.1 encoded ECDSA signature.
func parseECDSASignature(data []byte) (security.Signature, error) {
	var sig ecdsaSignature
	rest, err := asn1.Unmarshal(data, &sig)
	if err != nil {
		return security.Signature{}, err
	}
	if len(rest) > 0 {
		return security.Signature{}, fmt.Errorf("shouldn't have remainder in ECDSA signature")
	}
	return security.Signature{
		R: sig.R.Bytes(),
		S: sig.S.Bytes(),
	}, nil
}
