// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin freebsd linux netbsd openbsd

// Read system DNS config from /etc/resolv.conf

package dns

import (
	"bufio"
	"net"
	"os"
	"strconv"
	"strings"
)

type dnsConfig struct {
	servers  []string // servers to use
	search   []string // suffixes to append to local name
	ndots    int      // number of dots in name to trigger absolute lookup
	timeout  int      // seconds before giving up on packet
	attempts int      // lost packets before giving up on server
	rotate   bool     // round robin among servers
}

// See resolv.conf(5) on a Linux machine.
// TODO(rsc): Supposed to call uname() and chop the beginning
// of the host name to get the default search domain.
// We assume it's in resolv.conf anyway.
func dnsReadConfig() (*dnsConfig, error) {
	file, err := os.Open("/etc/resolv.conf")
	if err != nil {
		return nil, &ConfigError{err}
	}
	defer file.Close()

	conf := new(dnsConfig)
	conf.servers = make([]string, 3)[0:0] // small, but the standard limit
	conf.search = make([]string, 0)
	conf.ndots = 1
	conf.timeout = 5
	conf.attempts = 2
	conf.rotate = false
	b := bufio.NewReader(file)
	for {
		line, err := b.ReadString('\n')
		if err != nil {
			break
		}
		f := strings.Fields(line)
		if len(f) < 1 {
			continue
		}
		switch f[0] {
		case "nameserver": // add one name server
			a := conf.servers
			n := len(a)
			if len(f) > 1 && n < cap(a) {
				// One more check: make sure server name is
				// just an IP address.  Otherwise we need DNS
				// to look it up.
				name := f[1]
				switch len(net.ParseIP(name)) {
				case 16:
					name = "[" + name + "]"
					fallthrough
				case 4:
					a = a[0 : n+1]
					a[n] = name
					conf.servers = a
				}
			}

		case "domain": // set search path to just this domain
			if len(f) > 1 {
				conf.search = make([]string, 1)
				conf.search[0] = f[1]
			} else {
				conf.search = make([]string, 0)
			}

		case "search": // set search path to given servers
			conf.search = make([]string, len(f)-1)
			for i := 0; i < len(conf.search); i++ {
				conf.search[i] = f[i+1]
			}

		case "options": // magic options
			for i := 1; i < len(f); i++ {
				s := f[i]
				switch {
				case len(s) >= 6 && s[0:6] == "ndots:":
					n, _ := strconv.Atoi(s[6:])
					if n < 1 {
						n = 1
					}
					conf.ndots = n
				case len(s) >= 8 && s[0:8] == "timeout:":
					n, _ := strconv.Atoi(s[8:])
					if n < 1 {
						n = 1
					}
					conf.timeout = n
				case len(s) >= 8 && s[0:9] == "attempts:":
					n, _ := strconv.Atoi(s[9:])
					if n < 1 {
						n = 1
					}
					conf.attempts = n
				case s == "rotate":
					conf.rotate = true
				}
			}
		}
	}

	return conf, nil
}

// ConfigError represents an error reading the machine's DNS configuration.
type ConfigError struct {
	Err error
}

func (e *ConfigError) Error() string {
	return "error reading DNS config: " + e.Err.Error()
}

func (e *ConfigError) Timeout() bool   { return false }
func (e *ConfigError) Temporary() bool { return false }
