// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package rpc

import (
	"net"
	"reflect"
	"strings"
	"testing"

	"v.io/v23/naming"
)

func servers2names(servers []naming.MountedServer) []string {
	e := naming.MountEntry{Servers: servers}
	return e.Names()
}

func TestIncompatible(t *testing.T) {
	servers := []naming.MountedServer{}

	_, err := filterAndOrderServers(servers, []string{"tcp"})
	if err == nil || !strings.Contains(err.Error(), "failed to find any compatible servers") {
		t.Errorf("expected a different error: %v", err)
	}

	for _, a := range []string{"127.0.0.3", "127.0.0.4"} {
		name := naming.JoinAddressName(naming.FormatEndpoint("tcp", a), "")
		servers = append(servers, naming.MountedServer{Server: name})
	}

	_, err = filterAndOrderServers(servers, []string{"foobar"})
	if err == nil || !strings.HasSuffix(err.Error(), "undesired protocol: tcp]") {
		t.Errorf("expected a different error to: %v", err)
	}
}

func TestOrderingByProtocol(t *testing.T) { //nolint:gocyclo
	servers := []naming.MountedServer{}
	_, ipnet, _ := net.ParseCIDR("127.0.0.0/8")
	ipnets := []*net.IPNet{ipnet}

	for _, a := range []string{"127.0.0.3", "127.0.0.4"} {
		name := naming.JoinAddressName(naming.FormatEndpoint("tcp", a), "")
		servers = append(servers, naming.MountedServer{Server: name})
	}
	for _, a := range []string{"127.0.0.1", "127.0.0.2"} {
		name := naming.JoinAddressName(naming.FormatEndpoint("tcp4", a), "")
		servers = append(servers, naming.MountedServer{Server: name})
	}
	for _, a := range []string{"127.0.0.10", "127.0.0.11"} {
		name := naming.JoinAddressName(naming.FormatEndpoint("foobar", a), "")
		servers = append(servers, naming.MountedServer{Server: name})
	}
	for _, a := range []string{"127.0.0.7", "127.0.0.8"} {
		name := naming.JoinAddressName(naming.FormatEndpoint("tcp6", a), "")
		servers = append(servers, naming.MountedServer{Server: name})
	}
	if _, err := filterAndOrderServers(servers, []string{"batman"}, ipnets...); err == nil {
		t.Fatalf("expected an error")
	}

	// Add a server with naming.UnknownProtocol. This typically happens
	// when the endpoint is in <host>:<port> format. Currently, the sorting
	// is setup to always allow UnknownProtocol, but put it in the end.
	// May want to revisit this choice, but for now the test captures what
	// the current state of the code intends.
	servers = append(servers, naming.MountedServer{Server: "127.0.0.12:14141"})

	// Just foobar and tcp4
	want := []string{
		"/@6@foobar@127.0.0.10@@@@@@",
		"/@6@foobar@127.0.0.11@@@@@@",
		"/@6@tcp4@127.0.0.1@@@@@@",
		"/@6@tcp4@127.0.0.2@@@@@@",
		"/127.0.0.12:14141",
	}
	result, err := filterAndOrderServers(servers, []string{"foobar", "tcp4"}, ipnets...)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got := servers2names(result); !reflect.DeepEqual(got, want) {
		t.Errorf("got: %v, want %v", got, want)
	}

	// Everything, since we didn't specify a protocol, but ordered by
	// the internal metric - see defaultPreferredProtocolOrder.
	// The order will be the default preferred order for protocols, the
	// original ordering within each protocol, with protocols that
	// are not in the default ordering list at the end.
	want = []string{
		"/@6@tcp4@127.0.0.1@@@@@@",
		"/@6@tcp4@127.0.0.2@@@@@@",
		"/@6@tcp@127.0.0.3@@@@@@",
		"/@6@tcp@127.0.0.4@@@@@@",
		"/@6@tcp6@127.0.0.7@@@@@@",
		"/@6@tcp6@127.0.0.8@@@@@@",
		"/@6@foobar@127.0.0.10@@@@@@",
		"/@6@foobar@127.0.0.11@@@@@@",
		"/127.0.0.12:14141",
	}
	if result, err = filterAndOrderServers(servers, nil, ipnets...); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got := servers2names(result); !reflect.DeepEqual(got, want) {
		t.Errorf("got: %v, want %v", got, want)
	}

	if result, err = filterAndOrderServers(servers, []string{}, ipnets...); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got := servers2names(result); !reflect.DeepEqual(got, want) {
		t.Errorf("got: %v, want %v", got, want)
	}

	// Just "tcp" implies tcp4 and tcp6 as well.
	want = []string{
		"/@6@tcp@127.0.0.3@@@@@@",
		"/@6@tcp@127.0.0.4@@@@@@",
		"/@6@tcp4@127.0.0.1@@@@@@",
		"/@6@tcp4@127.0.0.2@@@@@@",
		"/@6@tcp6@127.0.0.7@@@@@@",
		"/@6@tcp6@127.0.0.8@@@@@@",
		"/127.0.0.12:14141",
	}
	if result, err = filterAndOrderServers(servers, []string{"tcp"}, ipnets...); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got := servers2names(result); !reflect.DeepEqual(got, want) {
		t.Errorf("got: %v, want %v", got, want)
	}

	// Ask for all protocols, with no ordering, except for locality
	want = []string{
		"/@6@tcp@127.0.0.3@@@@@@",
		"/@6@tcp@127.0.0.1@@@@@@",
		"/@6@tcp@74.125.69.139@@@@@@",
		"/@6@tcp@192.168.1.10@@@@@@",
		"/@6@tcp@74.125.142.83@@@@@@",
		"/127.0.0.12:14141",
		"/@6@foobar@127.0.0.10@@@@@@",
		"/@6@foobar@127.0.0.11@@@@@@",
	}
	servers = []naming.MountedServer{}
	// naming.UnknownProtocol
	servers = append(servers, naming.MountedServer{Server: "127.0.0.12:14141"})
	for _, a := range []string{"74.125.69.139", "127.0.0.3", "127.0.0.1", "192.168.1.10", "74.125.142.83"} {
		name := naming.JoinAddressName(naming.FormatEndpoint("tcp", a), "")
		servers = append(servers, naming.MountedServer{Server: name})
	}
	for _, a := range []string{"127.0.0.10", "127.0.0.11"} {
		name := naming.JoinAddressName(naming.FormatEndpoint("foobar", a), "")
		servers = append(servers, naming.MountedServer{Server: name})
	}
	if result, err = filterAndOrderServers(servers, []string{}, ipnets...); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	if got := servers2names(result); !reflect.DeepEqual(got, want) {
		t.Errorf("got: %v, want %v", got, want)
	}
}

