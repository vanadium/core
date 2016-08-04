// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"v.io/v23/verror"
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
	// --v23.i18n-catalogue
	Listen
	// --v23.permissions.file (which may be repeated to supply multiple values)
	// Permissions files are named - i.e. --v23.permissions.file=<name>:<file>
	// with the name "runtime" reserved for use by the runtime. "file" is
	// a JSON-encoded representation of the Permissions type defined in the
	// VDL package v.io/v23/security/access
	// -v23.permissions.literal
	Permissions
)

var (
	defaultNamespaceRoot = "/(dev.v.io:r:vprod:service:mounttabled)@ns.dev.v.io:8101" // GUARDED_BY namespaceMu
	namespaceMu          sync.Mutex

	defaultProtocol = "wsh" // GUARDED_BY listenMu
	defaultHostPort = ":0"  // GUARDED_BY listenMu
	listenMu        sync.RWMutex
)

// Flags represents the set of flag groups created by a call to
// CreateAndRegister.
type Flags struct {
	FlagSet *flag.FlagSet
	groups  map[FlagGroup]interface{}
}

type namespaceRootFlagVar struct {
	isSet bool // is true when a flag has been explicitly set.
	// isDefault true when a flag has the default value and is needed in
	// addition to isSet to distinguish between using a default value
	// as opposed to one from an environment variable.
	isDefault bool
	roots     []string
}

func (nsr *namespaceRootFlagVar) String() string {
	return fmt.Sprintf("%v", nsr.roots)
}

func (nsr *namespaceRootFlagVar) Set(v string) error {
	nsr.isDefault = false
	if !nsr.isSet {
		// override the default value
		nsr.isSet = true
		nsr.roots = []string{}
	}
	for _, t := range nsr.roots {
		if v == t {
			return nil
		}
	}
	nsr.roots = append(nsr.roots, v)
	return nil
}

type permsFlagVar struct {
	isSet bool
	files map[string]string
}

func (permsf *permsFlagVar) String() string {
	return fmt.Sprintf("%v", permsf.files)
}

func (permsf *permsFlagVar) Set(v string) error {
	if !permsf.isSet {
		// override the default value
		permsf.isSet = true
		permsf.files = make(map[string]string)
	}
	parts := strings.SplitN(v, ":", 2)
	if len(parts) != 2 {
		return verror.New(errNotNameColonFile, nil, v)
	}
	name, file := parts[0], parts[1]
	permsf.files[name] = file
	return nil
}

// RuntimeFlags contains the values of the Runtime flag group.
type RuntimeFlags struct {
	// NamespaceRoots may be initialized by ref.EnvNamespacePrefix* enivornment
	// variables as well as --v23.namespace.root. The command line
	// will override the environment.
	NamespaceRoots []string

	// Credentials may be initialized by the ref.EnvCredentials
	// environment variable. The command line will override the environment.
	Credentials string // TODO(cnicolaou): provide flag.Value impl

	// I18nCatalogue may be initialized by the ref.EnvI18nCatalogueFiles
	// environment variable.  The command line will override the
	// environment.
	I18nCatalogue string

	// Vtrace flags control various aspects of Vtrace.
	Vtrace VtraceFlags

	namespaceRootsFlag namespaceRootFlagVar
}

type VtraceFlags struct {
	// VtraceSampleRate is the rate (from 0.0 - 1.0) at which
	// vtrace traces started by this process are sampled for collection.
	SampleRate float64

	// VtraceDumpOnShutdown tells the runtime to dump all stored traces
	// to Stderr at shutdown if true.
	DumpOnShutdown bool

	// VtraceCacheSize is the number of traces to cache in memory.
	// TODO(mattr): Traces can be of widely varying size, we should have
	// some better measurement then just number of traces.
	CacheSize int

	// LogLevel is the level of vlogs that should be collected as part of
	// the trace
	LogLevel int

	// SpanRegexp matches a regular expression against span names and
	// annotations and forces any trace matching trace to be collected.
	CollectRegexp string
}

// PermissionsFlags contains the values of the PermissionsFlags flag group.
type PermissionsFlags struct {
	// List of named Permissions files.
	fileFlag permsFlagVar

	// Single json string, overrides everything.
	literal string
}

// PermissionsFile returns the file which is presumed to contain Permissions
// information associated with the supplied name parameter.
func (af PermissionsFlags) PermissionsFile(name string) string {
	return af.fileFlag.files[name]
}

