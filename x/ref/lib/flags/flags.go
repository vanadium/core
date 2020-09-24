// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"flag"
	"fmt"
	"strings"

	"v.io/x/lib/cmd/flagvar"
	"v.io/x/ref"
)

// FlagGroup is the type for identifying groups of related flags.
type FlagGroup int

const (
	// Runtime identifies the flags and associated environment variables
	// used by the Vanadium process runtime. Namely:
	// --v23.namespace.root (which may be repeated to supply multiple values)
	// --v23.credentials
	// --v23.i18n-catalogue
	// --v23.vtrace.sample-rate
	// --v23.vtrace.dump-on-shutdown
	// --v23.vtrace.cache-size
	// --v23.vtrace.collect-regexp
	Runtime FlagGroup = iota
	// Listen identifies the flags typically required to configure
	// rpc.ListenSpec. Namely:
	// --v23.tcp.protocol
	// --v23.tcp.address
	// --v23.proxy
	// --v23.proxy.policy
	// --v23.proxy.limit
	Listen
	// Permissions identifies the flags typically required to configure
	// authorization.
	// --v23.permissions.file (which may be repeated to supply multiple values)
	// Permissions files are named - i.e. --v23.permissions.file=<name>:<file>
	// with the name "runtime" reserved for use by the runtime. "file" is
	// a JSON-encoded representation of the Permissions type defined in the
	// VDL package v.io/v23/security/access
	// --v23.permissions.literal (which may be repeated to supply multiple values
	// which will be concatenated)
	// This flag allows permissions to specified directly on the command line.
	Permissions
	// Virtualized identifies the flags typically required to configure a server
	// running in some form of virtualized or isolated environment such
	// as AWS, GCE or within a docker container.
	// v23.virtualized.docker
	// v23.virtualized.provider
	// v23.virtualized.discover-public-address
	// v23.virtualized.tcp.public-protocol
	// v23.virtualized.tcp.public-address
	// v23.virtualized.dns.public-name
	Virtualized
)

// Flags represents the set of flag groups created by a call to
// CreateAndRegister.
type Flags struct {
	FlagSet *flag.FlagSet
	groups  map[FlagGroup]interface{}
}

// RuntimeFlags contains the values of the Runtime flag group.
type RuntimeFlags struct {
	// NamespaceRoots may be initialized by ref.EnvNamespacePrefix* enivornment
	// variables as well as --v23.namespace.root. The command line
	// will override the environment.
	NamespaceRoots NamespaceRootFlag `cmdline:"v23.namespace.root,,local namespace root; can be repeated to provided multiple roots"`

	// Credentials may be initialized by the ref.EnvCredentials
	// environment variable. The command line will override the environment.
	// TODO(cnicolaou): provide flag.Value impl
	Credentials string `cmdline:"v23.credentials,,directory to use for storing security credentials"`

	// I18nCatalogue may be initialized by the ref.EnvI18nCatalogueFiles
	// environment variable.  The command line will override the
	// environment.
	I18nCatalogue string `cmdline:"v23.i18n-catalogue,,'18n catalogue files to load, comma separated'"`

	// VtraceFlags control various aspects of Vtrace.
	VtraceFlags
}

// VtraceFlags represents the flags used to configure rpc tracing.
type VtraceFlags struct {
	// VtraceSampleRate is the rate (from 0.0 - 1.0) at which
	// vtrace traces started by this process are sampled for collection.
	SampleRate float64 `cmdline:"v23.vtrace.sample-rate,,Rate (from 0.0 to 1.0) to sample vtrace traces"`

	// VtraceDumpOnShutdown tells the runtime to dump all stored traces
	// to Stderr at shutdown if true.
	DumpOnShutdown bool `cmdline:"v23.vtrace.dump-on-shutdown,true,'If true, dump all stored traces on runtime shutdown'"`

	// VtraceCacheSize is the number of traces to cache in memory.
	// TODO(mattr): Traces can be of widely varying size, we should have
	// some better measurement then just number of traces.
	CacheSize int `cmdline:"v23.vtrace.cache-size,1024,The number of vtrace traces to store in memory"`

	// LogLevel is the level of vlogs that should be collected as part of
	// the trace
	LogLevel int `cmdline:"v23.vtrace.v,,The verbosity level of the log messages to be captured in traces"`

	// SpanRegexp matches a regular expression against span names and
	// annotations and forces any trace matching trace to be collected.
	CollectRegexp string `cmdline:"v23.vtrace.collect-regexp,,Spans and annotations that match this regular expression will trigger trace collection"`
}

