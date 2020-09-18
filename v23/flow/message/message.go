// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package message

import (
	"encoding/hex"
	"fmt"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc/version"
	"v.io/v23/security"
)

// TODO(mattr): Link to protocol doc.

// Append serializes the message m and appends it to the given byte slice.
// The resulting slice is returned, as well as any error that occurs during
// serialization.
func Append(ctx *context.T, m Message, to []byte) ([]byte, error) {
	return m.append(ctx, to)
}

// Read reads a message contained in the byte slice 'from'.
func Read(ctx *context.T, from []byte) (Message, error) { //nolint:gocyclo
	if len(from) == 0 {
		return nil, NewErrInvalidMsg(ctx, invalidType, 0, 0, nil)
	}
	var m Message
	msgType, from := from[0], from[1:]
	switch msgType {
	case setupType:
		m = &Setup{}
	case tearDownType:
		m = &TearDown{}
	case enterLameDuckType:
		m = &EnterLameDuck{}
	case ackLameDuckType:
		m = &AckLameDuck{}
	case authType:
		m = &Auth{}
	case openFlowType:
		m = &OpenFlow{}
	case releaseType:
		m = &Release{}
	case dataType:
		m = &Data{}
	case multiProxyType:
		m = &MultiProxyRequest{}
	case proxyServerType:
		m = &ProxyServerRequest{}
	case proxyResponseType:
		m = &ProxyResponse{}
	case proxyErrorReponseType:
		m = &ProxyErrorResponse{}
	case healthCheckRequestType:
		m = &HealthCheckRequest{}
	case healthCheckResponseType:
		m = &HealthCheckResponse{}
	default:
		return nil, ErrUnknownMsg.Errorf(ctx, "unknown message type: %02x", msgType)
	}
	return m, m.read(ctx, from)
}

// FlowID returns the id of the flow this message is associated with for
// message types that are bound to a flow, zero otherwise.
func FlowID(m interface{}) uint64 {
	switch a := m.(type) {
	case *Data:
		return a.ID
	case *OpenFlow:
		return a.ID
	default:
		return 0
	}
}

// Message is implemented by all low-level messages defined by this package.
type Message interface {
	append(ctx *context.T, data []byte) ([]byte, error)
	read(ctx *context.T, data []byte) error
}

// message types.
// Note that our types start from 0x7f and work their way down.
// This is important for the RPC transition.  The old system had
// messages that started from zero and worked their way up.
const (
	invalidType = 0x7f - iota
	setupType
	tearDownType
	enterLameDuckType
	ackLameDuckType
	authType
	openFlowType
	releaseType
	dataType
	multiProxyType
	proxyServerType
	proxyResponseType
	healthCheckRequestType
	healthCheckResponseType
	proxyErrorReponseType
)

// setup options.
const (
	invalidOption = iota
	peerNaClPublicKeyOption
	peerRemoteEndpointOption
	peerLocalEndpointOption
	mtuOption
	sharedTokensOption
)

// data flags.
const (
	// CloseFlag, when set on a Data message, indicates that the flow
	// should be closed after the payload for that message has been
	// processed.
	CloseFlag = 1 << iota
	// DisableEncryptionFlag, when set on a Data message, causes Append
	// and Read to behave specially.  During Append everything but the
	// Payload is serialized normally, but the Payload is left for the
	// caller to deal with.  Typically the caller will encrypt the
	// serialized result and send the unencrypted bytes as a raw follow
	// on message.  Read will expect to not find a payload, and the
	// caller will typically attach it by examining the next message on
	// the Conn.
	DisableEncryptionFlag
	// SideChannelFlag, when set on a OpenFlow message, marks the Flow
	// as a side channel. This means that the flow will not be counted
	// towards the "idleness" of the underlying connection.
	SideChannelFlag
)

// Setup is the first message over the wire.  It negotiates protocol version
// and encryption options for connection.
// New fields to Setup must be added in order of creation. i.e. the order of the fields
// should not be changed.
type Setup struct {
	Versions             version.RPCVersionRange
	PeerNaClPublicKey    *[32]byte
	PeerRemoteEndpoint   naming.Endpoint
	PeerLocalEndpoint    naming.Endpoint
	Mtu                  uint64
	SharedTokens         uint64
	uninterpretedOptions []option
}

