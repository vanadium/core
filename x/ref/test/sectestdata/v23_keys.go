// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sectestdata

import (
	"crypto"
	"crypto/x509"
	"embed"
	"encoding/pem"
	"fmt"

	"v.io/v23/security"
	"v.io/x/ref/lib/security/keys"
)

//go:embed testdata/v23-private-*.key testdata/v23-encrypted-*.key
var v23PrivateKeys embed.FS

//go:embed testdata/v23-public-*.key
var v23PublicKeys embed.FS

// V23KeySetID represents a set of keys, each set contains at least one
// instance of all supported key types.
type V23KeySetID int

const (
	V23keySetA V23KeySetID = iota
	V23KeySetB
	V23keySetAEncrypted
	V23keySetBEncrypted
)

func v23filename(typ keys.CryptoAlgo, pair string, set V23KeySetID) string {
	if len(typ.String()) == 0 {
		panic(fmt.Sprintf("unrecognised key type: %v", typ))
	}
	switch set {
	case V23keySetA:
		return fmt.Sprintf("v23-%s-a-%s.key", pair, typ)
	case V23KeySetB:
		return fmt.Sprintf("v23-%s-b-%s.key", pair, typ)
	case V23keySetAEncrypted:
		return fmt.Sprintf("v23-encrypted-a-%s.key", typ)
	case V23keySetBEncrypted:
		return fmt.Sprintf("v23-encrypted-b-%s.key", typ)
	}
	panic(fmt.Sprintf("unrecognised key set: %v", set))
}

func V23PrivateKey(typ keys.CryptoAlgo, set V23KeySetID) crypto.PrivateKey {
	data := V23PrivateKeyBytes(typ, set)
	block, _ := pem.Decode(data)
	if block == nil {
		panic("no pem block found")
	}
	var key crypto.PrivateKey
	var err error
	switch block.Type {
	case "EC PRIVATE KEY":
		key, err = x509.ParseECPrivateKey(block.Bytes)
	default:
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	}
	if err != nil {
		panic(err)
	}
	return key
}

func V23PrivateKeyBytes(typ keys.CryptoAlgo, set V23KeySetID) []byte {
	return fileContents(v23PrivateKeys, v23filename(typ, "private", set))
}

func V23PublicKeyBytes(typ keys.CryptoAlgo, set V23KeySetID) []byte {
	return fileContents(v23PublicKeys, v23filename(typ, "public", set))
}

func V23Signer(typ keys.CryptoAlgo, set V23KeySetID) security.Signer {
	signer, err := signerFromCryptoKey(V23PrivateKey(typ, set))
	if err != nil {
		panic(err)
	}
	return signer
}
