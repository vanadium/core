// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mounttablelib

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	mdns "github.com/vanadium/go-mdns-sd"
	"v.io/v23/context"
	"v.io/v23/glob"
	"v.io/v23/naming"
	"v.io/v23/rpc"
	"v.io/v23/security"
	"v.io/v23/security/access"
	"v.io/v23/services/mounttable"
	vdltime "v.io/v23/vdlroot/time"
	"v.io/v23/verror"
	"v.io/x/lib/netconfig"
	"v.io/x/ref/internal/logger"
)

const addressPrefix = "address:"

// neighborhood defines a set of machines on the same multicast media.
type neighborhood struct {
	mdns             *mdns.MDNS
	nelems           int
	stopWatch        chan struct{}
	lastSubscription time.Time
}

var _ rpc.Dispatcher = (*neighborhood)(nil)

type neighborhoodService struct {
	name  string
	elems []string
	nh    *neighborhood
}

func getPort(address string) uint16 {
	epAddr, _ := naming.SplitAddressName(address)

	ep, err := naming.ParseEndpoint(epAddr)
	if err != nil {
		return 0
	}
	addr := ep.Addr()
	if addr == nil {
		return 0
	}
	switch addr.Network() {
	case "tcp", "tcp4", "tcp6", "ws", "ws4", "ws6", "wsh", "wsh4", "wsh6":
	default:
		return 0
	}
	_, pstr, err := net.SplitHostPort(addr.String())
	if err != nil {
		return 0
	}
	port, err := strconv.ParseUint(pstr, 10, 16)
	if err != nil || port == 0 {
		return 0
	}
	return uint16(port)
}

func newNeighborhood(host string, addresses []string, loopback bool) (*neighborhood, error) {
	if strings.Contains(host, "/") {
		return nil, fmt.Errorf("hostname may not contain '/': %v", host)
	}

	// Create the TXT contents with addresses to announce. Also pick up a port number.
	var txt []string
	var port uint16
	for _, addr := range addresses {
		txt = append(txt, addressPrefix+addr)
		if port == 0 {
			port = getPort(addr)
		}
	}
	if txt == nil {
		return nil, fmt.Errorf("neighborhood passed no useful addresses")
	}
	if port == 0 {
		return nil, fmt.Errorf("neighborhood couldn't determine a port to use{")
	}

	// Start up MDNS, subscribe to the vanadium service, and add us as a vanadium service provider.
	m, err := mdns.NewMDNS(host, "", "", loopback, 0)
	if err != nil {
		// The name may not have been unique.  Try one more time with a unique
		// name.  NewMDNS will replace the "()" with "(hardware mac address)".
		if len(host) > 0 {
			m, err = mdns.NewMDNS(host+"()", "", "", loopback, 0)
		}
		if err != nil {
			logger.Global().Errorf("mdns startup failed: %s", err)
			return nil, err
		}
	}
	logger.Global().VI(2).Infof("listening for service vanadium on port %d", port)
	m.SubscribeToService("vanadium")
	if len(host) > 0 {
		m.AddService("vanadium", "", port, txt...) //nolint:errcheck
	}

	// A small sleep to allow the world to learn about us and vice versa.  Not
	// necessary but helpful.
	time.Sleep(50 * time.Millisecond)

	nh := &neighborhood{
		mdns: m,
	}

	// Watch the network configuration so that we can make MDNS reattach to
	// interfaces when the network changes.
	nh.stopWatch = make(chan struct{}, 1)
	go func() {
		ch, err := netconfig.NotifyChange()
		if err != nil {
			logger.Global().Errorf("neighborhood can't watch network: %v", err)
			return
		}
		select {
		case <-nh.stopWatch:
			return
		case <-ch:
			if _, err := nh.mdns.ScanInterfaces(); err != nil {
				logger.Global().Errorf("nighborhood can't scan interfaces: %s", err)
			}
		}
	}()

	return nh, nil
}

// NewLoopbackNeighborhoodDispatcher creates a new instance of a dispatcher for
// a neighborhood service provider on loopback interfaces (meant for testing).
func NewLoopbackNeighborhoodDispatcher(host string, addresses ...string) (rpc.Dispatcher, error) {
	return newNeighborhood(host, addresses, true)
}

// NewNeighborhoodDispatcher creates a new instance of a dispatcher for a
// neighborhood service provider.
func NewNeighborhoodDispatcher(host string, addresses ...string) (rpc.Dispatcher, error) {
	return newNeighborhood(host, addresses, false)
}

// Lookup implements rpc.Dispatcher.Lookup.
func (nh *neighborhood) Lookup(ctx *context.T, name string) (interface{}, security.Authorizer, error) {
	logger.Global().VI(1).Infof("*********************LookupServer '%s'\n", name)
	elems := strings.Split(name, "/")[nh.nelems:]
	if name == "" {
		elems = nil
	}
	ns := &neighborhoodService{
		name:  name,
		elems: elems,
		nh:    nh,
	}
	return mounttable.MountTableServer(ns), nh, nil
}

func (nh *neighborhood) Authorize(*context.T, security.Call) error {
	// TODO(rthellend): Figure out whether it's OK to accept all requests
	// unconditionally.
	return nil
}