type option struct {
	opt     uint64
	payload []byte
}

func appendSetupOption(option uint64, payload, buf []byte) []byte {
	return appendLenBytes(payload, writeVarUint64(option, buf))
}
func readSetupOption(ctx *context.T, orig []byte) (opt uint64, p, d []byte, err error) {
	var valid bool
	if opt, d, valid = readVarUint64(ctx, orig); !valid {
		err = ErrInvalidSetupOption.Errorf(ctx, "setup option: %v failed decoding at field: %v", invalidOption, 0)
	} else if p, d, valid = readLenBytes(ctx, d); !valid {
		err = ErrInvalidSetupOption.Errorf(ctx, "setup option: %v failed decoding at field: %v", opt, 1)
	}
	return
}

func (m *Setup) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, setupType)
	data = writeVarUint64(uint64(m.Versions.Min), data)
	data = writeVarUint64(uint64(m.Versions.Max), data)
	if m.PeerNaClPublicKey != nil {
		data = appendSetupOption(peerNaClPublicKeyOption,
			m.PeerNaClPublicKey[:], data)
	}
	if !m.PeerRemoteEndpoint.IsZero() {
		data = appendSetupOption(peerRemoteEndpointOption,
			[]byte(m.PeerRemoteEndpoint.String()), data)
	}
	if !m.PeerLocalEndpoint.IsZero() {
		data = appendSetupOption(peerLocalEndpointOption,
			[]byte(m.PeerLocalEndpoint.String()), data)
	}
	if m.Mtu != 0 {
		data = appendSetupOption(mtuOption, writeVarUint64(m.Mtu, nil), data)
	}
	if m.SharedTokens != 0 {
		data = appendSetupOption(sharedTokensOption, writeVarUint64(m.SharedTokens, nil), data)
	}
	for _, o := range m.uninterpretedOptions {
		data = appendSetupOption(o.opt, o.payload, data)
	}
	return data, nil
}
func (m *Setup) read(ctx *context.T, orig []byte) error {
	var (
		data  = orig
		valid bool
		v     uint64
	)
	if v, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, setupType, uint64(len(orig)), 0, nil)
	}
	m.Versions.Min = version.RPCVersion(v)
	if v, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, setupType, uint64(len(orig)), 1, nil)
	}
	m.Versions.Max = version.RPCVersion(v)
	for field := uint64(2); len(data) > 0; field++ {
		var (
			payload []byte
			opt     uint64
			err     error
		)
		if opt, payload, data, err = readSetupOption(ctx, data); err != nil {
			return NewErrInvalidMsg(ctx, setupType, uint64(len(orig)), field, err)
		}
		switch opt {
		case peerNaClPublicKeyOption:
			m.PeerNaClPublicKey = new([32]byte)
			copy(m.PeerNaClPublicKey[:], payload)
		case peerRemoteEndpointOption:
			m.PeerRemoteEndpoint, err = naming.ParseEndpoint(string(payload))
		case peerLocalEndpointOption:
			m.PeerLocalEndpoint, err = naming.ParseEndpoint(string(payload))
		case mtuOption:
			if mtu, _, valid := readVarUint64(ctx, payload); valid {
				m.Mtu = mtu
			} else {
				return ErrInvalidSetupOption.Errorf(ctx, "setup option: %v failed decoding at field: %v", opt, field)
			}
		case sharedTokensOption:
			if t, _, valid := readVarUint64(ctx, payload); valid {
				m.SharedTokens = t
			} else {
				return ErrInvalidSetupOption.Errorf(ctx, "setup option: %v failed decoding at field: %v", opt, field)
			}
		default:
			m.uninterpretedOptions = append(m.uninterpretedOptions, option{opt, payload})
		}
		if err != nil {
			return NewErrInvalidMsg(ctx, setupType, uint64(len(orig)), field, err)
		}
	}
	return nil
}
func (m *Setup) String() string {
	return fmt.Sprintf("Versions:[%d,%d] PeerNaClPublicKey:%v PeerRemoteEndpoint:%v PeerLocalEndpoint:%v options:%v",
		m.Versions.Min,
		m.Versions.Max,
		hex.EncodeToString(m.PeerNaClPublicKey[:]),
		m.PeerRemoteEndpoint,
		m.PeerLocalEndpoint,
		m.uninterpretedOptions)
}

