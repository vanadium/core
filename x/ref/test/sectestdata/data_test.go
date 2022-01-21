// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package sectestdata_test

import (
	"testing"

	libsec "v.io/x/ref/lib/security"
	"v.io/x/ref/test/sectestdata"
)

var supportedKeyTypes = []libsec.KeyType{
	libsec.ECDSA256, libsec.ECDSA384, libsec.ECDSA521,
	libsec.ED25519,
	libsec.RSA2048, libsec.RSA4096,
}

func TestSSHKeys(t *testing.T) {
	for _, kt := range supportedKeyTypes {
		for _, set := range []sectestdata.SSHKeySetID{
			sectestdata.SSHKeyAgentHosted,
			sectestdata.SSHKeySetRFC4716} {
			publicKey := sectestdata.SSHPublicKey(kt, set)
			if len(publicKey) == 0 {
				t.Errorf("empty file for %v %v", kt, set)
			}
		}
		for _, set := range []sectestdata.SSHKeySetID{
			sectestdata.SSHkeySetNative,
			sectestdata.SSHKeyAgentHosted,
		} {
			signer := sectestdata.SSHKeySigner(kt, set)
			if signer == nil {
				t.Errorf("no signer for %v %v", kt, set)
			}
		}

	}

}
