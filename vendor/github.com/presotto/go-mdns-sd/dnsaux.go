// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package mdns

// Helper routines for creating/manipulating dns messages.

import (
	"github.com/presotto/go-mdns-sd/go_dns"
	"net"
)

// Create a new (unpacked) dns message and initialize the fields.
func newDnsMsg(id uint16, response, authoritative bool) *dns.Msg {
	msg := new(dns.Msg)
	msg.ID = id
	msg.Response = response
	msg.Authoritative = authoritative
	msg.Question = make([]dns.Question, 0)
	msg.Answer = make([]dns.RR, 0, 0)
	msg.NS = make([]dns.RR, 0, 0)
	msg.Extra = make([]dns.RR, 0, 0)
	return msg
}

// Returns an A or AAAA RR, whichever is appropriate for the passed in address.
func NewAddressRR(name string, class uint16, ttl uint32, ip net.IP) dns.RR {
	var rr dns.RR
	if v4 := ip.To4(); v4 != nil {
		rra := new(dns.RR_A)
		rra.A = (uint32(v4[0]) << 24) | (uint32(v4[1]) << 16) | (uint32(v4[2]) << 8) | uint32(v4[3])
		rra.Header().Rrtype = dns.TypeA
		rr = rra
	} else {
		rraaaa := new(dns.RR_AAAA)
		copy(rraaaa.AAAA[:], ip)
		rraaaa.Header().Rrtype = dns.TypeAAAA
		rr = rraaaa
	}
	rr.Header().Name = name
	rr.Header().Class = class
	rr.Header().Ttl = ttl
	return rr
}

// Returns a SRV RR.
func NewSrvRR(name string, class uint16, ttl uint32, target string, port, priority, weight uint16) dns.RR {
	return &dns.RR_SRV{dns.RR_Header{name, dns.TypeSRV, class, ttl, 0}, priority, weight, port, target}
}

// Returns a TXT RR.  This is a limited TXT RR that can contain only one string
func NewTxtRR(name string, class uint16, ttl uint32, txt []string) dns.RR {
	if txt == nil {
		txt = []string{""}
	}
	return &dns.RR_TXT{dns.RR_Header{name, dns.TypeTXT, class, ttl, 0}, txt}
}

// Returns a PTR RR.
func NewPtrRR(name string, class uint16, ttl uint32, ptr string) dns.RR {
	return &dns.RR_PTR{dns.RR_Header{name, dns.TypePTR, class, ttl, 0}, ptr}
}

// Convert an A RR into a net.IP
func AtoIP(rr *dns.RR_A) net.IP {
	ip := make([]byte, 4)
	ip[0] = byte(rr.A >> 24)
	ip[1] = byte(rr.A >> 16)
	ip[2] = byte(rr.A >> 8)
	ip[3] = byte(rr.A)
	return ip
}

// Convert an AAAA RR into a net.IP
func AAAAtoIP(rr *dns.RR_AAAA) net.IP {
	ip := make([]byte, 16)
	copy(ip, rr.AAAA[:])
	return ip
}
