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

	// When set and non-empty, EnvCredentials is hosted on a read-only
	// filesystem.
	EnvCredentialsReadonlyFileSystem = "V23_CREDENTIALS_READONLY_FILESYSTEM"

	// EnvCredentialsNoLockDeprecated and V23_CREDENTIALS_NO_LOCK will
	// be removed in a subsequent release. This is essentially an alias
	// for V23_CREDENTIALS_READONLY_FILESYSTEM which should be used
	// instead.
	EnvCredentialsNoLockDeprecated = "V23_CREDENTIALS_NO_LOCK"

	// EnvCredentialsReloadInterval is the name of the environment variable
	// that specifies the interval between credentials reloads.
	EnvCredentialsReloadInterval = "V23_CREDENTIALS_RELOAD_INTERVAL"

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
	// Deprecated: use V23_VIRTUALIZATION_PROVIDER instead.
	EnvExpectGoogleComputeEngine = "V23_EXPECT_GOOGLE_COMPUTE_ENGINE"

	// EnvVirtualizationProvider is the name of the environment variable
	// naming the virtualization/cloud provider that the process is
	// expected to be run under. It replaces EnvExpectGoogleComputeEngine.
	EnvVirtualizationProvider = "V23_VIRTUALIZATION_PROVIDER"

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
// Runtime to initialize the principal.
func EnvClearCredentials() error {
	for _, v := range []string{
		EnvCredentials,
		EnvCredentialsReadonlyFileSystem,
		EnvCredentialsNoLockDeprecated,
	} {
		if err := os.Unsetenv(v); err != nil {
			return err
		}
	}
	return nil
}

// ReadonlyCredentialsDir returns true if the credentials directory is to be
// treated as readonly, for example, because it exists on a read-only
// filesystem. Any attempt to write to this directory should result in an
// error. It also returns the reason that the directory is considered readonly,
// for now, the name of the environment variable that is used to indicate this.
func ReadonlyCredentialsDir() (reason string, readonly bool) {
	switch {
	case len(os.Getenv(EnvCredentialsReadonlyFileSystem)) > 0:
		return EnvCredentialsReadonlyFileSystem, true
	case len(os.Getenv(EnvCredentialsNoLockDeprecated)) > 0:
		return EnvCredentialsNoLockDeprecated, true
	}
	return "", false
}
