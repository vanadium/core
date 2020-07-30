// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"os"
	"sync"

	"v.io/v23/rpc"
	"v.io/x/ref"
)

var (
	// All GUARDED_BY defaultMu
	defaultNamespaceRoots     []string
	defaultCredentialsDir     string
	defaultI18nCatalogue      string
	defaultProtocol           string
	defaultHostPort           string
	defaultProxy              string
	defaultProxyPolicy        rpc.ProxyPolicy
	defaultProxyLimit         int
	defaultPermissionsLiteral string
	defaultPermissions        map[string]string
	defaultVirtualized        VirtualizedFlagDefaults
	defaultMu                 sync.RWMutex
)

// SetDefaultProtocol sets the default protocol used when --v23.tcp.protocol is
// not provided. It must be called before flags are parsed for it to take effect.
func SetDefaultProtocol(protocol string) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultProtocol = protocol
}

// DefaultProtocol returns the current default protocol.
func DefaultProtocol() string {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultProtocol
}

// SetDefaultHostPort sets the default host and port used when --v23.tcp.address
// is not provided. It must be called before flags are parsed for it to take effect.
func SetDefaultHostPort(s string) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultHostPort = s
}

// DefaultHostPort returns the current default host port.
func DefaultHostPort() string {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultHostPort
}

// SetDefaultProxy sets the default proxy used when --v23.proxy
// is not provided. It must be called before flags are parsed for it to take effect.
func SetDefaultProxy(s string) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultProxy = s
}

// DefaultProxy returns the current default proxy.
func DefaultProxy() string {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultProxy
}

// SetDefaultProxyPolicy sets the default proxy used when --v23.proxy.policy
// is not provided. It must be called before flags are parsed for it to take effect.
func SetDefaultProxyPolicy(p rpc.ProxyPolicy) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultProxyPolicy = p
}

// DefaultProxyPolicy returns the current default proxy policy.
func DefaultProxyPolicy() rpc.ProxyPolicy {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultProxyPolicy
}

// SetDefaultProxyLimit sets the default proxy used when --v23.proxy.limit
// is not provided. It must be called before flags are parsed for it to take effect.
func SetDefaultProxyLimit(l int) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultProxyLimit = l
}

// DefaultProxyLimit returns the current default proxy limit.
func DefaultProxyLimit() int {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultProxyLimit
}

// SetDefaultNamespaceRoots sets the default value for --v23.namespace.root
func SetDefaultNamespaceRoots(roots ...string) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultNamespaceRoots = roots
}

// DefaultNamespaceRootsNoEnv returns the current default value of
// -v23.namespace.root ignoring V23_NAMESPACE_ROOT0...
func DefaultNamespaceRootsNoEnv() []string {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultNamespaceRoots
}

// DefaultNamespaceRoots returns the current default value of
// -v23.namespace.root taking the environment variables
// V23_NAMESPACE_ROOT0... into account.
func DefaultNamespaceRoots() []string {
	if _, l := ref.EnvNamespaceRoots(); len(l) > 0 {
		return l
	}
	return DefaultNamespaceRootsNoEnv()
}

// SetDefaultCredentialsDir sets the default value for --v23.credentials.
// It must be called before flags are parsed for it to take effect.
func SetDefaultCredentialsDir(credentialsDir string) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultCredentialsDir = credentialsDir
}

// DefaultCredentialsDirNoEnv returns the current default for --v23.credentials
// ignoring V23_CREDENTIALS
func DefaultCredentialsDirNoEnv() string {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultCredentialsDir
}

// DefaultCredentialsDir returns the current default for --v23.credentials
// taking V23_CREDENTIALS into account
func DefaultCredentialsDir() string {
	if e := os.Getenv(ref.EnvCredentials); len(e) > 0 {
		return e
	}
	return DefaultCredentialsDirNoEnv()
}

// SetDefaultI18nCatalogue sets the default value for --v23.i18n-catalogue.
// It must be called before flags are parsed for it to take effect.
func SetDefaultI18nCatalogue(i18nCatalogue string) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultI18nCatalogue = i18nCatalogue
}

// DefaultI18nCatalogueNoEnv returns the current default for
// --v23.i18n-catalogue ignoring V23_I18N_CATALOGUE.
func DefaultI18nCatalogueNoEnv() string {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultI18nCatalogue
}

// DefaultI18nCatalogue returns the current default for --v23.i18n-catalogue.
// taking V23_V23_I18N_CATALOGUE into account.
func DefaultI18nCatalogue() string {
	if e := os.Getenv(ref.EnvI18nCatalogueFiles); len(e) > 0 {
		return e
	}
	return DefaultCredentialsDirNoEnv()
}

// SetDefaultPermissionsLiteral sets the default value for
// --v23.permissions.literal.
func SetDefaultPermissionsLiteral(literal string) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultPermissionsLiteral = literal
}

// DefaultPermissionsLiteral returns the current default value for
// --v23.permissions.literal.
func DefaultPermissionsLiteral() string {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultPermissionsLiteral
}

// SetDefaultPermissions adds a name, file pair to the default value
// for --v23.permissions.file.
func SetDefaultPermissions(name, file string) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultPermissions[name] = file
}

// DefaultPermissions returns the current default values for
// --v23.permissions.file as a map.
func DefaultPermissions() map[string]string {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultPermissions
}

// SetDefaultVirtualizedFlagValues sets the default values to use for
// the Virtualized flags group.
func SetDefaultVirtualizedFlagValues(values VirtualizedFlagDefaults) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultVirtualized = values
}

// DefaultVirtualizedFlagValues returns the default values to use for
// the Virtualized flags group.
func DefaultVirtualizedFlagValues() VirtualizedFlagDefaults {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultVirtualized
}
