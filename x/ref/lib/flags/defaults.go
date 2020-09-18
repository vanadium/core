// Copyright 2018 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"fmt"
	"os"
	"reflect"
	"sync"

	"v.io/v23/rpc"
	"v.io/x/ref"
)

var (
	// All GUARDED_BY defaultMu
	// Initial values that override these are set by the init function in
	// sitedefaults.go. Any defaults set here will be overridden by those
	// set in sitedefaults.go. Note that the sitedefaults package is
	// is a submodule that can be maintained separately by each site via a
	// 'replace' statement in go.mod to provide site specific defaults.
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

	// defaultExplicitlySet is used to track which of the above have been
	// changed. See markAsNewDefault and hasNewDefault below.
	defaultExplicitlySet = map[interface{}]bool{}
	defaultMu            sync.RWMutex
)

func markAsNewDefault(ptr interface{}) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	markAsNewDefaultLocked(ptr)
}

func markAsNewDefaultLocked(ptr interface{}) {
	if reflect.TypeOf(ptr).Kind() != reflect.Ptr {
		panic(fmt.Sprintf("%T is not a pointer", ptr))
	}
	defaultExplicitlySet[ptr] = true
}

func hasNewDefault(ptr interface{}) bool {
	if reflect.TypeOf(ptr).Kind() != reflect.Ptr {
		panic(fmt.Sprintf("%T is not a pointer", ptr))
	}
	defaultMu.Lock()
	defer defaultMu.Unlock()
	return defaultExplicitlySet[ptr]
}

func refreshRuntimeFlagsFromDefaults(v *RuntimeFlags) error {
	if v.NamespaceRoots.isDefault {
		// Allow for reading the defaults from the environment.
		v.NamespaceRoots.Roots = DefaultNamespaceRoots()
	}
	if hasNewDefault(&defaultCredentialsDir) {
		v.Credentials = defaultCredentialsDir
	}
	if hasNewDefault(&defaultI18nCatalogue) {
		v.I18nCatalogue = defaultI18nCatalogue
	}
	return nil
}

func refreshListenFlagsFromDefaults(v *ListenFlags) error {
	if !v.Protocol.isSet {
		if err := v.Protocol.validator.Set(defaultProtocol); err != nil {
			return err
		}
	}
	if !v.Addresses.isSet {
		if err := v.Addresses.validator.Set(defaultHostPort); err != nil {
			return err
		}
	}
	if hasNewDefault(&defaultProxy) {
		v.Proxy = defaultProxy
	}
	if hasNewDefault(&defaultProxyPolicy) {
		v.ProxyPolicy = ProxyPolicyFlag(defaultProxyPolicy)
	}
	if hasNewDefault(&defaultProxyLimit) {
		v.ProxyLimit = defaultProxyLimit
	}
	return nil
}

func initVirtualizedFlagsFromDefaults(v *VirtualizedFlags) error {
	v.Dockerized = defaultVirtualized.Dockerized
	v.PublicDNSName.Set(defaultVirtualized.PublicDNSName)
	if err := v.VirtualizationProvider.Set(defaultVirtualized.VirtualizationProvider); err != nil {
		return err
	}
	if err := v.PublicProtocol.Set(defaultVirtualized.PublicProtocol); err != nil {
		return err
	}
	return v.PublicAddress.Set(defaultVirtualized.PublicAddress)
}

func refreshDefaults(f *Flags) error {
	for _, g := range f.groups {
		var err error
		switch v := g.(type) {
		case *RuntimeFlags:
			err = refreshRuntimeFlagsFromDefaults(v)
		case *ListenFlags:
			err = refreshListenFlagsFromDefaults(v)
		case *VirtualizedFlags:
			if hasNewDefault(&defaultVirtualized) {
				err = initVirtualizedFlagsFromDefaults(v)
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// SetDefaultProtocol sets the default protocol used when --v23.tcp.protocol is
// not provided. It must be called before flags are parsed for it to take effect.
func SetDefaultProtocol(protocol string) {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	defaultProtocol = protocol
	markAsNewDefaultLocked(&defaultProtocol)
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
	markAsNewDefaultLocked(&defaultHostPort)
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
	markAsNewDefaultLocked(&defaultProxy)
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
	markAsNewDefaultLocked(&defaultProxyPolicy)
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
	markAsNewDefaultLocked(&defaultProxyLimit)
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
	markAsNewDefaultLocked(&defaultNamespaceRoots)
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
	markAsNewDefaultLocked(&defaultCredentialsDir)
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
	markAsNewDefaultLocked(&defaultI18nCatalogue)
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
	markAsNewDefaultLocked(&defaultPermissionsLiteral)
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
	markAsNewDefaultLocked(&defaultVirtualized)
}

// DefaultVirtualizedFlagValues returns the default values to use for
// the Virtualized flags group taking V23_VIRTUALIZATION_PROVIDER into account.
// In addition, if V23_EXPECT_GOOGLE_COMPUTE_ENGINE is set then GCP is assumed
// to be the virtualization provider. V23_EXPECT_GOOGLE_COMPUTE_ENGINE will
// be removed in the near future (as of 7/30/20). V23_VIRTUALIZATION_PROVIDER will
// override the effect of V23_EXPECT_GOOGLE_COMPUTE_ENGINE.
func DefaultVirtualizedFlagValues() VirtualizedFlagDefaults {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	def := defaultVirtualized
	if p := os.Getenv(ref.EnvExpectGoogleComputeEngine); len(p) > 0 {
		def.VirtualizationProvider = string(GCP)
	}
	if p := os.Getenv(ref.EnvVirtualizationProvider); len(p) > 0 {
		def.VirtualizationProvider = p
	}
	return def
}
