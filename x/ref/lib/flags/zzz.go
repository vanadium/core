// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

// This init function is placed in zzz.go to ensure, at least with the
// standard go build system, that is is invoked after all other init functions.
// This reliese on the behaviour encoruaged by the go language spec:
// "To ensure reproducible initialization behavior, build systems are encouraged
// to present multiple files belonging to the same package in lexical file name
// order to a compiler."
func init() {
	merged := mergeDefaultValues()
	defaultNamespaceRoots = merged["namespaceRoots"].([]string)
	defaultCredentialsDir = merged["credentialsDir"].(string)
	defaultI18nCatalogue = merged["i18nCatalogue"].(string)
	defaultProtocol = merged["protocol"].(string)
	defaultHostPort = merged["hostPort"].(string)
	defaultProxy = merged["proxy"].(string)
	defaultPermissionsLiteral = merged["permissionsLiteral"].(string)
	defaultPermissions = merged["permissions"].(map[string]string)
}
