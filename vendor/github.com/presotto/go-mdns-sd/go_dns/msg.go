// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// DNS packet assembly.  See RFC 1035.
//
// This is intended to support name resolution during Dial.
// It doesn't have to be blazing fast.
//
// Each message structure has a Walk method that is used by
// a generic pack/unpack routine. Thus, if in the future we need
// to define new message structs, no new pack/unpack/printing code
// needs to be written.
//
// The first half of this file defines the DNS message formats.
// The second half implements the conversion to and from wire format.
// A few of the structure elements have string tags to aid the
// generic pack/unpack routines.
//
// TODO(rsc):  There are enough names defined in this file that they're all
// prefixed with dns.  Perhaps put this in its own package later.

package dns

import (
	"net"
	"strconv"
)

// Packet formats

// Wire constants.
const (
	// valid RR_Header.Rrtype and Question.qtype
	TypeA     = 1
	TypeNS    = 2
	TypeMD    = 3
	TypeMF    = 4
	TypeCNAME = 5
	TypeSOA   = 6
	TypeMB    = 7
	TypeMG    = 8
	TypeMR    = 9
	TypeNULL  = 10
	TypeWKS   = 11
	TypePTR   = 12
	TypeHINFO = 13
	TypeMINFO = 14
	TypeMX    = 15
	TypeTXT   = 16
	TypeAAAA  = 28
	TypeSRV   = 33

	// valid Question.qtype only
	TypeAXFR  = 252
	TypeMAILB = 253
	TypeMAILA = 254
	TypeALL   = 255

	// valid Question.qclass
	ClassINET   = 1
	ClassCSNET  = 2
	ClassCHAOS  = 3
	ClassHESIOD = 4
	ClassANY    = 255

	// Msg.rcode
	RcodeSuccess        = 0
	RcodeFormatError    = 1
	RcodeServerFailure  = 2
	RcodeNameError      = 3
	RcodeNotImplemented = 4
	RcodeRefused        = 5
)

// A dnsStruct describes how to iterate over its fields to emulate
// reflective marshalling.
type dnsStruct interface {
	// Walk iterates over fields of a structure and calls f
	// with a reference to that field, the name of the field
	// and a tag ("", "domain", "ipv4", "ipv6") specifying
	// particular encodings. Possible concrete types
	// for v are *uint16, *uint32, *string, or []byte, and
	// *int, *bool in the case of MsgHdr.
	// Whenever f returns false, Walk must stop and return
	// false, and otherwise return true.
	Walk(f func(v interface{}, name, tag string) (ok bool)) (ok bool)
}

// The wire format for the DNS packet header.
type dnsHeader struct {
	Id                                 uint16
	Bits                               uint16
	Qdcount, Ancount, Nscount, Arcount uint16
}

func (h *dnsHeader) Walk(f func(v interface{}, name, tag string) bool) bool {
	return f(&h.Id, "Id", "") &&
		f(&h.Bits, "Bits", "") &&
		f(&h.Qdcount, "Qdcount", "") &&
		f(&h.Ancount, "Ancount", "") &&
		f(&h.Nscount, "Nscount", "") &&
		f(&h.Arcount, "Arcount", "")
}

const (
	// dnsHeader.Bits
	_QR = 1 << 15 // query/response (response=1)
	_AA = 1 << 10 // authoritative
	_TC = 1 << 9  // truncated
	_RD = 1 << 8  // recursion desired
	_RA = 1 << 7  // recursion available
)

// DNS queries.
type Question struct {
	Name   string `net:"domain-name"` // `net:"domain-name"` specifies encoding; see packers below
	Qtype  uint16
	Qclass uint16
}

func (q *Question) Walk(f func(v interface{}, name, tag string) bool) bool {
	return f(&q.Name, "Name", "domain") &&
		f(&q.Qtype, "Qtype", "") &&
		f(&q.Qclass, "Qclass", "")
}

// DNS responses (resource records).
// There are many types of messages,
// but they all share the same header.
type RR_Header struct {
	Name     string `net:"domain-name"`
	Rrtype   uint16
	Class    uint16
	Ttl      uint32
	Rdlength uint16 // length of data after header
}