// TearDown is sent over the wire before a connection is closed.
type TearDown struct {
	Message string
}

func (m *TearDown) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, tearDownType)
	return append(data, []byte(m.Message)...), nil
}
func (m *TearDown) read(ctx *context.T, data []byte) error {
	m.Message = string(data)
	return nil
}
func (m *TearDown) String() string { return m.Message }

// EnterLameDuck is sent as notification that the sender is entering lameduck mode.
// The receiver should stop opening new flows on this connection and respond
// with an AckLameDuck message.
type EnterLameDuck struct{}

func (m *EnterLameDuck) append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, enterLameDuckType), nil
}
func (m *EnterLameDuck) read(ctx *context.T, data []byte) error {
	return nil
}

// AckLameDuck is sent in response to an EnterLameDuck message.  After
// this message is received no more OpenFlow messages should arrive.
type AckLameDuck struct{}

func (m *AckLameDuck) append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, ackLameDuckType), nil
}
func (m *AckLameDuck) read(ctx *context.T, data []byte) error {
	return nil
}

// auth is used to complete the auth handshake.
type Auth struct {
	BlessingsKey, DischargeKey uint64
	ChannelBinding             security.Signature
}

func (m *Auth) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, authType)
	data = writeVarUint64(m.BlessingsKey, data)
	data = writeVarUint64(m.DischargeKey, data)
	data = appendLenBytes(m.ChannelBinding.Purpose, data)
	data = appendLenBytes([]byte(m.ChannelBinding.Hash), data)
	data = appendLenBytes(m.ChannelBinding.R, data)
	data = appendLenBytes(m.ChannelBinding.S, data)
	return data, nil
}
func (m *Auth) read(ctx *context.T, orig []byte) error {
	var data, tmp []byte
	var valid bool
	if m.BlessingsKey, data, valid = readVarUint64(ctx, orig); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 0, nil)
	}
	if m.BlessingsKey == 0 {
		return ErrMissingBlessings.Errorf(ctx, "%02x: message received with no blessings", openFlowType)
	}
	if m.DischargeKey, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 1, nil)
	}
	if m.ChannelBinding.Purpose, data, valid = readLenBytes(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 2, nil)
	}
	if tmp, data, valid = readLenBytes(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 3, nil)
	}
	m.ChannelBinding.Hash = security.Hash(tmp)
	if m.ChannelBinding.R, data, valid = readLenBytes(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 4, nil)
	}
	if m.ChannelBinding.S, _, valid = readLenBytes(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 5, nil)
	}
	return nil
}

// OpenFlow is sent at the beginning of every new flow, it optionally contains payload.
type OpenFlow struct {
	ID                         uint64
	InitialCounters            uint64
	BlessingsKey, DischargeKey uint64
	Flags                      uint64
	Payload                    [][]byte
}

