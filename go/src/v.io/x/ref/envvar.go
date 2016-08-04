// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package ref defines constants used through the Vanadium reference
// implementation, which is implemented in its subdirectories.
package ref

import (
	"os"
	"strings"
)

const (
	// EnvCredentials is the name of the environment variable pointing to a
	// directory containing all the credentials of a principal (the blessing
	// store, the blessing roots, possibly the private key etc.).
	//
	// Typically only one of EnvCredentials or EnvAgentPath will be set in a
	// process. If both are set, then EnvCredentials takes preference.
	//
	// See v.io/x/ref/lib/security.CreatePersistentPrincipal.
	EnvCredentials = "V23_CREDENTIALS"

	// EnvCredentialsNoAgent is the name of the environment variable
	// indicating whether an agent should be started to serve credentials to
	// be loaded from a directory.  If set to anything but empty string, no
	// attempt is made to load an agent.
	//
	// Typically used in tests or on platforms where launching agents is not
	// desirable.
	//
	// See v.io/x/ref/security/agent/agentlib.LoadPrincipal.
	EnvCredentialsNoAgent = "V23_CREDENTIALS_NO_AGENT"

	// EnvAgentPath is the name of the environment variable pointing to a
	// socket of the agent process containing all the credentials for a
	// principal (the blessing store, the blessing roots, possibly the
	// private key etc.).
	//
	// Typically only one of EnvCredentials or EnvAgentPath will be set in a
	// process. If both are set, then EnvCredentials takes preference.
	EnvAgentPath = "V23_AGENT_PATH"

	// EnvNamespacePrefix is the prefix of all environment variables that
	// define a namespace root.
	EnvNamespacePrefix = "V23_NAMESPACE"

	// EnvI18nCatalogueFiles is the name of the environment variable
	// pointing to a comma-separated list of i18n catalogue files to be
	// loaded at startup.
	EnvI18nCatalogueFiles = "V23_I18N_CATALOGUE"

	// EnvOAuthIdentityProvider is the name of the environment variable
	// pointing to the url of the OAuth identity provider used by the
	// principal seekblessings command.
	EnvOAuthIdentityProvider = "V23_OAUTH_IDENTITY_PROVIDER"

	// When EnvExpectGoogleComputeEngine is set and non-empty, the runtime
	// initialization will fail if the GCE metadata is inaccessible.
	EnvExpectGoogleComputeEngine = "V23_EXPECT_GOOGLE_COMPUTE_ENGINE"

	// A comma-separated list of path names to export disk stats for.
	EnvSysStatsDiskPaths = "V23_SYS_STATS_DISK_PATHS"

	// When set and non-empty, the namespace client will not use caching.
	EnvDisableNamespaceCache = "V23_DISABLE_NS_CACHE"
)

// EnvNamespaceRoots returns the set of namespace roots to be used by the
// process, as specified by environment variables.
//
// It returns both a map of environment variable name to value and the list of
// values.
func EnvNamespaceRoots() (map[string]string, []string) {
	m := make(map[string]string)
	var l []string
	for _, ev := range os.Environ() {
		p := strings.SplitN(ev, "=", 2)
		if len(p) != 2 {
			continue
		}
		k, v := p[0], p[1]
		if strings.HasPrefix(k, EnvNamespacePrefix) && len(v) > 0 {
			l = append(l, v)
			m[k] = v
		}
	}
	return m, l
}

// EnvClearCredentials unsets all environment variables that are used by the
// Runtime to intialize the principal.
func EnvClearCredentials() error {
	for _, v := range []string{
		EnvCredentials,
	} {
		if err := os.Unsetenv(v); err != nil {
			return err
		}
	}
	return nil
}
