// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

// This file is intended to be overriden by local installations to set
// their site-specific defaults.

var defaultValues = map[string]interface{}{
	"namespaceRoots": []string{
		"/(dev.v.io:r:vprod:service:mounttabled)@ns.dev.v.io:8101",
	},
	"credentialsDir":     "",
	"i18nCatalogue":      "",
	"protocol":           "wsh",
	"hostPort":           ":0",
	"proxy":              "",
	"permissionsLiteral": "",
	"permissions":        map[string]string{},
}
