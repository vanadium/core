// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sectestdata

import (
	"crypto"
	"crypto/x509"
	"embed"
	"fmt"
	"path"

	"v.io/v23/security"
	seclib "v.io/x/ref/lib/security"
)

//go:embed testdata/v23-private-*.key
var v23PrivateKeys embed.FS

// V23KeySetID represents a set of keys, each set contains at least one
// instance of all supported key types.
type V23KeySetID int

const (
	V23keySetA V23KeySetID = iota
	V23KeySetB
)

func v23filename(typ seclib.KeyType, set V23KeySetID) string {
	if len(typ.String()) == 0 {
		panic(fmt.Sprintf("unrecognised key type: %v", typ))
	}
	switch set {
	case V23keySetA:
		return "v23-private-a-" + typ.String() + ".key"
	case V23KeySetB:
		return "v23-private-b-" + typ.String() + ".key"
	}
	panic(fmt.Sprintf("unrecognised key set: %v", set))
}

func V23PrivateKey(typ seclib.KeyType, set V23KeySetID) crypto.PrivateKey {
	return v23PrivateKeyFile(v23filename(typ, set))
}

func v23PrivateKeyFile(name string) crypto.PrivateKey {
	data, err := v23PrivateKeys.ReadFile(path.Join("testdata", name))
	if err != nil {
		panic(err)
	}
	key, err := x509.ParsePKCS8PrivateKey(data)
	if err != nil {
		panic(err)
	}
	return key
}

func V23Signer(typ seclib.KeyType, set V23KeySetID) security.Signer {
	signer, err := signerFromCryptoKey(V23PrivateKey(typ, set))
	if err != nil {
		panic(err)
	}
	return signer
}