func (h *RR_Header) Header() *RR_Header {
	return h
}

func (h *RR_Header) Walk(f func(v interface{}, name, tag string) bool) bool {
	return f(&h.Name, "Name", "domain") &&
		f(&h.Rrtype, "Rrtype", "") &&
		f(&h.Class, "Class", "") &&
		f(&h.Ttl, "Ttl", "") &&
		f(&h.Rdlength, "Rdlength", "")
}

type RR interface {
	dnsStruct
	Header() *RR_Header
}

// Specific DNS RR formats for each query type.

type RR_CNAME struct {
	Hdr   RR_Header
	Cname string `net:"domain-name"`
}

func (rr *RR_CNAME) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_CNAME) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) && f(&rr.Cname, "Cname", "domain")
}

type RR_HINFO struct {
	Hdr RR_Header
	Cpu string
	Os  string
}

func (rr *RR_HINFO) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_HINFO) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) && f(&rr.Cpu, "Cpu", "") && f(&rr.Os, "Os", "")
}

type RR_MB struct {
	Hdr RR_Header
	Mb  string `net:"domain-name"`
}

func (rr *RR_MB) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_MB) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) && f(&rr.Mb, "Mb", "domain")
}

type RR_MG struct {
	Hdr RR_Header
	Mg  string `net:"domain-name"`
}

func (rr *RR_MG) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_MG) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) && f(&rr.Mg, "Mg", "domain")
}

type RR_MINFO struct {
	Hdr   RR_Header
	Rmail string `net:"domain-name"`
	Email string `net:"domain-name"`
}

func (rr *RR_MINFO) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_MINFO) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) && f(&rr.Rmail, "Rmail", "domain") && f(&rr.Email, "Email", "domain")
}

type RR_MR struct {
	Hdr RR_Header
	Mr  string `net:"domain-name"`
}

func (rr *RR_MR) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_MR) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) && f(&rr.Mr, "Mr", "domain")
}

type RR_MX struct {
	Hdr  RR_Header
	Pref uint16
	Mx   string `net:"domain-name"`
}

func (rr *RR_MX) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_MX) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) && f(&rr.Pref, "Pref", "") && f(&rr.Mx, "Mx", "domain")
}

type RR_NS struct {
	Hdr RR_Header
	Ns  string `net:"domain-name"`
}

func (rr *RR_NS) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_NS) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) && f(&rr.Ns, "Ns", "domain")
}

type RR_PTR struct {
	Hdr RR_Header
	Ptr string `net:"domain-name"`
}

func (rr *RR_PTR) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_PTR) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) && f(&rr.Ptr, "Ptr", "domain")
}

type RR_SOA struct {
	Hdr     RR_Header
	Ns      string `net:"domain-name"`
	Mbox    string `net:"domain-name"`
	Serial  uint32
	Refresh uint32
	Retry   uint32
	Expire  uint32
	Minttl  uint32
}

func (rr *RR_SOA) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_SOA) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) &&
		f(&rr.Ns, "Ns", "domain") &&
		f(&rr.Mbox, "Mbox", "domain") &&
		f(&rr.Serial, "Serial", "") &&
		f(&rr.Refresh, "Refresh", "") &&
		f(&rr.Retry, "Retry", "") &&
		f(&rr.Expire, "Expire", "") &&
		f(&rr.Minttl, "Minttl", "")
}

type RR_TXT struct {
	Hdr RR_Header
	Txt []string
}

func (rr *RR_TXT) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_TXT) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) && f(&rr.Txt, "Txt", "")
}

type RR_SRV struct {
	Hdr      RR_Header
	Priority uint16
	Weight   uint16
	Port     uint16
	Target   string `net:"domain-name"`
}

func (rr *RR_SRV) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_SRV) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) &&
		f(&rr.Priority, "Priority", "") &&
		f(&rr.Weight, "Weight", "") &&
		f(&rr.Port, "Port", "") &&
		f(&rr.Target, "Target", "domain")
}

type RR_A struct {
	Hdr RR_Header
	A   uint32 `net:"ipv4"`
}

func (rr *RR_A) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_A) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) && f(&rr.A, "A", "ipv4")
}