func (m *OpenFlow) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, openFlowType)
	data = writeVarUint64(m.ID, data)
	data = writeVarUint64(m.InitialCounters, data)
	data = writeVarUint64(m.BlessingsKey, data)
	data = writeVarUint64(m.DischargeKey, data)
	data = writeVarUint64(m.Flags, data)
	if m.Flags&DisableEncryptionFlag == 0 {
		for _, p := range m.Payload {
			data = append(data, p...)
		}
	}
	return data, nil
}
func (m *OpenFlow) read(ctx *context.T, orig []byte) error {
	var (
		data  []byte
		valid bool
	)
	if m.ID, data, valid = readVarUint64(ctx, orig); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 0, nil)
	}
	if m.InitialCounters, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 1, nil)
	}
	if m.BlessingsKey, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 2, nil)
	}
	if m.BlessingsKey == 0 {
		return ErrMissingBlessings.Errorf(ctx, "%02x: message received with no blessings", openFlowType)
	}
	if m.DischargeKey, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 3, nil)
	}
	if m.Flags, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, dataType, uint64(len(orig)), 1, nil)
	}
	if m.Flags&DisableEncryptionFlag == 0 && len(data) > 0 {
		m.Payload = [][]byte{data}
	}
	return nil
}
func (m *OpenFlow) String() string {
	return fmt.Sprintf("ID:%d InitialCounters:%d BlessingsKey:0x%x DischargeKey:0x%x Flags:0x%x Payload:(%d bytes in %d slices)",
		m.ID,
		m.InitialCounters,
		m.BlessingsKey,
		m.DischargeKey,
		m.Flags,
		payloadSize(m.Payload),
		len(m.Payload))
}

// Release is sent as flows are read from locally.  The counters
// inform remote writers that there is local buffer space available.
type Release struct {
	Counters map[uint64]uint64
}

func (m *Release) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, releaseType)
	for fid, val := range m.Counters {
		data = writeVarUint64(fid, data)
		data = writeVarUint64(val, data)
	}
	return data, nil
}
func (m *Release) read(ctx *context.T, orig []byte) error {
	var (
		data     = orig
		valid    bool
		fid, val uint64
		n        uint64
	)
	if len(data) == 0 {
		return nil
	}
	m.Counters = map[uint64]uint64{}
	for len(data) > 0 {
		if fid, data, valid = readVarUint64(ctx, data); !valid {
			return NewErrInvalidMsg(ctx, releaseType, uint64(len(orig)), n, nil)
		}
		if val, data, valid = readVarUint64(ctx, data); !valid {
			return NewErrInvalidMsg(ctx, releaseType, uint64(len(orig)), n+1, nil)
		}
		m.Counters[fid] = val
		n += 2
	}
	return nil
}

// Data carries encrypted data for a specific flow.
type Data struct {
	ID      uint64
	Flags   uint64
	Payload [][]byte
}

func (m *Data) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, dataType)
	data = writeVarUint64(m.ID, data)
	data = writeVarUint64(m.Flags, data)
	if m.Flags&DisableEncryptionFlag == 0 {
		for _, p := range m.Payload {
			data = append(data, p...)
		}
	}
	return data, nil
}
func (m *Data) read(ctx *context.T, orig []byte) error {
	var (
		data  []byte
		valid bool
	)
	if m.ID, data, valid = readVarUint64(ctx, orig); !valid {
		return NewErrInvalidMsg(ctx, dataType, uint64(len(orig)), 0, nil)
	}
	if m.Flags, data, valid = readVarUint64(ctx, data); !valid {
		return NewErrInvalidMsg(ctx, dataType, uint64(len(orig)), 1, nil)
	}
	if m.Flags&DisableEncryptionFlag == 0 && len(data) > 0 {
		m.Payload = [][]byte{data}
	}
	return nil
}
func (m *Data) String() string {
	return fmt.Sprintf("ID:%d Flags:0x%x Payload:(%d bytes in %d slices)", m.ID, m.Flags, payloadSize(m.Payload), len(m.Payload))
}

// MultiProxyRequest is sent when a proxy wants to accept connections from another proxy.
type MultiProxyRequest struct{}

func (m *MultiProxyRequest) append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, multiProxyType), nil
}
func (m *MultiProxyRequest) read(ctx *context.T, orig []byte) error {
	return nil
}

// ProxyServerRequest is sent when a server wants to listen through a proxy.
type ProxyServerRequest struct{}

func (m *ProxyServerRequest) append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, proxyServerType), nil
}
func (m *ProxyServerRequest) read(ctx *context.T, orig []byte) error {
	return nil
}

// ProxyResponse is sent by a proxy in response to a ProxyServerRequest or
// MultiProxyRequest. It notifies the server of the endpoints it should publish.
// Or, it notifies a requesting proxy of the endpoints it accepts connections from.
type ProxyResponse struct {
	Endpoints []naming.Endpoint
}

