// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package internal

import (
	"sort"
	"strings"

	"v.io/x/ref/lib/security"
)

var keyTypeMap = map[string]security.KeyType{
	"ecdsa256": security.ECDSA256,
	"ecdsa384": security.ECDSA384,
	"ecdsa521": security.ECDSA521,
	"ed25519":  security.ED25519,
}

// IsSupportedKeyType returns true if the requested key type is supported
// by the principal command. Currently the supported types are:
//		ecdsa256
//		ecdsa384
//		ecdsa521
// 		ed25519
func IsSupportedKeyType(keyType string) (security.KeyType, bool) {
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