// CreateAndRegisterRuntimeFlags creates and registers a RuntimeFlags
// with the supplied flag.FlagSet.
func CreateAndRegisterRuntimeFlags(fs *flag.FlagSet) (*RuntimeFlags, error) {
	rf, err := NewRuntimeFlags()
	if err != nil {
		return nil, err
	}
	err = RegisterRuntimeFlags(fs, rf)
	if err != nil {
		return nil, err
	}
	return rf, nil
}

// NewRuntimeFlags creates a new RuntimeFlags with appropriate defaults.
func NewRuntimeFlags() (*RuntimeFlags, error) {
	rf := &RuntimeFlags{}
	_, roots := ref.EnvNamespaceRoots()
	if len(roots) == 0 {
		rf.NamespaceRoots.Roots = DefaultNamespaceRoots()
		rf.NamespaceRoots.isDefault = true
	} else {
		rf.NamespaceRoots.Roots = roots
	}
	rf.Credentials = DefaultCredentialsDir()
	rf.I18nCatalogue = DefaultI18nCatalogue()
	rf.VtraceFlags = VtraceFlags{
		SampleRate:     0.0,
		DumpOnShutdown: true,
		CacheSize:      1024,
		LogLevel:       0,
		CollectRegexp:  "",
	}
	return rf, nil
}

// RegisterRuntimeFlags registers the supplied RuntimeFlags variable with
// the supplied FlagSet.
func RegisterRuntimeFlags(fs *flag.FlagSet, f *RuntimeFlags) error {
	err := flagvar.RegisterFlagsInStruct(fs, "cmdline", f,
		map[string]interface{}{
			"v23.credentials":    DefaultCredentialsDir(),
			"v23.i18n-catalogue": DefaultI18nCatalogue(),
		},
		map[string]string{
			"v23.namespace.root": "[" + strings.Join(DefaultNamespaceRoots(), ",") + "]",
			"v23.credentials":    "",
			"v23.i18n-catalogue": "",
		},
	)
	return err
}

// CreateAndRegisterPermissionsFlags creates and registers a PermissionsFlags
// with the supplied FlagSet.
func CreateAndRegisterPermissionsFlags(fs *flag.FlagSet) (*PermissionsFlags, error) {
	pf, err := NewPermissionsFlags()
	if err != nil {
		return nil, err
	}
	err = RegisterPermissionsFlags(fs, pf)
	if err != nil {
		return nil, err
	}
	return pf, nil
}

// NewPermissionsFlags creates a PermissionsFlags with appropriate defaults.
func NewPermissionsFlags() (*PermissionsFlags, error) {
	return &PermissionsFlags{
		Files:   PermissionsFlag{files: DefaultPermissions()},
		Literal: PermissionsLiteralFlag{permissions: DefaultPermissionsLiteral()},
	}, nil
}

// RegisterPermissionsFlags registers the supplied PermissionsFlags with
// the supplied FlagSet.
func RegisterPermissionsFlags(fs *flag.FlagSet, f *PermissionsFlags) error {
	return flagvar.RegisterFlagsInStruct(fs, "cmdline", f,
		nil,
		map[string]string{
			"v23.permissions.file": "",
		})
}

// CreateAndRegisterListenFlags creates and registers the ListenFlags
// group with the supplied flag.FlagSet.
func CreateAndRegisterListenFlags(fs *flag.FlagSet) (*ListenFlags, error) {
	lf, err := NewListenFlags()
	if err != nil {
		return nil, err
	}
	if err := RegisterListenFlags(fs, lf); err != nil {
		return nil, err
	}
	return lf, nil
}