func (af PermissionsFlags) PermissionsLiteral() string {
	return af.literal
}

// ListenAddrs is the set of listen addresses captured from the command line.
// ListenAddrs mirrors rpc.ListenAddrs.
type ListenAddrs []struct {
	Protocol, Address string
}

// ListenFlags contains the values of the Listen flag group.
type ListenFlags struct {
	Addrs     ListenAddrs
	Proxy     string
	protocol  tcpProtocolFlagVar
	addresses ipHostPortFlagVar
}

type tcpProtocolFlagVar struct {
	isSet     bool
	validator TCPProtocolFlag
}

// Implements flag.Value.Get
func (proto tcpProtocolFlagVar) Get() interface{} {
	return proto.validator.String()
}

func (proto tcpProtocolFlagVar) String() string {
	return proto.validator.String()
}

// Implements flag.Value.Set
func (proto *tcpProtocolFlagVar) Set(s string) error {
	if err := proto.validator.Set(s); err != nil {
		return err
	}
	proto.isSet = true
	return nil
}

type ipHostPortFlagVar struct {
	isSet     bool
	validator IPHostPortFlag
	flags     *ListenFlags
}

// Implements flag.Value.Get
func (ip ipHostPortFlagVar) Get() interface{} {
	return ip.String()
}

// Implements flag.Value.Set
func (ip *ipHostPortFlagVar) Set(s string) error {
	if err := ip.validator.Set(s); err != nil {
		return err
	}
	a := struct {
		Protocol, Address string
	}{
		ip.flags.protocol.validator.String(),
		ip.validator.String(),
	}
	for _, t := range ip.flags.Addrs {
		if t.Protocol == a.Protocol && t.Address == a.Address {
			return nil
		}
	}
	ip.flags.Addrs = append(ip.flags.Addrs, a)
	ip.isSet = true
	return nil
}

// Implements flag.Value.String
func (ip ipHostPortFlagVar) String() string {
	s := ""
	for _, a := range ip.flags.Addrs {
		s += fmt.Sprintf("(%s %s)", a.Protocol, a.Address)
	}
	return s
}

// createAndRegisterRuntimeFlags creates and registers the RuntimeFlags
// group with the supplied flag.FlagSet.
func createAndRegisterRuntimeFlags(fs *flag.FlagSet) *RuntimeFlags {
	var (
		f             = &RuntimeFlags{}
		_, roots      = ref.EnvNamespaceRoots()
		creds         = os.Getenv(ref.EnvCredentials)
		i18nCatalogue = os.Getenv(ref.EnvI18nCatalogueFiles)
	)
	if len(roots) == 0 {
		f.namespaceRootsFlag.roots = []string{defaultNamespaceRoot}
		f.namespaceRootsFlag.isDefault = true
	} else {
		f.namespaceRootsFlag.roots = roots
	}

	fs.Var(&f.namespaceRootsFlag, "v23.namespace.root", "local namespace root; can be repeated to provided multiple roots")
	fs.Lookup("v23.namespace.root").DefValue = "[" + defaultNamespaceRoot + "]"
	fs.StringVar(&f.Credentials, "v23.credentials", creds, "directory to use for storing security credentials")
	fs.Lookup("v23.credentials").DefValue = ""
	fs.StringVar(&f.I18nCatalogue, "v23.i18n-catalogue", i18nCatalogue, "18n catalogue files to load, comma separated")
	fs.Lookup("v23.i18n-catalogue").DefValue = ""

	fs.Float64Var(&f.Vtrace.SampleRate, "v23.vtrace.sample-rate", 0.0, "Rate (from 0.0 to 1.0) to sample vtrace traces.")
	fs.BoolVar(&f.Vtrace.DumpOnShutdown, "v23.vtrace.dump-on-shutdown", true, "If true, dump all stored traces on runtime shutdown.")
	fs.IntVar(&f.Vtrace.CacheSize, "v23.vtrace.cache-size", 1024, "The number of vtrace traces to store in memory.")
	fs.IntVar(&f.Vtrace.LogLevel, "v23.vtrace.v", 0, "The verbosity level of the log messages to be captured in traces")
	fs.StringVar(&f.Vtrace.CollectRegexp, "v23.vtrace.collect-regexp", "", "Spans and annotations that match this regular expression will trigger trace collection.")

	return f
}

