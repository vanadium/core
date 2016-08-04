// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package flags_test

import (
	"net"
	"reflect"
	"strings"
	"testing"

	"v.io/x/ref/lib/flags"
)

func TestIPFlag(t *testing.T) {
	ip := &flags.IPFlag{}
	if err := ip.Set("172.16.1.22"); err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if got, want := ip.IP, net.ParseIP("172.16.1.22"); !got.Equal(want) {
		t.Errorf("got %s, expected %s", got, want)
	}
	if err := ip.Set("172.16"); err == nil || !strings.Contains(err.Error(), "failed to parse 172.16 as an IP address") {
		t.Errorf("expected error %v", err)
	}
}

func TestTCPFlag(t *testing.T) {
	tcp := &flags.TCPProtocolFlag{}
	if err := tcp.Set("tcp6"); err != nil {
		t.Errorf("unexpected error %s", err)
	}
	if got, want := tcp.Protocol, "tcp6"; got != want {
		t.Errorf("got %s, expected %s", got, want)
	}
	if err := tcp.Set("foo"); err == nil || !strings.Contains(err.Error(), "not a tcp protocol") {
		t.Errorf("expected error %v", err)
	}
}

func TestIPHostPortFlag(t *testing.T) {
	lh := []*net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}
	ip6 := []*net.IPAddr{{IP: net.ParseIP("FE80:0000:0000:0000:0202:B3FF:FE1E:8329")}}
	cases := []struct {
		input string
		want  flags.IPHostPortFlag
		str   string
	}{
		{"", flags.IPHostPortFlag{Port: ""}, ""},
		{":0", flags.IPHostPortFlag{Port: "0"}, ":0"},
		{":22", flags.IPHostPortFlag{Port: "22"}, ":22"},
		{"127.0.0.1", flags.IPHostPortFlag{IP: lh, Port: "0"}, "127.0.0.1:0"},
		{"127.0.0.1:10", flags.IPHostPortFlag{IP: lh, Port: "10"}, "127.0.0.1:10"},
		{"[]:0", flags.IPHostPortFlag{Port: "0"}, ":0"},
		{"[FE80:0000:0000:0000:0202:B3FF:FE1E:8329]:100", flags.IPHostPortFlag{IP: ip6, Port: "100"}, "[fe80::202:b3ff:fe1e:8329]:100"},
	}
	for _, c := range cases {
		got, want := &flags.IPHostPortFlag{}, &c.want
		c.want.Address = c.input
		if err := got.Set(c.input); err != nil || !reflect.DeepEqual(got, want) {
			if err != nil {
				t.Errorf("%q: unexpected error %s", c.input, err)
			} else {
				t.Errorf("%q: got %#v, want %#v", c.input, got, want)
			}
		}
		if got.String() != c.str {
			t.Errorf("%q: got %#v, want %#v", c.input, got.String(), c.str)
		}
	}

	host := &flags.IPHostPortFlag{}
	if err := host.Set("localhost:122"); err != nil {
		t.Errorf("unexpected error: %s", err)
	}
	if len(host.IP) == 0 {
		t.Errorf("localhost should have resolved to at least one address")
	}
	if got, want := host.Port, "122"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := host.String(), "localhost:122"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	for _, s := range []string{":", ":59999999", "nohost.invalid", "nohost.invalid:"} {
		f := &flags.IPHostPortFlag{}
		if err := f.Set(s); err == nil {
			t.Errorf("expected an error for %q, %#v", s, f)
		}
	}
}