// NewListenFlags creates a new ListenFlags with appropriate defaults.
func NewListenFlags() (*ListenFlags, error) {
	lf := &ListenFlags{}
	var ipHostPortFlag IPHostPortFlag
	if err := ipHostPortFlag.Set(DefaultHostPort()); err != nil {
		return nil, err
	}
	var protocolFlag TCPProtocolFlag
	if err := protocolFlag.Set(DefaultProtocol()); err != nil {
		return nil, err
	}
	var proxyFlag ProxyPolicyFlag
	if err := proxyFlag.Set(DefaultProxyPolicy().String()); err != nil {
		panic(err)
	}
	lf.Protocol = tcpProtocolFlagVar{validator: protocolFlag}
	lf.Addresses = ipHostPortFlagVar{validator: ipHostPortFlag}
	lf.Proxy = DefaultProxy()
	lf.ProxyPolicy = proxyFlag
	lf.ProxyLimit = DefaultProxyLimit()
	return lf, nil
}

// RegisterListenFlags registers the supplied ListenFlags variable with
// the supplied FlagSet.
func RegisterListenFlags(fs *flag.FlagSet, f *ListenFlags) error {
	f.Addresses.flags = f
	err := flagvar.RegisterFlagsInStruct(fs, "cmdline", f,
		map[string]interface{}{
			"v23.proxy":        DefaultProxy(),
			"v23.proxy.policy": DefaultProxyPolicy(),
			"v23.proxy.limit":  DefaultProxyLimit(),
		}, map[string]string{
			"v23.proxy": "",
		},
	)
	return err
}

// CreateAndRegisterVirtualizedFlags creates and registers the VirtualizedFlags
// group with the supplied flag.FlagSet.
func CreateAndRegisterVirtualizedFlags(fs *flag.FlagSet) (*VirtualizedFlags, error) {
	lf, err := NewVirtualizedFlags()
	if err != nil {
		return nil, err
	}
	err = RegisterVirtualizedFlags(fs, lf)
	if err != nil {
		return nil, err
	}
	return lf, nil
}

// NewVirtualizedFlags creates a new VirtualizedFlags with appropriate defaults.
func NewVirtualizedFlags() (*VirtualizedFlags, error) {
	vf := &VirtualizedFlags{}
	if err := initVirtualizedFlagsFromDefaults(vf); err != nil {
		return nil, err
	}
	return vf, nil
}

// RegisterVirtualizedFlags registers the supplied VirtualizedFlags variable with
// the supplied FlagSet.
func RegisterVirtualizedFlags(fs *flag.FlagSet, f *VirtualizedFlags) error {
	def := DefaultVirtualizedFlagValues()
	provider := &VirtualizationProviderFlag{}
	if err := provider.Set(def.VirtualizationProvider); err != nil {
		return err
	}
	address := &IPHostPortFlag{}
	if err := address.Set(def.PublicAddress); err != nil {
		return err
	}
	protocol := &TCPProtocolFlag{}
	if err := protocol.Set(def.PublicProtocol); err != nil {
		return err
	}
	dnsname := &HostPortFlag{}
	if err := dnsname.Set(def.PublicDNSName); err != nil {
		return err
	}
	return flagvar.RegisterFlagsInStruct(fs, "cmdline", f,
		map[string]interface{}{
			"v23.virtualized.docker":                      def.Dockerized,
			"v23.virtualized.provider":                    provider,
			"v23.virtualized.tcp.public-protocol":         protocol,
			"v23.virtualized.tcp.public-address":          address,
			"v23.virtualized.dns.public-name":             dnsname,
			"v23.virtualized.advertise-private-addresses": def.AdvertisePrivateAddresses,
		}, map[string]string{
			"v23.virtualized.docker":                      "",
			"v23.virtualized.provider":                    "",
			"v23.virtualized.tcp.public-protocol":         "",
			"v23.virtualized.tcp.public-address":          "",
			"v23.virtualized.dns.public-name":             "",
			"v23.virtualized.advertise-private-addresses": "",
		},
	)
}

