// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"flag"
	"strings"

	"v.io/v23/verror"
	"v.io/x/lib/cmd/flagvar"
	"v.io/x/ref"
)

const pkgPath = "v.io/x/ref/lib/flags"

var (
	errNotNameColonFile = verror.Register(pkgPath+".errNotNameColonFile", verror.NoRetry, "{1:}{2:} {3} is not in 'name:file' format{:_}")
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
	// v23.virtualized.literal-dns-name
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
func CreateAndRegisterRuntimeFlags(fs *flag.FlagSet) *RuntimeFlags {
	rf := NewRuntimeFlags()
	RegisterRuntimeFlags(fs, rf)
	return rf
}

// NewRuntimeFlags creates a new RuntimeFlags with appropriate defaults.
func NewRuntimeFlags() *RuntimeFlags {
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
	return rf
}

// RegisterRuntimeFlags registers the supplied RuntimeFlags variable with
// the supplied FlagSet.
func RegisterRuntimeFlags(fs *flag.FlagSet, f *RuntimeFlags) {
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
	if err != nil {
		// panic since this is clearly a programming error.
		panic(err)
	}
}

// CreateAndRegisterPermissionsFlags creates and registers a PermissionsFlags
// with the supplied FlagSet.
func CreateAndRegisterPermissionsFlags(fs *flag.FlagSet) *PermissionsFlags {
	pf := NewPermissionsFlags()
	RegisterPermissionsFlags(fs, pf)
	return pf
}

// NewPermissionsFlags creates a PermissionsFlags with appropriate defaults.
func NewPermissionsFlags() *PermissionsFlags {
	return &PermissionsFlags{
		Files:   PermissionsFlag{files: DefaultPermissions()},
		Literal: PermissionsLiteralFlag{permissions: DefaultPermissionsLiteral()},
	}
}

// RegisterPermissionsFlags registers the supplied PermissionsFlags with
// the supplied FlagSet.
func RegisterPermissionsFlags(fs *flag.FlagSet, f *PermissionsFlags) {
	err := flagvar.RegisterFlagsInStruct(fs, "cmdline", f,
		nil,
		map[string]string{
			"v23.permissions.file": "",
		})
	if err != nil {
		// panic since this is clearly a programming error.
		panic(err)
	}
}

// CreateAndRegisterListenFlags creates and registers the ListenFlags
// group with the supplied flag.FlagSet.
func CreateAndRegisterListenFlags(fs *flag.FlagSet) *ListenFlags {
	lf := NewListenFlags()
	RegisterListenFlags(fs, lf)
	return lf
}

// NewListenFlags creates a new ListenFlags with appropriate defaults.
func NewListenFlags() *ListenFlags {
	lf := &ListenFlags{}
	var ipHostPortFlag IPHostPortFlag
	if err := ipHostPortFlag.Set(DefaultHostPort()); err != nil {
		panic(err)
	}
	var protocolFlag TCPProtocolFlag
	if err := protocolFlag.Set(DefaultProtocol()); err != nil {
		panic(err)
	}
	lf.Protocol = tcpProtocolFlagVar{validator: protocolFlag}
	lf.Addresses = ipHostPortFlagVar{validator: ipHostPortFlag}
	lf.Proxy = DefaultProxy()
	return lf
}

// RegisterListenFlags registers the supplied ListenFlags variable with
// the supplied FlagSet.
func RegisterListenFlags(fs *flag.FlagSet, f *ListenFlags) {
	f.Addresses.flags = f
	err := flagvar.RegisterFlagsInStruct(fs, "cmdline", f,
		map[string]interface{}{
			"v23.proxy": DefaultProxy(),
		}, map[string]string{
			"v23.proxy": "",
		},
	)
	if err != nil {
		// panic since this is clearly a programming error.
		panic(err)
	}
}

// CreateAndRegisterVirtualizedFlags creates and registers the VirtualizedFlags
// group with the supplied flag.FlagSet.
func CreateAndRegisterVirtualizedFlags(fs *flag.FlagSet) *VirtualizedFlags {
	lf := NewVirtualizedFlags()
	RegisterVirtualizedFlags(fs, lf)
	return lf
}

// NewVirtualizedFlags creates a new VirtualizedFlags with appropriate defaults.
func NewVirtualizedFlags() *VirtualizedFlags {
	def := DefaultVirtualizedFlagValues()
	vf := &VirtualizedFlags{}
	var ipHostPortFlag IPHostPortFlag
	if err := ipHostPortFlag.Set(def.PublicAddress); err != nil {
		panic(err)
	}
	var protocolFlag TCPProtocolFlag
	if err := protocolFlag.Set(def.PublicProtocol); err != nil {
		panic(err)
	}
	vf.PublicProtocol = tcpProtocolFlagVar{validator: protocolFlag}
	vf.PublicAddress = ipHostPortFlagVar{validator: ipHostPortFlag}
	return vf
}

// RegisterVirtualizedFlags registers the supplied VirtualizedFlags variable with
// the supplied FlagSet.
func RegisterVirtualizedFlags(fs *flag.FlagSet, f *VirtualizedFlags) {
	def := DefaultVirtualizedFlagValues()
	err := flagvar.RegisterFlagsInStruct(fs, "cmdline", f,
		map[string]interface{}{
			"v23.virtualized.docker":                  def.Dockerized,
			"v23.virtualized.provider":                def.VirtualizationProvider,
			"v23.virtualized.discover-public-address": def.DiscoverPublicIP,
			"v23.virtualized.tcp.public-protocol":     def.PublicProtocol,
			"v23.virtualized.tcp.public-address":      def.PublicAddress,
			"v23.virtualized.literal-dns-name":        def.LiteralDNSName,
		}, map[string]string{
			"v23.virtualized.docker":                  "false",
			"v23.virtualized.provider":                "",
			"v23.virtualized.discover-public-address": "true",
			"v23.virtualized.tcp.public-protocol":     "",
			"v23.virtualized.tcp.public-address":      "",
			"v23.virtualized.literal-dns-name":        "",
		},
	)
	if err != nil {
		// panic since this is clearly a programming error.
		panic(err)
	}
}

// CreateAndRegister creates a new set of flag groups as specified by the
// supplied flag group parameters and registers them with the supplied
// flag.FlagSet.
func CreateAndRegister(fs *flag.FlagSet, groups ...FlagGroup) *Flags {
	if len(groups) == 0 {
		return nil
	}
	f := &Flags{FlagSet: fs, groups: make(map[FlagGroup]interface{})}
	for _, g := range groups {
		switch g {
		case Runtime:
			f.groups[Runtime] = CreateAndRegisterRuntimeFlags(fs)
		case Listen:
			f.groups[Listen] = CreateAndRegisterListenFlags(fs)
		case Permissions:
			f.groups[Permissions] = CreateAndRegisterPermissionsFlags(fs)
		case Virtualized:
			f.groups[Virtualized] = CreateAndRegisterVirtualizedFlags(fs)
		}
	}
	return f
}

func refreshDefaults(f *Flags) {
	for _, g := range f.groups {
		switch v := g.(type) {
		case *RuntimeFlags:
			if v.NamespaceRoots.isDefault {
				v.NamespaceRoots.Roots = DefaultNamespaceRoots()
			}
		case *ListenFlags:
			if !v.Protocol.isSet {
				v.Protocol.validator.Set(defaultProtocol) //nolint:errcheck
			}
			if !v.Addresses.isSet {
				v.Addresses.validator.Set(defaultHostPort) //nolint:errcheck
			}

		case *VirtualizedFlags:
			if !v.PublicProtocol.isSet {
				v.PublicProtocol.validator.Set(defaultVirtualized.PublicProtocol) //nolint:errcheck
			}
			if !v.PublicAddress.isSet {
				v.PublicAddress.validator.Set(defaultVirtualized.PublicAddress) //nolint:errcheck
			}
		}
	}
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

func (f *Flags) VirtualizedFlags() VirtualizedFlags {
	if p := f.groups[Virtualized]; p != nil {
		vf := p.(*VirtualizedFlags)
		n := *vf
		return n
	}
	return VirtualizedFlags{}
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
// specify flag overrides.
func (f *Flags) Parse(args []string, cfg map[string]string) error {
	// Refresh any defaults that may have changed.
	refreshDefaults(f)

	// TODO(cnicolaou): implement a single env var 'VANADIUM_OPTS'
	// that can be used to specify any command line.
	if err := f.FlagSet.Parse(args); err != nil {
		return err
	}
	for k, v := range cfg {
		if f.FlagSet.Lookup(k) != nil {
			f.FlagSet.Set(k, v) //nolint:errcheck
		}
	}
	return nil
}
