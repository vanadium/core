// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sectestdata

import (
	"testing"
)

func TestSSHKeys(t *testing.T) {
	for _, kt := range SupportedKeyAlgos {
		for _, set := range []SSHKeySetID{
			SSHKeyAgentHosted,
			SSHKeyPublic} {
			publicKey := SSHPublicKeyBytes(kt, set)
			if len(publicKey) == 0 {
				t.Errorf("empty file for %v %v", kt, set)
			}
		}
		for _, set := range []SSHKeySetID{
			SSHKeyPrivate,
			// TODO(cnicolaou): enabled this once the handling of ssh public
			//                  keys is cleaned up.
			// SSHKeyAgentHosted,
		} {
			signer := SSHKeySigner(kt, set)
			if signer == nil {
				t.Errorf("no signer for %v %v", kt, set)
			}
		}
	}
}

func TestV23Keys(t *testing.T) {
	for _, kt := range SupportedKeyAlgos {
		for _, set := range []V23KeySetID{
			V23keySetA, V23KeySetB,
		} {
			privateKey := V23PrivateKey(kt, set)
			if privateKey == nil {
				t.Errorf("no private key file for %v %v", kt, set)
			}
			signer := V23Signer(kt, set)
			if signer == nil {
				t.Errorf("no signer for %v %v", kt, set)
			}
		}
	}
}

func TestSSLData(t *testing.T) {
	keys, certs, opts := VanadiumSSLData()
	for _, kt := range SupportedKeyAlgos {
		host := kt.String()
		if _, ok := keys[host]; !ok {
			t.Errorf("missing private key for %v", host)
		}
		if _, ok := certs[host]; !ok {
			t.Errorf("missing cert for %v", host)
		}
	}

	for _, kt := range SupportedKeyAlgos {
		if pk := X509PublicKey(kt); pk == nil {
			t.Errorf("missing public key for %v", kt)
		}
		if pk := X509PrivateKey(kt); pk == nil {
			t.Errorf("missing privae key for %v", kt)
		}
		if signer := X509Signer(kt); signer == nil {
			t.Errorf("missing signer for %v", kt)
		}
	}

	for _, cert := range certs {
		chain, err := cert.Verify(opts)
		if err != nil {
			t.Errorf("failed to verify cert: %v: %v", cert.DNSNames, err)
		}
		if len(chain) == 0 {
			t.Errorf("missing cert chain for cert: %v: %v", cert.DNSNames, err)
		}
	}
}
