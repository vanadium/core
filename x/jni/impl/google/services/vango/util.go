// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vango

import (
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/conventions"
	"v.io/v23/discovery"
	"v.io/v23/flow"
	"v.io/v23/naming"
	"v.io/v23/options"
	"v.io/v23/rpc"
	"v.io/v23/security"
)

type echoServer struct{}

func (*echoServer) Echo(ctx *context.T, call rpc.ServerCall, arg string) (string, error) {
	ctx.Infof("echoServer got message '%s' from %v", arg, call.RemoteEndpoint())
	return arg, nil
}

func runServer(ctx *context.T, name string) error {
	_, server, err := v23.WithNewServer(ctx, name, &echoServer{}, security.AllowEveryone())
	if err != nil {
		return err
	}
	ctx.Infof("Server listening on %v", server.Status().Endpoints)
	ctx.Infof("Server listen errors: %v", server.Status().ListenErrors)
	return nil
}

func runClient(ctx *context.T, name string) error {
	summary, err := runTimedCall(ctx, name, "new connection")
	if err != nil {
		return err
	}
	ctx.Infof("Client success: %v", summary)

	summary, err = runTimedCall(ctx, name, "cached connection")
	if err != nil {
		return err
	}
	ctx.Infof("Client success: %v", summary)
	return nil
}

type conner interface {
	Conn() flow.ManagedConn
}

func runTimedCall(ctx *context.T, name, message string, opts ...rpc.CallOpt) (string, error) {
	summary := fmt.Sprintf("[%s] to %v", message, name)
	start := time.Now()
	call, err := v23.GetClient(ctx).StartCall(ctx, name, "Echo", []interface{}{message}, opts...)
	if err != nil {
		return summary, err
	}
	var recvd string
	if err := call.Finish(&recvd); err != nil {
		return summary, err
	}
	elapsed := time.Since(start)
	if recvd != message {
		return summary, fmt.Errorf("got [%s], want [%s]", recvd, message)
	}
	me := security.LocalBlessingNames(ctx, call.Security())
	them, _ := call.RemoteBlessings()
	connstr := "<unknown>"
	if cn, ok := call.(conner); ok {
		connstr = fmt.Sprintf("%p", cn.Conn())
	}
	return fmt.Sprintf("%s in %v (THEM:%v EP:%v) (ME:%v) conn %v", summary, elapsed, them, call.Security().RemoteEndpoint(), me, connstr), nil
}

func username(blessingNames []string) string {
	var ret []string
	for _, p := range conventions.ParseBlessingNames(blessingNames...) {
		ret = append(ret, p.User)
	}
	return strings.Join(ret, ",")
}

func mountName(ctx *context.T, addendums ...string) string {
	var (
		p     = v23.GetPrincipal(ctx)
		b, _  = p.BlessingStore().Default()
		names = conventions.ParseBlessingNames(security.BlessingNames(p, b)...)
	)
	if len(names) == 0 {
		return ""
	}
	return naming.Join(append([]string{names[0].Home()}, addendums...)...)
}

func addRegisteredProto(ls *rpc.ListenSpec, proto, addr string) {
	for _, p := range flow.RegisteredProtocols() {
		if p == proto {
			ls.Addrs = append(ls.Addrs, rpc.ListenAddrs{{Protocol: p, Address: addr}}...)
		}
	}
}

type addrList []net.Addr

func (l addrList) Len() int { return len(l) }
func (l addrList) Less(i, j int) bool {
	if l[i].Network() == l[j].Network() {
		return addrString(l[i]) < addrString(l[j])
	}
	return l[i].Network() < l[j].Network()
}
func (l addrList) Swap(i, j int) { l[i], l[j] = l[j], l[i] }

func addrString(a net.Addr) string {
	if !strings.HasPrefix(a.Network(), "tcp") && !strings.HasPrefix(a.Network(), "wsh") {
		return a.String()
	}
	host, port, err := net.SplitHostPort(a.String())
	if err != nil {
		return a.String()
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return a.String()
	}
	if ip.IsLoopback() {
		host = "loopback"
	}
	if ip.IsLinkLocalUnicast() {
		host = "linklocal"
	}
	if ip.IsMulticast() {
		// Shouldn't happen
		host = "multicast"
	}
	return net.JoinHostPort(host, port)
}

func serverAddrs(status rpc.ServerStatus) []string {
	var addrs addrList
	for _, ep := range status.Endpoints {
		addrs = append(addrs, ep.Addr())
	}
	return prettyAddrList(addrs)
}

func prettyAddrList(addrs addrList) []string {
	if len(addrs) == 0 {
		return nil
	}
	sort.Sort(addrs)
	var ret []string
	for i, a := range addrs {
		str := fmt.Sprintf("(%v, %v)", a.Network(), addrString(a))
		if i == 0 || ret[len(ret)-1] != str {
			ret = append(ret, str)
		}
	}
	return ret
}

func newPeer(ctx *context.T, u discovery.Update) (*peer, error) {
	var (
		addrs     addrList
		usernames = make(map[string]bool)
		me        = naming.MountEntry{IsLeaf: true}
		ns        = v23.GetNamespace(ctx)
	)
	for _, vname := range u.Addresses() {
		entry, err := ns.Resolve(ctx, vname)
		if err != nil {
			ctx.Errorf("Failed to resolve advertised address [%v]: %v", vname, err)
			continue
		}
		for _, s := range entry.Servers {
			epstr, _ := naming.SplitAddressName(s.Server) // suffix should be empty since the server address is what is advertised.
			ep, err := naming.ParseEndpoint(epstr)
			if err != nil {
				ctx.Errorf("Failed to resolve advertised address [%v] into an endpoint: %v", epstr, err)
				continue
			}
			addrs = append(addrs, ep.Addr())
			usernames[username(ep.BlessingNames())] = true
			me.Servers = append(me.Servers, s)
		}
	}
	if len(usernames) == 0 {
		return nil, fmt.Errorf("could not determine user associated with AdId: %v, addrs: %v", u.Id(), u.Addresses())
	}
	ulist := make([]string, 0, len(usernames))
	for u := range usernames {
		ulist = append(ulist, u)
	}
	return &peer{
		username:    strings.Join(ulist, ", "),
		description: fmt.Sprintf("%v at %v (AdId: %v)", ulist, prettyAddrList(addrs), u.Id()),
		adID:        u.Id(),
		preresolved: options.Preresolved{Resolution: &me},
	}, nil

}

type peer struct {
	username    string // Username claimed by the advertisement
	description string
	preresolved options.Preresolved
	adID        discovery.AdId
}

func (p *peer) call(ctx *context.T, message string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()
	start := time.Now()
	call, err := v23.GetClient(ctx).StartCall(ctx, "", "Echo", []interface{}{message}, p.preresolved)
	if err != nil {
		return "", err
	}
	var recvd string
	if err := call.Finish(&recvd); err != nil {
		return "", err
	}
	elapsed := time.Since(start)
	if recvd != message {
		return "", fmt.Errorf("got [%s], want [%s]", recvd, message)
	}
	them, _ := call.RemoteBlessings()
	theiraddr := call.Security().RemoteEndpoint().Addr()
	return fmt.Sprintf("Called %v at (%v, %v) in %v, and said [%v]", username(them), theiraddr.Network(), theiraddr, elapsed, message), nil
}