type RR_AAAA struct {
	Hdr  RR_Header
	AAAA [16]byte `net:"ipv6"`
}

func (rr *RR_AAAA) Header() *RR_Header {
	return &rr.Hdr
}

func (rr *RR_AAAA) Walk(f func(v interface{}, name, tag string) bool) bool {
	return rr.Hdr.Walk(f) && f(rr.AAAA[:], "AAAA", "ipv6")
}

// Packing and unpacking.
//
// All the packers and unpackers take a (msg []byte, off int)
// and return (off1 int, ok bool).  If they return ok==false, they
// also return off1==len(msg), so that the next unpacker will
// also fail.  This lets us avoid checks of ok until the end of a
// packing sequence.

// Map of constructors for each RR wire type.
var rr_mk = map[int]func() RR{
	TypeCNAME: func() RR { return new(RR_CNAME) },
	TypeHINFO: func() RR { return new(RR_HINFO) },
	TypeMB:    func() RR { return new(RR_MB) },
	TypeMG:    func() RR { return new(RR_MG) },
	TypeMINFO: func() RR { return new(RR_MINFO) },
	TypeMR:    func() RR { return new(RR_MR) },
	TypeMX:    func() RR { return new(RR_MX) },
	TypeNS:    func() RR { return new(RR_NS) },
	TypePTR:   func() RR { return new(RR_PTR) },
	TypeSOA:   func() RR { return new(RR_SOA) },
	TypeTXT:   func() RR { return new(RR_TXT) },
	TypeSRV:   func() RR { return new(RR_SRV) },
	TypeA:     func() RR { return new(RR_A) },
	TypeAAAA:  func() RR { return new(RR_AAAA) },
}

// Pack a domain name s into msg[off:].
// Domain names are a sequence of counted strings
// split at the dots.  They end with a zero-length string.
func packDomainName(s string, msg []byte, off int) (off1 int, ok bool) {
	// Add trailing dot to canonicalize name.
	if n := len(s); n == 0 || s[n-1] != '.' {
		s += "."
	}

	// Each dot ends a segment of the name.
	// We trade each dot byte for a length byte.
	// There is also a trailing zero.
	// Check that we have all the space we need.
	tot := len(s) + 1
	if off+tot > len(msg) {
		return len(msg), false
	}

	// Emit sequence of counted strings, chopping at dots.
	begin := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			if i-begin >= 1<<6 { // top two bits of length must be clear
				return len(msg), false
			}
			msg[off] = byte(i - begin)
			off++
			for j := begin; j < i; j++ {
				msg[off] = s[j]
				off++
			}
			begin = i + 1
		}
	}
	msg[off] = 0
	off++
	return off, true
}

// Unpack a domain name.
// In addition to the simple sequences of counted strings above,
// domain names are allowed to refer to strings elsewhere in the
// packet, to avoid repeating common suffixes when returning
// many entries in a single domain.  The pointers are marked
// by a length byte with the top two bits set.  Ignoring those
// two bits, that byte and the next give a 14 bit offset from msg[0]
// where we should pick up the trail.
// Note that if we jump elsewhere in the packet,
// we return off1 == the offset after the first pointer we found,
// which is where the next record will start.
// In theory, the pointers are only allowed to jump backward.
// We let them jump anywhere and stop jumping after a while.
func unpackDomainName(msg []byte, off int) (s string, off1 int, ok bool) {
	s = ""
	ptr := 0 // number of pointers followed
Loop:
	for {
		if off >= len(msg) {
			return "", len(msg), false
		}
		c := int(msg[off])
		off++
		switch c & 0xC0 {
		case 0x00:
			if c == 0x00 {
				// end of name
				break Loop
			}
			// literal string
			if off+c > len(msg) {
				return "", len(msg), false
			}
			s += string(msg[off:off+c]) + "."
			off += c
		case 0xC0:
			// pointer to somewhere else in msg.
			// remember location after first ptr,
			// since that's how many bytes we consumed.
			// also, don't follow too many pointers --
			// maybe there's a loop.
			if off >= len(msg) {
				return "", len(msg), false
			}
			c1 := msg[off]
			off++
			if ptr == 0 {
				off1 = off
			}
			if ptr++; ptr > 10 {
				return "", len(msg), false
			}
			off = (c^0xC0)<<8 | int(c1)
		default:
			// 0x80 and 0x40 are reserved
			return "", len(msg), false
		}
	}
	if ptr == 0 {
		off1 = off
	}
	return s, off1, true
}