// Stop performs cleanup.
func (nh *neighborhood) Stop() {
	close(nh.stopWatch)
	nh.mdns.Stop()
}

// neighbor returns the MountedServers for a particular neighbor.
func (nh *neighborhood) neighbor(instance string) []naming.MountedServer {
	now := time.Now()
	var reply []naming.MountedServer
	si := nh.mdns.ResolveInstance(instance, "vanadium")

	// Use a map to dedup any addresses seen
	addrMap := make(map[string]vdltime.Deadline)

	// Look for any TXT records with addresses.
	for _, rr := range si.TxtRRs {
		for _, s := range rr.Txt {
			if !strings.HasPrefix(s, addressPrefix) {
				continue
			}
			addr := s[len(addressPrefix):]
			ttl := time.Second * time.Duration(rr.Header().Ttl)
			addrMap[addr] = vdltime.Deadline{Time: now.Add(ttl)}
		}
	}
	for addr, deadline := range addrMap {
		reply = append(reply, naming.MountedServer{
			Server:   addr,
			Deadline: deadline,
		})
	}
	return reply
}

// neighbors returns all neighbors and their MountedServer structs.
func (nh *neighborhood) neighbors() map[string][]naming.MountedServer {
	// If we haven't refreshed in a while, do it now.
	if time.Since(nh.lastSubscription) > time.Duration(30)*time.Second {
		nh.mdns.SubscribeToService("vanadium")
		time.Sleep(50 * time.Millisecond)
		nh.lastSubscription = time.Now()
	}
	neighbors := make(map[string][]naming.MountedServer)
	members := nh.mdns.ServiceDiscovery("vanadium")
	for _, m := range members {
		if neighbor := nh.neighbor(m.Name); neighbor != nil {
			neighbors[m.Name] = neighbor
		}
	}
	logger.Global().VI(2).Infof("members %v neighbors %v", members, neighbors)
	return neighbors
}

// ResolveStep implements ResolveStep
func (ns *neighborhoodService) ResolveStep(ctx *context.T, _ rpc.ServerCall) (entry naming.MountEntry, err error) {
	nh := ns.nh
	ctx.VI(2).Infof("ResolveStep %v\n", ns.elems)
	if len(ns.elems) == 0 {
		// Nothing can be mounted at the root.
		err = naming.ErrNoSuchNameRoot.Errorf(ctx, "namespace root name %s doesn't exist", ns.elems)
		return
	}

	// We can only resolve the first element and it always refers to a mount table (for now).
	neighbor := nh.neighbor(ns.elems[0])
	if neighbor == nil {
		err = naming.ErrNoSuchName.Errorf(ctx, "name %s doesn't exist", ns.elems)
		entry.Name = ns.name
		return
	}
	entry.ServesMountTable = true
	entry.Name = naming.Join(ns.elems[1:]...)
	entry.Servers = neighbor
	return
}

// Mount not implemented.
func (ns *neighborhoodService) Mount(ctx *context.T, _ rpc.ServerCall, _ string, _ uint32, _ naming.MountFlag) error {
	return verror.ErrNotImplemented.Errorf(ctx, "not implemented")
}

// Unmount not implemented.
func (*neighborhoodService) Unmount(ctx *context.T, _ rpc.ServerCall, _ string) error {
	return verror.ErrNotImplemented.Errorf(ctx, "not implemented")
}

// Delete not implemented.
func (*neighborhoodService) Delete(ctx *context.T, _ rpc.ServerCall, _ bool) error {
	return verror.ErrNotImplemented.Errorf(ctx, "not implemented")
}

// Glob__ implements rpc.AllGlobber
//nolint:golint // API change required.
func (ns *neighborhoodService) Glob__(ctx *context.T, call rpc.GlobServerCall, g *glob.Glob) error {
	// return all neighbors that match the first element of the pattern.
	nh := ns.nh

	sender := call.SendStream()
	switch len(ns.elems) {
	case 0:
		matcher := g.Head()
		for k, n := range nh.neighbors() {
			if matcher.Match(k) {
				//nolint:errcheck
				sender.Send(naming.GlobReplyEntry{Value: naming.MountEntry{Name: k, Servers: n, ServesMountTable: true}})

			}
		}
		return nil
	case 1:
		neighbor := nh.neighbor(ns.elems[0])
		if neighbor == nil {
			return naming.ErrNoSuchName.Errorf(ctx, "name %s doesn't exist", ns.elems[0])
		}
		//nolint:errcheck
		sender.Send(naming.GlobReplyEntry{Value: naming.MountEntry{Name: "", Servers: neighbor, ServesMountTable: true}})
		return nil
	default:
		return naming.ErrNoSuchName.Errorf(ctx, "name %s doesn't exist", ns.elems)
	}
}

func (*neighborhoodService) SetPermissions(ctx *context.T, _ rpc.ServerCall, _ access.Permissions, _ string) error {
	return verror.ErrNotImplemented.Errorf(ctx, "not implemented")
}

func (*neighborhoodService) GetPermissions(*context.T, rpc.ServerCall) (access.Permissions, string, error) {
	return nil, "", nil
}
