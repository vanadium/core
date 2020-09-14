// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"

	"v.io/v23/naming"
	"v.io/v23/verror"

	"v.io/x/lib/netstate"
)

var (
	errNoCompatibleServers        = verror.NewID("errNoComaptibleServers")
	defaultPreferredProtocolOrder = mkProtocolRankMap([]string{"unixfd", "wsh", "tcp4", "tcp", "*"})
)

type serverLocality int

const (
	unknownNetwork serverLocality = iota
	remoteNetwork
	localNetwork
)

const maxCacheSize = 1 << 11

var (
	cacheMu         sync.Mutex
	cacheValid      <-chan struct{}
	ipNetworksCache []*net.IPNet
	serversCache    = make(map[string]sortableServer) // keyed by concatenated protocols and server name.
)

func init() {
	valid := make(chan struct{})
	cacheValid = valid
	close(valid)
}

// filterAndOrderServers returns a set of servers that are compatible with
// the current client in order of 'preference' specified by the supplied
// protocols and a notion of 'locality' according to the supplied protocol
// list as follows:
// - if the protocol parameter is non-empty, then only servers matching those
// protocols are returned and the endpoints are ordered first by protocol
// and then by locality within each protocol. If tcp4 and unixfd are requested
// for example then only protocols that match tcp4 and unixfd will returned
// with the tcp4 ones preceding the unixfd ones.
// - if the protocol parameter is empty, then a default protocol ordering
// will be used, but unlike the previous case, any servers that don't support
// these protocols will be returned also, but following the default
// preferences.
func filterAndOrderServers(servers []naming.MountedServer, protocols []string, ipnets ...*net.IPNet) ([]naming.MountedServer, error) {
	if ipnets == nil {
		if err := refreshCache(); err != nil {
			return nil, err
		}
		cacheMu.Lock()
		ipnets = ipNetworksCache
		cacheMu.Unlock()
	}
	var (
		errs       = []error{}
		list       = make(sortableServerList, 0, len(servers))
		protoRanks = mkProtocolRankMap(protocols)
		// TODO(suharshs): We can sort protocols before concatenating to increase
		// cache usage.
		// We prefix cached server information by protocols because different
		// preferred protocols will have different ranks.
		protocolsKey = strings.Join(protocols, ",")
	)
	if len(protoRanks) == 0 {
		protoRanks = defaultPreferredProtocolOrder
	}
	adderr := func(name string, err error) {
		errs = append(errs, verror.SubErr{Name: "server=" + name, Err: err, Options: verror.Print})
	}
	for _, server := range servers {
		ss, err := mkSortableServer(server, protoRanks, protocolsKey, ipnets)
		if err != nil {
			adderr(server.Server, err)
			continue
		}
		list = append(list, ss)
	}
	if len(list) == 0 {
		return nil,
			verror.WithSubErrors(errNoCompatibleServers.Errorf(nil, "failed to find any compatible servers"), errs...)
	}
	// TODO(ashankar): Don't have to use stable sorting, could
	// just use sort.Sort. The only problem with that is the
	// unittest.
	sort.Stable(list)
	// Convert to []naming.MountedServer
	ret := make([]naming.MountedServer, len(list))
	for idx, item := range list {
		ret[idx] = item.server
	}
	return ret, nil
}

func mkSortableServer(server naming.MountedServer, protoRanks map[string]int, protocolsKey string, ipnets []*net.IPNet) (sortableServer, error) {
	name := server.Server
	k := name + "," + protocolsKey
	cacheMu.Lock()
	ss, ok := serversCache[k]
	cacheMu.Unlock()
	if ok {
		return ss, nil
	}

	ep, err := name2endpoint(name)
	if err != nil {
		return sortableServer{}, fmt.Errorf("malformed endpoint: %v", err)
	}
	rank, err := protocol2rank(ep.Addr().Network(), protoRanks)
	if err != nil {
		return sortableServer{}, err
	}
	defer cacheMu.Unlock()
	cacheMu.Lock()
	if len(serversCache) >= maxCacheSize {
		for k := range serversCache {
			delete(serversCache, k)
			break
		}
	}
	ss = sortableServer{
		server:       server,
		protocolRank: rank,
		locality:     locality(ep, ipnets),
	}
	serversCache[k] = ss
	return ss, nil
}