// packStruct packs a structure into msg at specified offset off, and
// returns off1 such that msg[off:off1] is the encoded data.
func packStruct(any dnsStruct, msg []byte, off int) (off1 int, ok bool) {
	ok = any.Walk(func(field interface{}, name, tag string) bool {
		switch fv := field.(type) {
		default:
			println("net: dns: unknown packing type")
			return false
		case *uint16:
			i := *fv
			if off+2 > len(msg) {
				return false
			}
			msg[off] = byte(i >> 8)
			msg[off+1] = byte(i)
			off += 2
		case *uint32:
			i := *fv
			msg[off] = byte(i >> 24)
			msg[off+1] = byte(i >> 16)
			msg[off+2] = byte(i >> 8)
			msg[off+3] = byte(i)
			off += 4
		case []byte:
			n := len(fv)
			if off+n > len(msg) {
				return false
			}
			copy(msg[off:off+n], fv)
			off += n
		case *string:
			s := *fv
			switch tag {
			default:
				println("net: dns: unknown string tag", tag)
				return false
			case "domain":
				off, ok = packDomainName(s, msg, off)
				if !ok {
					return false
				}
			case "":
				// Counted string: 1 byte length.
				if len(s) > 255 || off+1+len(s) > len(msg) {
					return false
				}
				msg[off] = byte(len(s))
				off++
				off += copy(msg[off:], s)
			}
		case *[]string:
			// Pack the strings back to back.
			if *fv == nil {
				return false
			}
			for _, s := range *fv {
				// Counted string: 1 byte length.
				if len(s) > 255 || off+1+len(s) > len(msg) {
					return false
				}
				msg[off] = byte(len(s))
				off++
				off += copy(msg[off:], s)
			}
		}
		return true
	})
	if !ok {
		return len(msg), false
	}
	return off, true
}

// unpackStruct decodes msg[off:] into the given structure, and
// returns off1 such that msg[off:off1] is the encoded data.
func unpackStruct(any dnsStruct, msg []byte, off int) (off1 int, ok bool) {
	ok = any.Walk(func(field interface{}, name, tag string) bool {
		switch fv := field.(type) {
		default:
			println("net: dns: unknown packing type")
			return false
		case *uint16:
			if off+2 > len(msg) {
				return false
			}
			*fv = uint16(msg[off])<<8 | uint16(msg[off+1])
			off += 2
		case *uint32:
			if off+4 > len(msg) {
				return false
			}
			*fv = uint32(msg[off])<<24 | uint32(msg[off+1])<<16 |
				uint32(msg[off+2])<<8 | uint32(msg[off+3])
			off += 4
		case []byte:
			n := len(fv)
			if off+n > len(msg) {
				return false
			}
			copy(fv, msg[off:off+n])
			off += n
		case *string:
			var s string
			switch tag {
			default:
				println("net: dns: unknown string tag", tag)
				return false
			case "domain":
				s, off, ok = unpackDomainName(msg, off)
				if !ok {
					return false
				}
			case "":
				if off >= len(msg) || off+1+int(msg[off]) > len(msg) {
					return false
				}
				n := int(msg[off])
				off++
				s = string(msg[off : off+n])
				off += n
			}
			*fv = s
		case *[]string:
			for off != len(msg) {
				if off > len(msg) || off+1+int(msg[off]) > len(msg) {
					return false
				}
				n := int(msg[off])
				off++
				*fv = append(*fv, string(msg[off:off+n]))
				off += n
			}
			if *fv == nil {
				return false
			}
		}
		return true
	})
	if !ok {
		return len(msg), false
	}
	return off, true
}

