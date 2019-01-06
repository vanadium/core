// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import "v.io/x/ref/lib/flags/sitedefaults"

var defaultValues = []map[string]interface{}{}

func mergeDefaultValues() map[string]interface{} {
	merged := map[string]interface{}{}
	for _, v := range defaultValues {
		for name, val := range v {
			merged[name] = val
		}
	}
	return merged
}

func registerDefaults(values map[string]interface{}) {
	defaultValues = append(defaultValues, values)
}

// This init function will be the first called.
func init() {
	registerDefaults(map[string]interface{}{
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
	})
	registerDefaults(sitedefaults.Defaults)
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
