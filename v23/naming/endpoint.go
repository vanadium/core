// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package naming

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
)

const (
	separator          = "@"
	separatorRune      = '@'
	suffix             = "@@"
	blessingsSeparator = ","
	routeSeparator     = ","
)

var (
	errInvalidEndpointString = errors.New("invalid endpoint string")
	hostportEP               = regexp.MustCompile(`^(?:\((.*)\)@)?([^@]+)$`)
	// DefaultEndpointVersion is the default of endpoints that we will create
	// when the version is otherwise unspecified.
	DefaultEndpointVersion = 6
)

// Endpoint represents unique identifiers for entities communicating over a
// network.  End users don't use endpoints - they deal solely with object names,
// with the MountTable providing translation of object names to endpoints.
type Endpoint struct {
	Protocol string
	Address  string
	// RoutingID returns the RoutingID associated with this Endpoint.
	RoutingID     RoutingID
	routes        []string
	blessingNames []string

	// ServesMountTable is true if this endpoint serves a mount table.
	// TODO(mattr): Remove it?
	ServesMountTable bool
}

// ParseEndpoint returns an Endpoint by parsing the supplied endpoint
// string as per the format described above. It can be used to test
// a string to see if it's in valid endpoint format.
//
// NewEndpoint will accept strings both in the @ format described
// above and in internet host:port format.
//
// All implementations of NewEndpoint should provide appropriate
// defaults for any endpoint subfields not explicitly provided as
// follows:
//   - a missing protocol will default to a protocol appropriate for the
//     implementation hosting NewEndpoint
//   - a missing host:port will default to :0 - i.e. any port on all
//     interfaces
//   - a missing routing id should default to the null routing id
//   - a missing codec version should default to AnyCodec
//   - a missing RPC version should default to the highest version
//     supported by the runtime implementation hosting NewEndpoint
func ParseEndpoint(input string) (Endpoint, error) {
	// If the endpoint does not end in a @, it must be in [blessing@]host:port format.
	if parts := hostportEP.FindStringSubmatch(input); len(parts) > 0 {
		hostport := parts[len(parts)-1]
		var blessing string
		if len(parts) > 2 {
			blessing = parts[1]
		}
		return parseHostPort(blessing, hostport)
	}

	// Trim the prefix and suffix and parse the rest.
	input = strings.TrimPrefix(strings.TrimSuffix(input, suffix), separator)
	idx := strings.IndexRune(input, separatorRune)
	if idx < 0 {
		return Endpoint{}, errInvalidEndpointString
	}
	switch input[:idx] {
	case "6":
		return parseV6(input[idx+1:])
	default:
		return Endpoint{}, errInvalidEndpointString
	}
}

func parseHostPort(blessing, hostport string) (Endpoint, error) {
	// Could be in host:port format.
	var ep Endpoint
	if _, _, err := net.SplitHostPort(hostport); err != nil {
		return ep, errInvalidEndpointString
	}
	if strings.HasSuffix(hostport, "#") {
		hostport = strings.TrimSuffix(hostport, "#")
	} else {
		ep.ServesMountTable = true
	}
	ep.Protocol = UnknownProtocol
	ep.Address = hostport
	ep.RoutingID = NullRoutingID
	if len(blessing) > 0 {
		ep.blessingNames = []string{blessing}
	}
	return ep, nil
}

func parseRoutes(input string) ([]string, error) {
	if len(input) == 0 {
		return nil, nil
	}
	routes := strings.Split(input, routeSeparator)
	var ok bool
	for i := range routes {
		if routes[i], ok = Unescape(routes[i]); !ok {
			return nil, fmt.Errorf("invalid route: bad escape %s", routes[i])
		}
	}
	return routes, nil
}