// CreateAndRegister creates a new set of flag groups as specified by the
// supplied flag group parameters and registers them with the supplied
// flag.FlagSet.
func CreateAndRegister(fs *flag.FlagSet, groups ...FlagGroup) (*Flags, error) {
	if len(groups) == 0 {
		return nil, nil
	}
	f := &Flags{FlagSet: fs, groups: make(map[FlagGroup]interface{})}
	for _, g := range groups {
		var err error
		switch g {
		case Runtime:
			f.groups[Runtime], err = CreateAndRegisterRuntimeFlags(fs)
		case Listen:
			f.groups[Listen], err = CreateAndRegisterListenFlags(fs)
		case Permissions:
			f.groups[Permissions], err = CreateAndRegisterPermissionsFlags(fs)
		case Virtualized:
			f.groups[Virtualized], err = CreateAndRegisterVirtualizedFlags(fs)
		}
		if err != nil {
			return nil, err
		}
	}
	return f, nil
}

// RuntimeFlags returns the Runtime flag subset stored in its Flags
// instance.
func (f *Flags) RuntimeFlags() RuntimeFlags {
	if p := f.groups[Runtime]; p == nil {
		return RuntimeFlags{}
	}
	from := f.groups[Runtime].(*RuntimeFlags)
	to := *from
	to.NamespaceRoots.Roots = make([]string, len(from.NamespaceRoots.Roots))
	copy(to.NamespaceRoots.Roots, from.NamespaceRoots.Roots)
	return to
}

// ListenFlags returns a copy of the Listen flag group stored in Flags.
// This copy will contain default values if the Listen flag group
// was not specified when CreateAndRegister was called. The HasGroup
// method can be used for testing to see if any given group was configured.
func (f *Flags) ListenFlags() ListenFlags {
	if p := f.groups[Listen]; p != nil {
		lf := p.(*ListenFlags)
		n := *lf
		if len(lf.Addrs) == 0 {
			n.Addrs = ListenAddrs{{n.Protocol.String(), n.Addresses.validator.String()}}
			return n
		}
		n.Addrs = make(ListenAddrs, len(lf.Addrs))
		copy(n.Addrs, lf.Addrs)
		return n
	}
	return ListenFlags{}
}

// PermissionsFlags returns a copy of the Permissions flag group stored in
// Flags. This copy will contain default values if the Permissions flag group
// was not specified when CreateAndRegister was called. The HasGroup method can
// be used for testing to see if any given group was configured.
func (f *Flags) PermissionsFlags() PermissionsFlags {
	if p := f.groups[Permissions]; p != nil {
		return *(p.(*PermissionsFlags))
	}
	return PermissionsFlags{}
}

// VirtualizedFlags returns a copy of the Virtualized flag group stored in Flags.
// This copy will contain default values if the Virtualized flag group
// was not specified when CreateAndRegister was called. The HasGroup
// method can be used for testing to see if any given group was configured.
func (f *Flags) VirtualizedFlags() VirtualizedFlags {
	if p := f.groups[Virtualized]; p != nil {
		vf := p.(*VirtualizedFlags)
		n := *vf
		n.PublicAddress.Set(vf.PublicAddress.String())
		n.PublicProtocol.Set(vf.PublicProtocol.String())
		return n
	}
	vf := VirtualizedFlags{}
	vf.VirtualizationProvider.Set("")
	return vf
}

// HasGroup returns group if the supplied FlagGroup has been created
// for these Flags.
func (f *Flags) HasGroup(group FlagGroup) bool {
	_, present := f.groups[group]
	return present
}

// Args returns the unparsed args, as per flag.Args.
func (f *Flags) Args() []string {
	return f.FlagSet.Args()
}

// Parse parses the supplied args, as per flag.Parse.  The config can optionally
// specify flag overrides. Any default values modified since the last call to
// Parse will used.
func (f *Flags) Parse(args []string, cfg map[string]string) error {
	// Refresh any defaults that may have changed.
	if err := refreshDefaults(f); err != nil {
		return err
	}

	// TODO(cnicolaou): implement a single env var 'VANADIUM_OPTS'
	// that can be used to specify any command line.
	if err := f.FlagSet.Parse(args); err != nil {
		return err
	}

	for k, v := range cfg {
		if f.FlagSet.Lookup(k) != nil {
			if err := f.FlagSet.Set(k, v); err != nil {
				return fmt.Errorf("failed to set flag %v to %v: %v", k, v, err)
			}
		}
	}
	return nil
}
