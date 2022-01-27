// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sshkeys

import (
	"encoding/pem"
	"reflect"
	"testing"

	"v.io/x/ref/test/sectestdata"
)

func TestSSHKeys(t *testing.T) {
	for _, kt := range sectestdata.SupportedKeyAlgos {
		public := sectestdata.SSHPublicKeyBytes(kt, sectestdata.SSHKeyPublic)
		key, err := parseAuthorizedKey(public)
		if err != nil {
			t.Fatal(err)
		}

		api := &sshkeyAPI{}

		ck, err := api.PublicKey(key)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := reflect.TypeOf(key).String(), sectestdata.SSHPublicKeyType(kt); got != want {
			t.Errorf("got %v, does not match %v", got, want)
		}

		if got, want := reflect.TypeOf(ck).String(), sectestdata.CryptoSignerType(kt); got != want {
			t.Errorf("got %v, does not match %v", got, want)
		}

		if got, want := reflect.TypeOf(key).String(), sectestdata.SSHPublicKeyType(kt); got != want {
			t.Errorf("got %v, does not match %v", got, want)
		}
		block, _ := pem.Decode(sectestdata.SSHPrivateKeyBytes(kt,
			sectestdata.SSHKeyPrivate))

		privateKeyType, _ := sectestdata.CryptoType(kt)

		key, err = parseOpensshPrivateKey(block)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := reflect.TypeOf(key).String(), privateKeyType; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

		block, _ = pem.Decode(sectestdata.SSHPrivateKeyBytes(kt,
			sectestdata.SSHKeyEncrypted))
		_, key, err = decryptOpensshPrivateKey(block, sectestdata.Password())
		if err != nil {
			t.Fatal(err)
		}
		if got, want := reflect.TypeOf(key).String(), privateKeyType; got != want {
			t.Errorf("got %v, want %v", got, want)
		}

	}
}