func (m *ProxyResponse) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, proxyResponseType)
	for _, ep := range m.Endpoints {
		data = appendLenBytes([]byte(ep.String()), data)
	}
	return data, nil
}
func (m *ProxyResponse) read(ctx *context.T, orig []byte) error {
	var (
		data    = orig
		epBytes []byte
		valid   bool
	)
	for i := 0; len(data) > 0; i++ {
		if epBytes, data, valid = readLenBytes(ctx, data); !valid {
			return NewErrInvalidMsg(ctx, proxyResponseType, uint64(len(orig)), uint64(i), nil)
		}
		ep, err := naming.ParseEndpoint(string(epBytes))
		if err != nil {
			return NewErrInvalidMsg(ctx, proxyResponseType, uint64(len(orig)), uint64(i), err)
		}
		m.Endpoints = append(m.Endpoints, ep)
	}
	return nil
}
func (m *ProxyResponse) String() string {
	strs := make([]string, len(m.Endpoints))
	for i, ep := range m.Endpoints {
		strs[i] = ep.String()
	}
	return fmt.Sprintf("Endpoints:%v", strs)
}

// ProxyErrorResponse is send by a proxy in response to a ProxyServerRequest or
// MultiProxyRequest if the proxy encountered an error while responding to the
// request. It should be sent in lieu of a ProxyResponse.
type ProxyErrorResponse struct {
	Error string
}

func (m *ProxyErrorResponse) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, proxyErrorReponseType)
	data = append(data, []byte(m.Error)...)
	return data, nil
}
func (m *ProxyErrorResponse) read(ctx *context.T, orig []byte) error {
	m.Error = string(orig)
	return nil
}
func (m *ProxyErrorResponse) String() string {
	return m.Error
}

// HealthCheckRequest is periodically sent to test the health of a channel.
type HealthCheckRequest struct{}

func (m *HealthCheckRequest) append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, healthCheckRequestType), nil
}
func (m *HealthCheckRequest) read(ctx *context.T, data []byte) error {
	return nil
}

// HealthCheckResponse messages are sent in response to HealthCheckRequest messages.
type HealthCheckResponse struct{}

func (m *HealthCheckResponse) append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, healthCheckResponseType), nil
}
func (m *HealthCheckResponse) read(ctx *context.T, data []byte) error {
	return nil
}

func appendLenBytes(b []byte, buf []byte) []byte {
	buf = writeVarUint64(uint64(len(b)), buf)
	return append(buf, b...)
}

func readLenBytes(ctx *context.T, data []byte) (b, rest []byte, valid bool) {
	l, data, valid := readVarUint64(ctx, data)
	if !valid || uint64(len(data)) < l {
		return nil, data, false
	}
	if l > 0 {
		return data[:l], data[l:], true
	}
	return nil, data, true
}

func readVarUint64(ctx *context.T, data []byte) (uint64, []byte, bool) {
	if len(data) == 0 {
		return 0, data, false
	}
	l := data[0]
	if l <= 0x7f {
		return uint64(l), data[1:], true
	}
	l = 0xff - l + 1
	if l > 8 || len(data)-1 < int(l) {
		return 0, data, false
	}
	var out uint64
	for i := 1; i < int(l+1); i++ {
		out = out<<8 | uint64(data[i])
	}
	return out, data[l+1:], true
}

func writeVarUint64(u uint64, buf []byte) []byte {
	if u <= 0x7f {
		return append(buf, byte(u))
	}
	shift, l := 56, byte(7)
	for ; shift >= 0 && (u>>uint(shift))&0xff == 0; shift, l = shift-8, l-1 {
	}
	buf = append(buf, 0xff-l)
	for ; shift >= 0; shift -= 8 {
		buf = append(buf, byte(u>>uint(shift))&0xff)
	}
	return buf
}

func payloadSize(payload [][]byte) int {
	sz := 0
	for _, p := range payload {
		sz += len(p)
	}
	return sz
}
