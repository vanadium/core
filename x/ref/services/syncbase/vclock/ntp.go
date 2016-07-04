// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package vclock

// This file contains interfaces and functions for talking to an NTP server.

import (
	"fmt"
	"net"
	"time"

	"v.io/x/lib/vlog"
)

type NtpData struct {
	// NTP time minus system clock time. True UTC time can be estimated as system
	// time plus skew.
	// Note: NTP documentation typically refers to this value as "offset".
	skew time.Duration

	// Round-trip network delay experienced when talking to NTP server. The
	// smaller the delay, the more accurate the skew.
	delay time.Duration

	// Transmission timestamp from NTP server. Note, this value does not reflect
	// round-trip time.
	ntpTs time.Time
}

type NtpSource interface {
	// NtpSync obtains 'sampleCount' samples of NtpData from an NTP server and
	// returns the one with the smallest 'delay' value.
	NtpSync(sampleCount int) (*NtpData, error)
}

// newVClockNtpSource returns a new NtpSource implementation that talks to the
// specified ntpHost.
func newVClockNtpSource(ntpHost string, vclock *VClock) NtpSource {
	return &ntpSourceImpl{ntpHost, vclock}
}

// Note: ntpSourceImpl only uses a few methods from *VClock, namely
// SysClockVals{,IfNotChanged}. However, we want ntpSourceImpl to see the
// effects of calls to VClock.InjectFakeSysClock. The current approach is not
// particularly elegant, but it works, and the mess is fully contained within
// the vclock package. Shimming doesn't seem worthwhile.
type ntpSourceImpl struct {
	ntpHost string
	vclock  *VClock
}

var _ NtpSource = (*ntpSourceImpl)(nil)

func (ns *ntpSourceImpl) NtpSync(sampleCount int) (*NtpData, error) {
	if ns.ntpHost == "" {
		return nil, fmt.Errorf("vclock: NtpSync: no NTP server")
	}
	var res *NtpData
	for i := 0; i < sampleCount; i++ {
		if sample, err := ns.sample(); err == nil {
			if (res == nil) || (sample.delay < res.delay) {
				res = sample
			}
		}
	}
	if res == nil {
		return nil, fmt.Errorf("vclock: NtpSync: failed to get sample from NTP server: %s", ns.ntpHost)
	}
	return res, nil
}

////////////////////////////////////////////////////////////////////////////////
// Implementation of NtpSync

// sample sends a single request to the NTP server and returns the resulting
// NtpData.
//
// The NTP protocol involves sending a request of size 48 bytes, with the first
// byte containing protocol version and mode, and the last 8 bytes containing
// transmit timestamp. The current NTP version is 4. A response from NTP server
// contains original timestamp (client's transmit timestamp from request) from
// bytes 24 to 31, server's receive timestamp from byte 32 to 39 and server's
// transmit time from byte 40 to 47. Client can register the response receive
// time as soon it receives a response from server.
// Based on the 4 timestamps the client can compute the skew between the
// two vclocks and the roundtrip network delay for the request.
func (ns *ntpSourceImpl) sample() (*NtpData, error) {
	raddr, err := net.ResolveUDPAddr("udp", ns.ntpHost)
	if err != nil {
		return nil, err
	}

	con, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return nil, err
	}
	defer con.Close()

	// For detecting sysclock changes that would break the NTP send/recv time
	// calculations. The system clock's elapsed time cannot be modified by users,
	// so to detect sysclock changes we compare the change in elapsed time to the
	// change in sysclock time. If we detect a sysclock change, our send and recv
	// timestamps are not comparable, so we return an error.
	origNow, origElapsedTime, err := ns.vclock.SysClockVals()
	if err != nil {
		vlog.Errorf("vclock: NtpSync: SysClockVals failed: %v", err)
		return nil, err
	}

	msg := ns.createRequest(origNow)
	_, err = con.Write(msg)
	if err != nil {
		return nil, err
	}

	con.SetDeadline(time.Now().Add(5 * time.Second))
	_, err = con.Read(msg)
	if err != nil {
		return nil, err
	}

	// Check for sysclock changes.
	recvNow, _, err := ns.vclock.SysClockValsIfNotChanged(origNow, origElapsedTime)
	if err != nil {
		vlog.Errorf("vclock: NtpSync: SysClockValsIfNotChanged failed: %v", err)
		return nil, err
	}

	clientTransmitTs := extractTime(msg[24:32])
	serverReceiveTs := extractTime(msg[32:40])
	serverTransmitTs := extractTime(msg[40:48])
	clientReceiveTs := recvNow

	// Following code extracts the vclock skew and network delay based on the
	// transmit and receive timestamps on the client and the server as per
	// the formula explained at http://www.eecis.udel.edu/~mills/time.html
	data := NtpData{}
	data.skew = (serverReceiveTs.Sub(clientTransmitTs) + serverTransmitTs.Sub(clientReceiveTs)) / 2
	data.delay = clientReceiveTs.Sub(clientTransmitTs) - serverTransmitTs.Sub(serverReceiveTs)
	data.ntpTs = serverTransmitTs
	return &data, nil
}

func (ns *ntpSourceImpl) createRequest(now time.Time) []byte {
	data := make([]byte, 48)
	data[0] = 0x23 // protocol version = 4, mode = 3 (Client)

	// For NTP the prime epoch, or base date of era 0, is 0 h 1 January 1900 UTC
	t0 := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
	d := now.Sub(t0)
	nsec := d.Nanoseconds()

	// The encoding of timestamp below is an exact opposite of the decoding
	// being done in extractTime(). Refer extractTime() for more explaination.
	sec := nsec / 1e9                  // Integer part of seconds since epoch
	frac := ((nsec % 1e9) << 32) / 1e9 // fractional part of seconds since epoch

	// write the timestamp to Transmit Timestamp section of request.
	data[43] = byte(sec)
	data[42] = byte(sec >> 8)
	data[41] = byte(sec >> 16)
	data[40] = byte(sec >> 24)

	data[47] = byte(frac)
	data[46] = byte(frac >> 8)
	data[45] = byte(frac >> 16)
	data[44] = byte(frac >> 24)
	return data
}

// extractTime takes a byte array which contains encoded timestamp from NTP
// server starting at the 0th byte and is 8 bytes long. The encoded timestamp is
// in seconds since 1900. The first 4 bytes contain the integer part of of the
// seconds while the last 4 bytes contain the fractional part of the seconds
// where (FFFFFFFF + 1) represents 1 second while 00000001 represents 2^(-32) of
// a second.
func extractTime(data []byte) time.Time {
	var sec, frac uint64
	sec = uint64(data[3]) | uint64(data[2])<<8 | uint64(data[1])<<16 | uint64(data[0])<<24
	frac = uint64(data[7]) | uint64(data[6])<<8 | uint64(data[5])<<16 | uint64(data[4])<<24

	// multiply the integral second part with 1Billion to convert to nanoseconds
	nsec := sec * 1e9
	// multiply frac part with 2^(-32) to get the correct value in seconds and
	// then multiply with 1Billion to convert to nanoseconds. The multiply by
	// Billion is done first to make sure that we dont lose precision.
	nsec += (frac * 1e9) >> 32

	return time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(nsec)).Local()
}