func TestOrderingByLocality(t *testing.T) {
	servers := []naming.MountedServer{}
	_, ipnet, _ := net.ParseCIDR("127.0.0.0/8")
	ipnets := []*net.IPNet{ipnet}

	for _, a := range []string{"74.125.69.139", "127.0.0.3", "127.0.0.1", "192.168.1.10", "74.125.142.83"} {
		name := naming.JoinAddressName(naming.FormatEndpoint("tcp", a), "")
		servers = append(servers, naming.MountedServer{Server: name})
	}
	result, err := filterAndOrderServers(servers, []string{"tcp"}, ipnets...)
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	want := []string{
		"/@6@tcp@127.0.0.3@@@@@@",
		"/@6@tcp@127.0.0.1@@@@@@",
		"/@6@tcp@74.125.69.139@@@@@@",
		"/@6@tcp@192.168.1.10@@@@@@",
		"/@6@tcp@74.125.142.83@@@@@@",
	}
	if got := servers2names(result); !reflect.DeepEqual(got, want) {
		t.Errorf("got: %v, want %v", got, want)
	}
	for _, a := range []string{"74.125.69.139", "127.0.0.3:123", "127.0.0.1", "192.168.1.10", "74.125.142.83"} {
		name := naming.JoinAddressName(naming.FormatEndpoint("ws", a), "")
		servers = append(servers, naming.MountedServer{Server: name})
	}

	if result, err = filterAndOrderServers(servers, []string{"ws", "tcp"}, ipnets...); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
	want = []string{
		"/@6@ws@127.0.0.3:123@@@@@@",
		"/@6@ws@127.0.0.1@@@@@@",
		"/@6@ws@74.125.69.139@@@@@@",
		"/@6@ws@192.168.1.10@@@@@@",
		"/@6@ws@74.125.142.83@@@@@@",
		"/@6@tcp@127.0.0.3@@@@@@",
		"/@6@tcp@127.0.0.1@@@@@@",
		"/@6@tcp@74.125.69.139@@@@@@",
		"/@6@tcp@192.168.1.10@@@@@@",
		"/@6@tcp@74.125.142.83@@@@@@",
	}
	if got := servers2names(result); !reflect.DeepEqual(got, want) {
		t.Errorf("got: %v, want %v", got, want)
	}
}