// name2endpoint returns the naming.Endpoint encoded in a name.
func name2endpoint(name string) (naming.Endpoint, error) {
	addr := name
	if naming.Rooted(name) {
		addr, _ = naming.SplitAddressName(name)
	}
	return naming.ParseEndpoint(addr)
}

// protocol2rank returns the "rank" of a protocol (given a map of ranks).
// The higher the rank, the more preferable the protocol.
func protocol2rank(protocol string, ranks map[string]int) (int, error) {
	if r, ok := ranks[protocol]; ok {
		return r, nil
	}
	// Special case: if "wsh" has a rank but "wsh4"/"wsh6" don't,
	// then they get the same rank as "wsh". Similar for "tcp" and "ws".
	//
	// TODO(jhahn): We have similar protocol equivalency checks at a few places.
	// Figure out a way for this mapping to be shared.
	if p := protocol; p == "wsh4" || p == "wsh6" || p == "tcp4" || p == "tcp6" || p == "ws4" || p == "ws6" {
		if r, ok := ranks[p[:len(p)-1]]; ok {
			return r, nil
		}
	}
	// "*" means that any protocol is acceptable.
	if r, ok := ranks["*"]; ok {
		return r, nil
	}
	// UnknownProtocol should be rare, it typically happens when
	// the endpoint is described in <host>:<port> format instead of
	// the full fidelity description (@<version>@<protocol>@...).
	if protocol == naming.UnknownProtocol {
		return -1, nil
	}
	return 0, fmt.Errorf("undesired protocol: %v", protocol)
}

// locality returns the serverLocality to use given an endpoint and the
// set of IP networks configured on this machine.
func locality(ep naming.Endpoint, ipnets []*net.IPNet) serverLocality {
	if len(ipnets) < 1 {
		return unknownNetwork // 0 IP networks, locality doesn't matter.

	}
	host, _, err := net.SplitHostPort(ep.Addr().String())
	if err != nil {
		host = ep.Addr().String()
	}
	ip := net.ParseIP(host)
	if ip == nil {
		// Not an IP address (possibly not an IP network).
		return unknownNetwork
	}
	for _, ipnet := range ipnets {
		if ipnet.Contains(ip) {
			return localNetwork
		}
	}
	return remoteNetwork
}

// ipNetworks returns the IP networks on this machine.
// The returned chan is closed when the ipnetworks have changed.
func ipNetworks() ([]*net.IPNet, <-chan struct{}, error) {
	ifcs, valid, err := netstate.GetAllAddresses()
	if err != nil {
		return nil, nil, err
	}
	ret := make([]*net.IPNet, 0, len(ifcs))
	for _, a := range ifcs {
		_, ipnet, err := net.ParseCIDR(a.String())
		if err != nil {
			return nil, nil, err
		}
		ret = append(ret, ipnet)
	}
	return ret, valid, nil
}

func refreshCache() error {
	cacheMu.Lock()
	select {
	case <-cacheValid:
		var err error
		if ipNetworksCache, cacheValid, err = ipNetworks(); err != nil {
			return err
		}
		serversCache = make(map[string]sortableServer)
	default:
	}
	cacheMu.Unlock()
	return nil
}

type sortableServer struct {
	server       naming.MountedServer
	protocolRank int            // larger values are preferred.
	locality     serverLocality // larger values are preferred.
}

func (s *sortableServer) String() string {
	return fmt.Sprintf("%v", s.server)
}

type sortableServerList []sortableServer

func (l sortableServerList) Len() int      { return len(l) }
func (l sortableServerList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l sortableServerList) Less(i, j int) bool {
	if l[i].protocolRank == l[j].protocolRank {
		return l[i].locality > l[j].locality
	}
	return l[i].protocolRank > l[j].protocolRank
}

func mkProtocolRankMap(list []string) map[string]int {
	if len(list) == 0 {
		return nil
	}
	m := make(map[string]int)
	for idx, protocol := range list {
		m[protocol] = len(list) - idx
	}
	return m
}