func createAndRegisterPermissionsFlags(fs *flag.FlagSet) *PermissionsFlags {
	f := &PermissionsFlags{}
	fs.Var(&f.fileFlag, "v23.permissions.file", "specify a perms file as <name>:<permsfile>")
	fs.StringVar(&f.literal, "v23.permissions.literal", "", "explicitly specify the runtime perms as a JSON-encoded access.Permissions. Overrides all --v23.permissions.file flags.")
	return f
}

// SetDefaultProtocol sets the default protocol used when --v23.tcp.protocol is
// not provided. It must be called before flags are parsed for it to take effect.
func SetDefaultProtocol(protocol string) {
	listenMu.Lock()
	defaultProtocol = protocol
	listenMu.Unlock()
}

// SetDefaultHostPort sets the default host and port used when --v23.tcp.address
// is not provided. It must be called before flags are parsed for it to take effect.
func SetDefaultHostPort(s string) {
	listenMu.Lock()
	defaultHostPort = s
	listenMu.Unlock()
}

// SetDefaultNamespaceRoot sets the default value for --v23.namespace.root
func SetDefaultNamespaceRoot(root string) {
	namespaceMu.Lock()
	defaultNamespaceRoot = root
	namespaceMu.Unlock()
}

// DefaultNamespaceRoot gets the default value of --v23.namespace.root
func DefaultNamespaceRoot() string {
	namespaceMu.Lock()
	defer namespaceMu.Unlock()
	return defaultNamespaceRoot
}

// createAndRegisterListenFlags creates and registers the ListenFlags
// group with the supplied flag.FlagSet.
func createAndRegisterListenFlags(fs *flag.FlagSet) *ListenFlags {
	listenMu.RLock()
	defer listenMu.RUnlock()
	var ipHostPortFlag IPHostPortFlag
	if err := ipHostPortFlag.Set(defaultHostPort); err != nil {
		panic(err)
	}
	var protocolFlag TCPProtocolFlag
	if err := protocolFlag.Set(defaultProtocol); err != nil {
		panic(err)
	}
	f := &ListenFlags{
		protocol:  tcpProtocolFlagVar{validator: protocolFlag},
		addresses: ipHostPortFlagVar{validator: ipHostPortFlag},
	}
	f.addresses.flags = f

	fs.Var(&f.protocol, "v23.tcp.protocol", "protocol to listen with")
	fs.Var(&f.addresses, "v23.tcp.address", "address to listen on")
	fs.StringVar(&f.Proxy, "v23.proxy", "", "object name of proxy service to use to export services across network boundaries")

	return f
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
			f.groups[Runtime] = createAndRegisterRuntimeFlags(fs)
		case Listen:
			f.groups[Listen] = createAndRegisterListenFlags(fs)
		case Permissions:
			f.groups[Permissions] = createAndRegisterPermissionsFlags(fs)
		}
	}
	return f
}

func refreshDefaults(f *Flags) {
	for _, g := range f.groups {
		switch v := g.(type) {
		case *RuntimeFlags:
			if v.namespaceRootsFlag.isDefault {
				v.namespaceRootsFlag.roots = []string{defaultNamespaceRoot}
				v.NamespaceRoots = v.namespaceRootsFlag.roots
			}
		case *ListenFlags:
			if !v.protocol.isSet {
				v.protocol.validator.Set(defaultProtocol)
			}
			if !v.addresses.isSet {
				v.addresses.validator.Set(defaultHostPort)
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
	to.NamespaceRoots = make([]string, len(from.NamespaceRoots))
	copy(to.NamespaceRoots, from.NamespaceRoots)
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
			n.Addrs = ListenAddrs{{n.protocol.String(), n.addresses.validator.String()}}
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
			f.FlagSet.Set(k, v)
		}
	}

	hasrt := f.groups[Runtime] != nil
	if hasrt {
		runtime := f.groups[Runtime].(*RuntimeFlags)
		if runtime.namespaceRootsFlag.isSet {
			// command line overrides the environment.
			runtime.NamespaceRoots = runtime.namespaceRootsFlag.roots
		} else {
			// we have a default value for the command line, which
			// is only used if the environment variables have not been
			// supplied.
			if len(runtime.NamespaceRoots) == 0 {
				runtime.NamespaceRoots = runtime.namespaceRootsFlag.roots
			}
		}
	}
	return nil
}