func parseV6(input string) (ep Endpoint, err error) {
	err = errInvalidEndpointString

	idx := strings.IndexRune(input, separatorRune)
	if idx < 0 {
		return
	}
	ep.Protocol = input[:idx]
	if len(ep.Protocol) == 0 {
		ep.Protocol = UnknownProtocol
	}

	input = input[idx+1:]
	idx = strings.IndexRune(input, separatorRune)
	if idx < 0 {
		return
	}
	var ok bool
	if ep.Address, ok = Unescape(input[:idx]); !ok {
		return ep, fmt.Errorf("invalid address: bad escape %s", input[:idx])
	}
	if len(ep.Address) == 0 {
		ep.Address = net.JoinHostPort("", "0")
	}

	input = input[idx+1:]
	idx = strings.IndexRune(input, separatorRune)
	if idx < 0 {
		return
	}
	ep.routes, err = parseRoutes(input[:idx])
	if err != nil {
		return
	}

	input = input[idx+1:]
	idx = strings.IndexRune(input, separatorRune)
	if idx < 0 {
		return
	}
	if err := ep.RoutingID.FromString(input[:idx]); err != nil {
		return ep, fmt.Errorf("invalid routing id: %v", err)
	}

	input = input[idx+1:]
	idx = strings.IndexRune(input, separatorRune)
	if idx < 0 {
		return
	}
	switch p := input[:idx]; p {
	case "", "m":
		ep.ServesMountTable = true
	case "s", "l":
	default:
		return ep, fmt.Errorf("invalid mount table flag (%v)", p)
	}

	if blessings := input[idx+1:]; len(blessings) > 0 {
		ep.blessingNames = strings.Split(blessings, blessingsSeparator)
	}
	return ep, nil
}

// WithBlessingNames derives a new endpoint with the given
// blessing names, but otherwise identical to e.
func (e Endpoint) WithBlessingNames(names []string) Endpoint {
	e.blessingNames = append([]string{}, names...)
	return e
}

// WithRoutes derives a new endpoint with the given
// routes, but otherwise identical to e.
func (e Endpoint) WithRoutes(routes []string) Endpoint {
	e.routes = append([]string{}, routes...)
	return e
}

// BlessingNames returns a copy of the blessings that the process associated with
// this Endpoint will present.
func (e Endpoint) BlessingNames() []string {
	return append([]string{}, e.blessingNames...)
}

// Routes returns a copy of the local routing identifiers used for proxying connections
// with multiple proxies.
func (e Endpoint) Routes() []string {
	return append([]string{}, e.routes...)
}

// IsZero returns true if the endpoint is equivalent to the zero value.
func (e Endpoint) IsZero() bool {
	return e.Protocol == "" &&
		e.Address == "" &&
		e.RoutingID == RoutingID{} &&
		len(e.routes) == 0 &&
		len(e.blessingNames) == 0 &&
		!e.ServesMountTable
}

// VersionedString returns a string in the specified format. If the version
// number is unsupported, the current 'default' version will be used.
func (e Endpoint) VersionedString(version int) string {
	// nologcall
	return e.versionedString(version)
}

func (e Endpoint) versionedString(version int) string {
	out := &strings.Builder{}
	out.Grow(256)
	switch version {
	case 6:
		out.WriteString("@6@")
		out.WriteString(e.Protocol)
		out.WriteByte('@')
		out.WriteString(Escape(e.Address, "@"))
		out.WriteByte('@')
		for i, r := range e.routes {
			out.WriteString(Escape(r, routeSeparator))
			if i < len(e.routes)-1 {
				out.WriteString(routeSeparator)
			}
		}
		out.WriteByte('@')
		out.WriteString(e.RoutingID.String())
		if e.ServesMountTable {
			out.WriteString("@m@")
		} else {
			out.WriteString("@s@")
		}
		for i, b := range e.blessingNames {
			out.WriteString(b)
			if i < len(e.blessingNames)-1 {
				out.WriteString(blessingsSeparator)
			}
		}
		out.WriteString("@@")
		return out.String()
	default:
		return e.VersionedString(DefaultEndpointVersion)
	}
}

func (e Endpoint) String() string {
	return e.versionedString(DefaultEndpointVersion)
}

// Name returns a string reprsentation of this Endpoint that can
// be used as a name with rpc.StartCall.
func (e Endpoint) Name() string {
	return JoinAddressName(e.String(), "")
}

// Addr returns a net.Addr whose String method will return the
// the underlying network address encoded in the endpoint rather than
// the endpoint string itself.
// For example, for TCP based endpoints it will return a net.Addr
// whose network is "tcp" and string representation is <host>:<port>,
// than the full Vanadium endpoint as per the String method above.
func (e Endpoint) Addr() net.Addr {
	return addr{network: e.Protocol, address: e.Address}
}

type addr struct {
	network, address string
}

// Network returns "v23" so that Endpoint can implement net.Addr.
func (a addr) Network() string {
	return a.network
}

// String returns a string representation of the endpoint.
//
// The String method formats the endpoint as:
//
//	@<version>@<version specific fields>@@
//
// Where version is an unsigned integer.
//
// Version 6 is the current version for RPC:
//
//	@6@<protocol>@<address>@<route>[,<route>]...@<routingid>@m|s@[<blessing>[,<blessing>]...]@@
//
// Along with Network, this method ensures that Endpoint implements net.Addr.
func (a addr) String() string {
	return a.address
}
