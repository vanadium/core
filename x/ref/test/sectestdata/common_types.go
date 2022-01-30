// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sectestdata

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"reflect"

	"v.io/x/ref/lib/security/keys"
)

var (
	publicKeyTypes  = map[keys.CryptoAlgo]string{}
	privateKeyTypes = map[keys.CryptoAlgo]string{}
)

func init() {
	publicKeyTypes[keys.ECDSA256] = reflect.TypeOf(&ecdsa.PublicKey{}).String()
	publicKeyTypes[keys.ECDSA384] = reflect.TypeOf(&ecdsa.PublicKey{}).String()
	publicKeyTypes[keys.ECDSA521] = reflect.TypeOf(&ecdsa.PublicKey{}).String()
	publicKeyTypes[keys.ED25519] = reflect.TypeOf(ed25519.PublicKey{}).String()
	publicKeyTypes[keys.RSA2048] = reflect.TypeOf(&rsa.PublicKey{}).String()
	publicKeyTypes[keys.RSA4096] = reflect.TypeOf(&rsa.PublicKey{}).String()

	privateKeyTypes[keys.ECDSA256] = reflect.TypeOf(&ecdsa.PrivateKey{}).String()
	privateKeyTypes[keys.ECDSA384] = reflect.TypeOf(&ecdsa.PrivateKey{}).String()
	privateKeyTypes[keys.ECDSA521] = reflect.TypeOf(&ecdsa.PrivateKey{}).String()
	privateKeyTypes[keys.ED25519] = reflect.TypeOf(ed25519.PrivateKey{}).String()
	privateKeyTypes[keys.RSA2048] = reflect.TypeOf(&rsa.PrivateKey{}).String()
	privateKeyTypes[keys.RSA4096] = reflect.TypeOf(&rsa.PrivateKey{}).String()
}

// CryptoType returns the types of the expected crypto keys.
func CryptoType(kd keys.CryptoAlgo) (private, public string) {
	return privateKeyTypes[kd], publicKeyTypes[kd]
}

// SSHPublicKeyType returns the expected type of the ssh public keys used
// for the specified key type.
func SSHPublicKeyType(typ keys.CryptoAlgo) string {
	switch typ {
	case keys.ECDSA256, keys.ECDSA384, keys.ECDSA521:
		return "*ssh.ecdsaPublicKey"
	case keys.ED25519:
		return "ssh.ed25519PublicKey"
	case keys.RSA2048, keys.RSA4096:
		return "*ssh.rsaPublicKey"
	}
	return "unknown"
}