// Generic struct printer. Prints fields with tag "ipv4" or "ipv6"
// as IP addresses.
func printStruct(any dnsStruct) string {
	s := "{"
	i := 0
	any.Walk(func(val interface{}, name, tag string) bool {
		i++
		if i > 1 {
			s += ", "
		}
		s += name + "="
		switch tag {
		case "ipv4":
			i := *val.(*uint32)
			s += net.IPv4(byte(i>>24), byte(i>>16), byte(i>>8), byte(i)).String()
		case "ipv6":
			i := val.([]byte)
			s += net.IP(i).String()
		default:
			var i int64
			switch v := val.(type) {
			default:
				// can't really happen.
				s += "<unknown type>"
				return true
			case *string:
				s += *v
				return true
			case *[]string:
				for _, x := range *v {
					s += x + " "
				}
				return true
			case []byte:
				s += string(v)
				return true
			case *bool:
				if *v {
					s += "true"
				} else {
					s += "false"
				}
				return true
			case *int:
				i = int64(*v)
			case *uint:
				i = int64(*v)
			case *uint8:
				i = int64(*v)
			case *uint16:
				i = int64(*v)
			case *uint32:
				i = int64(*v)
			case *uint64:
				i = int64(*v)
			case *uintptr:
				i = int64(*v)
			}
			s += strconv.Itoa(int(i))
		}
		return true
	})
	s += "}"
	return s
}

// Resource record packer.
func packRR(rr RR, msg []byte, off int) (off2 int, ok bool) {
	var off1 int
	// pack twice, once to find end of header
	// and again to find end of packet.
	// a bit inefficient but this doesn't need to be fast.
	// off1 is end of header
	// off2 is end of rr
	off1, ok = packStruct(rr.Header(), msg, off)
	off2, ok = packStruct(rr, msg, off)
	if !ok {
		return len(msg), false
	}
	// pack a third time; redo header with correct data length
	rr.Header().Rdlength = uint16(off2 - off1)
	packStruct(rr.Header(), msg, off)
	return off2, true
}

// Resource record unpacker.
func unpackRR(msg []byte, off int) (rr RR, off1 int, ok bool) {
	// unpack just the header, to find the rr type and length
	var h RR_Header
	off0 := off
	if off, ok = unpackStruct(&h, msg, off); !ok {
		return nil, len(msg), false
	}
	end := off + int(h.Rdlength)

	// make a slice ending at the end of the RR so that unpacking
	// can't overflow the RR
	if end < len(msg) {
		msg = msg[:end]
	}

	// make an rr of that type and re-unpack.
	// again inefficient but doesn't need to be fast.
	mk, known := rr_mk[int(h.Rrtype)]
	if !known {
		return &h, end, true
	}
	rr = mk()
	off, ok = unpackStruct(rr, msg, off0)
	if off != end {
		return &h, end, true
	}
	return rr, off, ok
}

// Usable representation of a DNS packet.

// A manually-unpacked version of (id, bits).
// This is in its own struct for easy printing.
type MsgHdr struct {
	ID                 uint16
	Response           bool
	Opcode             int
	Authoritative      bool
	Truncated          bool
	RecursionDesired   bool
	RecursionAvailable bool
	Rcode              int
}

func (h *MsgHdr) Walk(f func(v interface{}, name, tag string) bool) bool {
	return f(&h.ID, "id", "") &&
		f(&h.Response, "response", "") &&
		f(&h.Opcode, "opcode", "") &&
		f(&h.Authoritative, "authoritative", "") &&
		f(&h.Truncated, "truncated", "") &&
		f(&h.RecursionDesired, "recursion_desired", "") &&
		f(&h.RecursionAvailable, "recursion_available", "") &&
		f(&h.Rcode, "rcode", "")
}

type Msg struct {
	MsgHdr
	Question []Question
	Answer   []RR
	NS       []RR
	Extra    []RR
}

