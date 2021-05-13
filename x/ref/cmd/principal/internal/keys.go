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

func IsSupportedKeyType(keyType string) (security.KeyType, bool) {
	k, ok := keyTypeMap[strings.ToLower(keyType)]
	return k, ok
}

func SupportedKeyTypes() []string {
	s := []string{}
	for k := range keyTypeMap {
		s = append(s, k)
	}
	sort.Strings(s)
	return s
}
