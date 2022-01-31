// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/youmark/pkcs8"
	"v.io/x/ref/lib/security"
	"v.io/x/ref/test/sectestdata"
)

func filenames(paths []string) []string {
	r := []string{}
	for _, p := range paths {
		r = append(r, filepath.Base(p))
	}
	return r
}

func TestConversionFromLegacyFormat(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	sectestdata.V23CopyLegacyPrincipals(dir)

	expectedFilenames := []string{
		"dir.lock", "privatekey.pem", "publickey.pem",
	}
	for _, kt := range sectestdata.SupportedKeyAlgos {
		for _, tc := range []struct {
			prefix     string
			passphrase []byte
		}{
			{"plain", nil},
			{"encrypted", sectestdata.Password()},
		} {

			principal := fmt.Sprintf("v23-%s-%s-principal", tc.prefix, kt)
			pdir := filepath.Join(dir, principal)
			if err := security.ConvertPrivateKeyForPrincipal(ctx, pdir, tc.passphrase); err != nil {
				t.Fatalf("%v: %v", pdir, err)
			}
			matches, err := filepath.Glob(filepath.Join(pdir, "*"))
			if err != nil {
				t.Fatalf("%v: %v", pdir, err)
			}
			if got, want := filenames(matches), expectedFilenames; !reflect.DeepEqual(got, want) {
				t.Fatalf("%v: got %v, want %v", pdir, got, want)
			}
			newKeyBytes, err := os.ReadFile(filepath.Join(pdir, "privatekey.pem"))
			if err != nil {
				t.Fatalf("%v: %v", pdir, err)
			}
			block, _ := pem.Decode(newKeyBytes)
			if block.Type != "PRIVATE KEY" && block.Type != "ENCRYPTED PRIVATE KEY" {
				t.Fatalf("%v: wrong PEM type: %v", pdir, block.Type)

			}
			key, err := pkcs8.ParsePKCS8PrivateKey(block.Bytes, tc.passphrase)
			if err != nil {
				t.Fatalf("%v: %v", pdir, err)
			}
			switch key.(type) {
			case *ecdsa.PrivateKey:
			case ed25519.PrivateKey:
			case *rsa.PrivateKey:
			default:
				t.Fatalf("%v: wrong key type %T", pdir, key)
			}
		}
	}
}
