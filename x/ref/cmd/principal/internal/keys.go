// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"sort"
	"strings"

	"v.io/x/ref/lib/security/keys"
)

var keyTypeMap = map[string]keys.CryptoAlgo{
	"ecdsa256": keys.ECDSA256,
	"ecdsa384": keys.ECDSA384,
	"ecdsa521": keys.ECDSA521,
	"ed25519":  keys.ED25519,
	"rsa2048":  keys.RSA2048,
	"rsa4096":  keys.RSA4096,
}

// IsSupportedKeyType returns true if the requested key type is supported
// by the principal command. Currently the supported types are:
//		ecdsa256
//		ecdsa384
//		ecdsa521
// 		ed25519
//		rsa2048
//		rsa4096
func IsSupportedKeyType(keyType string) (keys.CryptoAlgo, bool) {
	k, ok := keyTypeMap[strings.ToLower(keyType)]
	return k, ok
}

// SupportedKeyTypes returns the currently supported key types.
func SupportedKeyTypes() []string {
	s := []string{}
	for k := range keyTypeMap {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}