func (dns *Msg) Pack() (msg []byte, ok bool) {
	var dh dnsHeader

	// Convert convenient Msg into wire-like dnsHeader.
	dh.Id = dns.ID
	dh.Bits = uint16(dns.Opcode)<<11 | uint16(dns.Rcode)
	if dns.RecursionAvailable {
		dh.Bits |= _RA
	}
	if dns.RecursionDesired {
		dh.Bits |= _RD
	}
	if dns.Truncated {
		dh.Bits |= _TC
	}
	if dns.Authoritative {
		dh.Bits |= _AA
	}
	if dns.Response {
		dh.Bits |= _QR
	}

	// Prepare variable sized arrays.
	question := dns.Question
	answer := dns.Answer
	ns := dns.NS
	extra := dns.Extra

	dh.Qdcount = uint16(len(question))
	dh.Ancount = uint16(len(answer))
	dh.Nscount = uint16(len(ns))
	dh.Arcount = uint16(len(extra))

	// Could work harder to calculate message size,
	// but this is far more than we need and not
	// big enough to hurt the allocator.
	msg = make([]byte, 2000)

	// Pack it in: header and then the pieces.
	off := 0
	off, ok = packStruct(&dh, msg, off)
	for i := 0; i < len(question); i++ {
		off, ok = packStruct(&question[i], msg, off)
	}
	for i := 0; i < len(answer); i++ {
		off, ok = packRR(answer[i], msg, off)
	}
	for i := 0; i < len(ns); i++ {
		off, ok = packRR(ns[i], msg, off)
	}
	for i := 0; i < len(extra); i++ {
		off, ok = packRR(extra[i], msg, off)
	}
	if !ok {
		return nil, false
	}
	return msg[0:off], true
}

func (dns *Msg) Unpack(msg []byte) bool {
	// Header.
	var dh dnsHeader
	off := 0
	var ok bool
	if off, ok = unpackStruct(&dh, msg, off); !ok {
		return false
	}
	dns.ID = dh.Id
	dns.Response = (dh.Bits & _QR) != 0
	dns.Opcode = int(dh.Bits>>11) & 0xF
	dns.Authoritative = (dh.Bits & _AA) != 0
	dns.Truncated = (dh.Bits & _TC) != 0
	dns.RecursionDesired = (dh.Bits & _RD) != 0
	dns.RecursionAvailable = (dh.Bits & _RA) != 0
	dns.Rcode = int(dh.Bits & 0xF)

	// Arrays.
	dns.Question = make([]Question, dh.Qdcount)
	dns.Answer = make([]RR, 0, dh.Ancount)
	dns.NS = make([]RR, 0, dh.Nscount)
	dns.Extra = make([]RR, 0, dh.Arcount)

	var rec RR

	for i := 0; i < len(dns.Question); i++ {
		off, ok = unpackStruct(&dns.Question[i], msg, off)
	}
	for i := 0; i < int(dh.Ancount); i++ {
		rec, off, ok = unpackRR(msg, off)
		if !ok {
			return false
		}
		dns.Answer = append(dns.Answer, rec)
	}
	for i := 0; i < int(dh.Nscount); i++ {
		rec, off, ok = unpackRR(msg, off)
		if !ok {
			return false
		}
		dns.NS = append(dns.NS, rec)
	}
	for i := 0; i < int(dh.Arcount); i++ {
		rec, off, ok = unpackRR(msg, off)
		if !ok {
			return false
		}
		dns.Extra = append(dns.Extra, rec)
	}
	//	if off != len(msg) {
	//		println("extra bytes in dns packet", off, "<", len(msg));
	//	}
	return true
}

func (dns *Msg) String() string {
	s := "DNS: " + printStruct(&dns.MsgHdr) + "\n"
	if len(dns.Question) > 0 {
		s += "-- Questions\n"
		for i := 0; i < len(dns.Question); i++ {
			s += printStruct(&dns.Question[i]) + "\n"
		}
	}
	if len(dns.Answer) > 0 {
		s += "-- Answers\n"
		for i := 0; i < len(dns.Answer); i++ {
			s += printStruct(dns.Answer[i]) + "\n"
		}
	}
	if len(dns.NS) > 0 {
		s += "-- Name servers\n"
		for i := 0; i < len(dns.NS); i++ {
			s += printStruct(dns.NS[i]) + "\n"
		}
	}
	if len(dns.Extra) > 0 {
		s += "-- Extra\n"
		for i := 0; i < len(dns.Extra); i++ {
			s += printStruct(dns.Extra[i]) + "\n"
		}
	}
	return s
}
