// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package naming

import (
	"net"
	"reflect"
	"testing"
)

func TestEndpoint(t *testing.T) {
	defver := DefaultEndpointVersion
	defer func() {
		DefaultEndpointVersion = defver
	}()
	v6a := Endpoint{
		Protocol:         UnknownProtocol,
		Address:          "batman.com:1234",
		RoutingID:        FixedRoutingID(0xdabbad00),
		ServesMountTable: true,
	}
	v6b := Endpoint{
		Protocol:         UnknownProtocol,
		Address:          "batman.com:2345",
		RoutingID:        FixedRoutingID(0xdabbad00),
		ServesMountTable: false,
	}
	v6c := Endpoint{
		Protocol:         "tcp",
		Address:          "batman.com:2345",
		RoutingID:        FixedRoutingID(0x0),
		ServesMountTable: false,
	}
	v6d := Endpoint{
		Protocol:         "ws6",
		Address:          "batman.com:2345",
		RoutingID:        FixedRoutingID(0x0),
		ServesMountTable: false,
	}
	v6e := Endpoint{
		Protocol:         "tcp",
		Address:          "batman.com:2345",
		RoutingID:        FixedRoutingID(0xba77),
		routes:           []string{"1"},
		ServesMountTable: true,
		blessingNames:    []string{"dev.v.io:foo@bar.com", "dev.v.io:bar@bar.com:delegate"},
	}
	v6f := Endpoint{
		Protocol:         "tcp",
		Address:          "batman.com:2345",
		routes:           []string{"1", "2", "3"},
		RoutingID:        FixedRoutingID(0xba77),
		ServesMountTable: true,
		// Blessings that look similar to other parts of the endpoint.
		blessingNames: []string{"@@", "@s", "@m"},
	}
	v6g := Endpoint{
		Protocol: "tcp",
		Address:  "batman.com:2345",
		// Routes that have commas should be escaped correctly
		routes:           []string{"a,b", ",ab", "ab,"},
		RoutingID:        FixedRoutingID(0xba77),
		ServesMountTable: true,
	}

	testcasesA := []struct {
		endpoint Endpoint
		address  string
	}{
		{v6a, "batman.com:1234"},
		{v6b, "batman.com:2345"},
		{v6c, "batman.com:2345"},
	}
	for _, test := range testcasesA {
		addr := test.endpoint.Addr()
		if addr.String() != test.address {
			t.Errorf("unexpected address %q, not %q", addr.String(), test.address)
		}
	}

	// Test v6 endpoints.
	testcasesC := []struct {
		Endpoint Endpoint
		String   string
		Version  int
	}{
		{v6a, "@6@@batman.com:1234@@000000000000000000000000dabbad00@m@@@", 6},
		{v6b, "@6@@batman.com:2345@@000000000000000000000000dabbad00@s@@@", 6},
		{v6c, "@6@tcp@batman.com:2345@@00000000000000000000000000000000@s@@@", 6},
		{v6d, "@6@ws6@batman.com:2345@@00000000000000000000000000000000@s@@@", 6},
		{v6e, "@6@tcp@batman.com:2345@1@0000000000000000000000000000ba77@m@dev.v.io:foo@bar.com,dev.v.io:bar@bar.com:delegate@@", 6},
		{v6f, "@6@tcp@batman.com:2345@1,2,3@0000000000000000000000000000ba77@m@@@,@s,@m@@", 6},
		{v6g, "@6@tcp@batman.com:2345@a%2Cb,%2Cab,ab%2C@0000000000000000000000000000ba77@m@@@", 6},
	}

	for i, test := range testcasesC {
		if got, want := test.Endpoint.VersionedString(test.Version), test.String; got != want {
			t.Errorf("Test %d: Got %q want %q for endpoint (v%d): %#v", i, got, want, test.Version, test.Endpoint)
		}
		ep, err := ParseEndpoint(test.String)
		if err != nil {
			t.Errorf("Test %d: NewEndpoint(%q) failed with %v", i, test.String, err)
			continue
		}
		if !reflect.DeepEqual(ep, test.Endpoint) {
			t.Errorf("Test %d: Got endpoint %#v, want %#v for string %q", i, ep, test.Endpoint, test.String)
		}
	}
}

