// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build darwin freebsd linux netbsd openbsd

package dns

import (
	"net"
)

// LookupSRV tries to resolve an SRV query of the given service,
// protocol, and domain name.  The proto is "tcp" or "udp".
// The returned records are sorted by priority and randomized
// by weight within a priority.
//
// LookupSRV constructs the DNS name to look up following RFC 2782.
// That is, it looks up _service._proto.name.  To accommodate services
// publishing SRV records under non-standard names, if both service
// and proto are empty strings, LookupSRV looks up name directly.
func LookupSRV(service, proto, name string) (cname string, addrs []*net.SRV, err error) {
	var target string
	if service == "" && proto == "" {
		target = name
	} else {
		target = "_" + service + "._" + proto + "." + name
	}
	var records []RR
	cname, records, err = lookup(target, TypeSRV)
	if err != nil {
		return
	}
	addrs = make([]*net.SRV, len(records))
	for i, rr := range records {
		r := rr.(*RR_SRV)
		addrs[i] = &net.SRV{r.Target, r.Port, r.Priority, r.Weight}
	}
	byPriorityWeight(addrs).sort()
	return
}

// LookupMX returns the DNS MX records for the given domain name sorted by preference.
func LookupMX(name string) (mx []*net.MX, err error) {
	_, records, err := lookup(name, TypeMX)
	if err != nil {
		return
	}
	mx = make([]*net.MX, len(records))
	for i, rr := range records {
		r := rr.(*RR_MX)
		mx[i] = &net.MX{r.Mx, r.Pref}
	}
	byPref(mx).sort()
	return
}

// LookupTXT returns the DNS TXT records for the given domain name.
func LookupTXT(name string) (txt []string, err error) {
	_, records, err := lookup(name, TypeTXT)
	if err != nil {
		return
	}
	for _, r := range records {
		txt = append(txt, r.(*RR_TXT).Txt...)
	}
	return
}

// LookupAddr performs a reverse lookup for the given address, returning a list
// of names mapping to that address.
func LookupAddr(addr string) (name []string, err error) {
	name = lookupStaticAddr(addr)
	if len(name) > 0 {
		return
	}
	var arpa string
	arpa, err = reverseaddr(addr)
	if err != nil {
		return
	}
	var records []RR
	_, records, err = lookup(arpa, TypePTR)
	if err != nil {
		return
	}
	name = make([]string, len(records))
	for i := range records {
		r := records[i].(*RR_PTR)
		name[i] = r.Ptr
	}
	return
}
