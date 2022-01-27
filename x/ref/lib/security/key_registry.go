// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/lib/security/keys/indirectkeyfiles"
	"v.io/x/ref/lib/security/keys/sshkeys"
)

var keyRegistry *keys.Registrar

func KeyRegistrar() *keys.Registrar {
	return keyRegistry
}

func init() {
	keyRegistry = keys.NewRegistrar()
	keys.MustRegisterCommon(keyRegistry)
	indirectkeyfiles.MustRegister(keyRegistry)
	sshkeys.MustRegister(keyRegistry)
}