type endpointTest struct {
	input, output string
	err           error
}

func runEndpointTests(t *testing.T, testcases []endpointTest) {
	for _, test := range testcases {
		ep, err := ParseEndpoint(test.input)
		if err == nil && test.err == nil && ep.String() != test.output {
			t.Errorf("NewEndpoint(%q): unexpected endpoint string %q != %q",
				test.input, ep.String(), test.output)
			continue
		}
		switch {
		case test.err == err: // do nothing
		case test.err == nil && err != nil:
			t.Errorf("NewEndpoint(%q): unexpected error %q", test.output, err)
		case test.err != nil && err == nil:
			t.Errorf("NewEndpoint(%q): missing error %q", test.output, test.err)
		case err.Error() != test.err.Error():
			t.Errorf("NewEndpoint(%q): unexpected error  %q != %q", test.output, err, test.err)
		}
	}
}

func TestHostPortEndpoint(t *testing.T) {
	defver := DefaultEndpointVersion
	defer func() {
		DefaultEndpointVersion = defver
	}()
	DefaultEndpointVersion = 6
	testcases := []endpointTest{
		{"localhost:10", "@6@@localhost:10@@00000000000000000000000000000000@m@@@", nil},
		{"localhost:", "@6@@localhost:@@00000000000000000000000000000000@m@@@", nil},
		{"localhost", "", errInvalidEndpointString},
		{"(dev.v.io:service:mounttabled)@ns.dev.v.io:8101", "@6@@ns.dev.v.io:8101@@00000000000000000000000000000000@m@dev.v.io:service:mounttabled@@", nil},
		{"(dev.v.io:users:foo@bar.com)@ns.dev.v.io:8101", "@6@@ns.dev.v.io:8101@@00000000000000000000000000000000@m@dev.v.io:users:foo@bar.com@@", nil},
		{"(@1@tcp)@ns.dev.v.io:8101", "@6@@ns.dev.v.io:8101@@00000000000000000000000000000000@m@@1@tcp@@", nil},
	}
	runEndpointTests(t, testcases)
}

func TestParseHostPort(t *testing.T) {
	dns := Endpoint{
		Protocol:         "tcp",
		Address:          "batman.com:4444",
		ServesMountTable: true,
	}
	ipv4 := Endpoint{
		Protocol:         "tcp",
		Address:          "192.168.1.1:4444",
		ServesMountTable: true,
	}
	ipv6 := Endpoint{
		Protocol:         "tcp",
		Address:          "[01:02::]:4444",
		ServesMountTable: true,
	}
	testcases := []struct {
		Endpoint   Endpoint
		Host, Port string
	}{
		{dns, "batman.com", "4444"},
		{ipv4, "192.168.1.1", "4444"},
		{ipv6, "01:02::", "4444"},
	}

	for _, test := range testcases {
		addr := net.JoinHostPort(test.Host, test.Port)
		epString := FormatEndpoint("tcp", addr)
		if ep, err := ParseEndpoint(epString); err != nil {
			t.Errorf("NewEndpoint(%q) failed with %v", addr, err)
		} else if !reflect.DeepEqual(test.Endpoint, ep) {
			t.Errorf("Got endpoint %T = %#v, want %T = %#v for string %q", ep, ep, test.Endpoint, test.Endpoint, addr)
		}
	}
}

func TestEscapeEndpoint(t *testing.T) {
	defver := DefaultEndpointVersion
	defer func() {
		DefaultEndpointVersion = defver
	}()
	testcases := []Endpoint{
		{Protocol: "unix", Address: "@", RoutingID: FixedRoutingID(0xdabbad00)},
		{Protocol: "unix", Address: "@/%", RoutingID: FixedRoutingID(0xdabbad00)},
	}
	for i, ep := range testcases {
		epstr := ep.String()
		got, err := ParseEndpoint(epstr)
		if err != nil {
			t.Errorf("Test %d: NewEndpoint(%q) failed with %v", i, epstr, err)
			continue
		}
		if !reflect.DeepEqual(ep, got) {
			t.Errorf("Test %d: Got endpoint %#v, want %#v", i, got, ep)
		}
	}
}
