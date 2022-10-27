// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package message defines the low level messages used to set up authenticated
// and encrypted network flows.
//
// One complexity is the need to allow for unencrypted payloads, which are
// sent separately and immediately following the Message that contains them
// rather than as part of that message itself. Such Messages have the
// DisableEncryptionFlag set. The wire protocol for the encrypted payload
// case is (< and > indicate message start stop):
//
//	<OpenFlow(DisableEncryptionFlag==false,payload)><other Messages>
//	<Data(DisableEncryptionFlag==false,payload)><other Messages>
//
// And for the unencrypted payload case:
//
//	<OpenFlow(DisableEncryptionFlag==true)><payload)><other Messages>
//	<Data(DisableEncryptionFlag==true)><payload)><other Messages>
//
// Thus two network writes and reads are required for the DisableEncryptionFlag
// is set case. In both cases, the Messages themselves are encrypted.
// This facility is primarily used to support proxies. In the case where
// DisableEncryptionFlag is set, but the payload is empty, the flag should
// be cleared to avoid the receiver having to read a zero length frame.
package message

import (
	"encoding/hex"
	"fmt"

	"v.io/v23/context"
	"v.io/v23/naming"
	"v.io/v23/rpc/version"
	"v.io/v23/security"
)

// Read reads a message contained in the byte slice 'from'.
func Read(ctx *context.T, from []byte) (Message, error) {
	if len(from) == 0 {
		return nil, NewErrInvalidMsg(ctx, InvalidType, 0, 0, nil)
	}
	switch from[0] {
	case DataType:
		return Data{}.ReadDirect(ctx, from[1:])
	case OpenFlowType:
		return OpenFlow{}.ReadDirect(ctx, from[1:])
	}
	return ReadNoPayload(ctx, from)
}

// ReadNoPayload reads any of the messages that do not carry a payload,
// ie. all except the Data and OpenFlow messages.
func ReadNoPayload(ctx *context.T, from []byte) (Message, error) {
	if len(from) == 0 {
		return nil, NewErrInvalidMsg(ctx, InvalidType, 0, 0, nil)
	}
	msgType, from := from[0], from[1:]
	switch msgType {
	case SetupType:
		return Setup{}.ReadDirect(ctx, from)
	case AuthType, AuthED25519Type, AuthRSAType:
		var m = Auth{signatureType: msgType}
		return m.Read(ctx, from)
	case ReleaseType:
		return Release{}.Read(ctx, from)
	case HealthCheckRequestType:
		return HealthCheckRequest{}.Read(ctx, from)
	case HealthCheckResponseType:
		return HealthCheckResponse{}.Read(ctx, from)
	case TearDownType:
		return TearDown{}.Read(ctx, from)
	case EnterLameDuckType:
		return EnterLameDuck{}.Read(ctx, from)
	case AckLameDuckType:
		return AckLameDuck{}.Read(ctx, from)
	case MultiProxyType:
		return MultiProxyRequest{}.Read(ctx, from)
	case ProxyServerType:
		return ProxyServerRequest{}.Read(ctx, from)
	case ProxyResponseType:
		return ProxyResponse{}.Read(ctx, from)
	case ProxyErrorReponseType:
		return ProxyErrorResponse{}.Read(ctx, from)
	}
	return nil, ErrUnknownMsg.Errorf(ctx, "unknown message type: %02x", msgType)
}

// FlowID returns the id of the flow this message is associated with for
// message types that are bound to a flow, zero otherwise.
func FlowID(m interface{}) uint64 {
	switch a := m.(type) {
	case Data:
		return a.ID
	case OpenFlow:
		return a.ID
	default:
		return 0
	}
}

// Message is implemented by all low-level messages defined by this package.
type Message interface {
	// Append serializes the message m and appends it to the given byte slice.
	// The resulting slice is returned, as well as any error that occurs during
	// serialization.
	Append(ctx *context.T, data []byte) ([]byte, error)

	// Read reads a message contained in the byte slice 'from'.
	Read(ctx *context.T, data []byte) (Message, error)

	// Copy must be called to make internal copies of any byte slices that
	// may be in a shared underlying slice (eg. in a slice allocated for
	// decrypting  a message). It should be called before passing a Message to
	// another goroutine for example.
	Copy() Message
}

// message types.
// Note that our types start from 0x7f and work their way down.
// This is important for the RPC transition.  The old system had
// messages that started from zero and worked their way up.
const (
	InvalidType = 0x7f - iota
	SetupType
	TearDownType
	EnterLameDuckType
	AckLameDuckType
	AuthType
	OpenFlowType
	ReleaseType
	DataType
	MultiProxyType
	ProxyServerType
	ProxyResponseType
	HealthCheckRequestType
	HealthCheckResponseType
	ProxyErrorReponseType
	AuthED25519Type
	AuthRSAType
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

func appendSetupOptionString(option uint64, payload string, buf []byte) []byte {
	return appendLenString(payload, writeVarUint64(option, buf))
}

func readSetupOption(ctx *context.T, orig []byte) (opt uint64, p, d []byte, err error) {
	var valid bool
	if opt, d, valid = readVarUint64(orig); !valid {
		err = ErrInvalidSetupOption.Errorf(ctx, "setup option: %v failed decoding at field: %v", invalidOption, 0)
	} else if p, d, valid = readLenBytes(d); !valid {
		err = ErrInvalidSetupOption.Errorf(ctx, "setup option: %v failed decoding at field: %v", opt, 1)
	}
	return
}

func (m Setup) Append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, SetupType)
	data = writeVarUint64(uint64(m.Versions.Min), data)
	data = writeVarUint64(uint64(m.Versions.Max), data)
	if m.PeerNaClPublicKey != nil {
		data = appendSetupOption(peerNaClPublicKeyOption,
			m.PeerNaClPublicKey[:], data)
	}
	if !m.PeerRemoteEndpoint.IsZero() {
		data = appendSetupOptionString(peerRemoteEndpointOption,
			m.PeerRemoteEndpoint.String(), data)
	}
	if !m.PeerLocalEndpoint.IsZero() {
		data = appendSetupOptionString(peerLocalEndpointOption,
			m.PeerLocalEndpoint.String(), data)
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

func (m Setup) Read(ctx *context.T, orig []byte) (Message, error) {
	return m.ReadDirect(ctx, orig)
}

func (m Setup) ReadDirect(ctx *context.T, orig []byte) (Setup, error) {
	var (
		data  = orig
		valid bool
		v     uint64
	)
	if v, data, valid = readVarUint64(data); !valid {
		return Setup{}, NewErrInvalidMsg(ctx, SetupType, uint64(len(orig)), 0, nil)
	}
	m.Versions.Min = version.RPCVersion(v)
	if v, data, valid = readVarUint64(data); !valid {
		return Setup{}, NewErrInvalidMsg(ctx, SetupType, uint64(len(orig)), 1, nil)
	}
	m.Versions.Max = version.RPCVersion(v)
	for field := uint64(2); len(data) > 0; field++ {
		var (
			payload []byte
			opt     uint64
			err     error
		)
		if opt, payload, data, err = readSetupOption(ctx, data); err != nil {
			return Setup{}, NewErrInvalidMsg(ctx, SetupType, uint64(len(orig)), field, err)
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
			if mtu, _, valid := readVarUint64(payload); valid {
				m.Mtu = mtu
			} else {
				return Setup{}, ErrInvalidSetupOption.Errorf(ctx, "setup option: %v failed decoding at field: %v", opt, field)
			}
		case sharedTokensOption:
			if t, _, valid := readVarUint64(payload); valid {
				m.SharedTokens = t
			} else {
				return Setup{}, ErrInvalidSetupOption.Errorf(ctx, "setup option: %v failed decoding at field: %v", opt, field)
			}
		default:
			m.uninterpretedOptions = append(m.uninterpretedOptions, option{opt, payload})
		}
		if err != nil {
			return Setup{}, NewErrInvalidMsg(ctx, SetupType, uint64(len(orig)), field, err)
		}
	}
	return m, nil
}

func (m Setup) Copy() Message {
	for i, v := range m.uninterpretedOptions {
		m.uninterpretedOptions[i].payload = copyBytes(v.payload)
	}
	return m
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

func (m TearDown) Append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, TearDownType)
	return append(data, []byte(m.Message)...), nil
}

func (m TearDown) Read(ctx *context.T, data []byte) (Message, error) {
	m.Message = string(data)
	return m, nil
}

func (m TearDown) Copy() Message { return m }

func (m TearDown) String() string { return m.Message }

// EnterLameDuck is sent as notification that the sender is entering lameduck mode.
// The receiver should stop opening new flows on this connection and respond
// with an AckLameDuck message.
type EnterLameDuck struct{}

func (m EnterLameDuck) Append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, EnterLameDuckType), nil
}

func (m EnterLameDuck) Read(ctx *context.T, data []byte) (Message, error) {
	return EnterLameDuck{}, nil
}

func (m EnterLameDuck) Copy() Message { return m }

// AckLameDuck is sent in response to an EnterLameDuck message.  After
// this message is received no more OpenFlow messages should arrive.
type AckLameDuck struct{}

func (m AckLameDuck) Append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, AckLameDuckType), nil
}

func (m AckLameDuck) Read(ctx *context.T, data []byte) (Message, error) {
	return AckLameDuck{}, nil
}

func (m AckLameDuck) Copy() Message { return m }

// Auth is used to complete the auth handshake.
type Auth struct {
	signatureType              byte
	BlessingsKey, DischargeKey uint64
	ChannelBinding             security.Signature
}

func (m Auth) appendCommon(data []byte) []byte {
	data = writeVarUint64(m.BlessingsKey, data)
	data = writeVarUint64(m.DischargeKey, data)
	data = appendLenBytes(m.ChannelBinding.Purpose, data)
	data = appendLenBytes([]byte(m.ChannelBinding.Hash), data)
	return data
}

func (m Auth) Append(ctx *context.T, data []byte) ([]byte, error) {
	switch {
	case len(m.ChannelBinding.Ed25519) > 0:
		data = append(data, AuthED25519Type)
		data = m.appendCommon(data)
		data = appendLenBytes(m.ChannelBinding.Ed25519, data)
	case len(m.ChannelBinding.Rsa) > 0:
		data = append(data, AuthRSAType)
		data = m.appendCommon(data)
		data = appendLenBytes(m.ChannelBinding.Rsa, data)
	default:
		data = append(data, AuthType)
		data = m.appendCommon(data)
		data = appendLenBytes(m.ChannelBinding.R, data)
		data = appendLenBytes(m.ChannelBinding.S, data)
	}
	return data, nil
}

func (m Auth) Read(ctx *context.T, orig []byte) (Message, error) {
	var data, tmp []byte
	var valid bool
	if m.BlessingsKey, data, valid = readVarUint64(orig); !valid {
		return Auth{}, NewErrInvalidMsg(ctx, OpenFlowType, uint64(len(orig)), 0, nil)
	}
	if m.BlessingsKey == 0 {
		return Auth{}, ErrMissingBlessings.Errorf(ctx, "%02x: message received with no blessings", OpenFlowType)
	}
	if m.DischargeKey, data, valid = readVarUint64(data); !valid {
		return Auth{}, NewErrInvalidMsg(ctx, OpenFlowType, uint64(len(orig)), 1, nil)
	}
	if m.ChannelBinding.Purpose, data, valid = readLenBytes(data); !valid {
		return Auth{}, NewErrInvalidMsg(ctx, OpenFlowType, uint64(len(orig)), 2, nil)
	}
	if tmp, data, valid = readLenBytes(data); !valid {
		return Auth{}, NewErrInvalidMsg(ctx, OpenFlowType, uint64(len(orig)), 3, nil)
	}
	m.ChannelBinding.Hash = security.Hash(tmp)
	switch m.signatureType {
	case AuthType:
		if m.ChannelBinding.R, data, valid = readLenBytes(data); !valid {
			return Auth{}, NewErrInvalidMsg(ctx, OpenFlowType, uint64(len(orig)), 4, nil)
		}
		if m.ChannelBinding.S, _, _ = readLenBytes(data); !valid {
			return Auth{}, NewErrInvalidMsg(ctx, OpenFlowType, uint64(len(orig)), 5, nil)
		}
	case AuthED25519Type:
		if m.ChannelBinding.Ed25519, _, _ = readLenBytes(data); !valid {
			return Auth{}, NewErrInvalidMsg(ctx, OpenFlowType, uint64(len(orig)), 4, nil)
		}
	case AuthRSAType:
		if m.ChannelBinding.Rsa, _, _ = readLenBytes(data); !valid {
			return Auth{}, NewErrInvalidMsg(ctx, OpenFlowType, uint64(len(orig)), 4, nil)
		}
	}
	return m, nil
}

func (m Auth) Copy() Message {
	m.ChannelBinding.Purpose = copyBytes(m.ChannelBinding.Purpose)
	switch m.signatureType {
	case AuthType:
		m.ChannelBinding.R = copyBytes(m.ChannelBinding.R)
		m.ChannelBinding.S = copyBytes(m.ChannelBinding.S)
	case AuthED25519Type:
		m.ChannelBinding.Ed25519 = copyBytes(m.ChannelBinding.Ed25519)
	case AuthRSAType:
		m.ChannelBinding.Rsa = copyBytes(m.ChannelBinding.Rsa)
	}
	return m
}

func (m Auth) SetSignatureType(sigType byte) Auth {
	m.signatureType = sigType
	return m
}

// OpenFlow is sent at the beginning of every new flow, it optionally contains payload.
type OpenFlow struct {
	ID                         uint64
	InitialCounters            uint64
	BlessingsKey, DischargeKey uint64
	Flags                      uint64
	Payload                    [][]byte
	nocopy                     bool
}

func (m OpenFlow) Append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, OpenFlowType)
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

func (m OpenFlow) Read(ctx *context.T, orig []byte) (Message, error) {
	return m.ReadDirect(ctx, orig)
}

func (m OpenFlow) ReadDirect(ctx *context.T, orig []byte) (OpenFlow, error) {

	var (
		data  []byte
		valid bool
	)
	if m.ID, data, valid = readVarUint64(orig); !valid {
		return OpenFlow{}, NewErrInvalidMsg(ctx, OpenFlowType, uint64(len(orig)), 0, nil)
	}
	if m.InitialCounters, data, valid = readVarUint64(data); !valid {
		return OpenFlow{}, NewErrInvalidMsg(ctx, OpenFlowType, uint64(len(orig)), 1, nil)
	}
	if m.BlessingsKey, data, valid = readVarUint64(data); !valid {
		return OpenFlow{}, NewErrInvalidMsg(ctx, OpenFlowType, uint64(len(orig)), 2, nil)
	}
	if m.BlessingsKey == 0 {
		return OpenFlow{}, ErrMissingBlessings.Errorf(ctx, "%02x: message received with no blessings", OpenFlowType)
	}
	if m.DischargeKey, data, valid = readVarUint64(data); !valid {
		return OpenFlow{}, NewErrInvalidMsg(ctx, OpenFlowType, uint64(len(orig)), 3, nil)
	}
	if m.Flags, data, valid = readVarUint64(data); !valid {
		return OpenFlow{}, NewErrInvalidMsg(ctx, DataType, uint64(len(orig)), 1, nil)
	}
	if m.Flags&DisableEncryptionFlag == 0 && len(data) > 0 {
		m.Payload = [][]byte{data}
	}
	return m, nil
}

func (m OpenFlow) Copy() Message {
	if m.nocopy {
		return m
	}
	for i, v := range m.Payload {
		m.Payload[i] = copyBytes(v)
	}
	return m
}

// SetNoCopy is used to indicate whether a subsequent call to Copy
// needs to copy the payload.
func (m OpenFlow) SetNoCopy(nocopy bool) OpenFlow {
	m.nocopy = nocopy
	return m
}

func (m OpenFlow) String() string {
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
	Counters []Counter
}

// Counter represents the flow control tokens to be assigned to
// a speficic flow.
type Counter struct {
	FlowID uint64
	Tokens uint64
}

func (m Release) Append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, ReleaseType)
	for _, c := range m.Counters {
		data = writeVarUint64(c.FlowID, data)
		data = writeVarUint64(c.Tokens, data)
	}
	return data, nil
}

func (m Release) Read(ctx *context.T, orig []byte) (Message, error) {
	var (
		valid    bool
		fid, val uint64
		n        uint64
	)
	if len(orig) == 0 {
		return Release{}, nil
	}
	m.Counters = []Counter{}
	for len(orig) > 0 {
		if fid, orig, valid = readVarUint64(orig); !valid {
			return Release{}, NewErrInvalidMsg(ctx, ReleaseType, uint64(len(orig)), n*2, nil)
		}
		if val, orig, valid = readVarUint64(orig); !valid {
			return Release{}, NewErrInvalidMsg(ctx, ReleaseType, uint64(len(orig)), (n*2)+1, nil)
		}
		m.Counters = append(m.Counters, Counter{FlowID: fid, Tokens: val})
		n++
	}
	return m, nil
}

func (m Release) Copy() Message { return m }

func (m Release) String() string {
	return fmt.Sprintf("release #%v counters", len(m.Counters))
}

// Data carries encrypted data for a specific flow.
type Data struct {
	ID      uint64
	Flags   uint64
	Payload [][]byte
	nocopy  bool
}

func (m Data) Append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, DataType)
	data = writeVarUint64(m.ID, data)
	data = writeVarUint64(m.Flags, data)
	if m.Flags&DisableEncryptionFlag == 0 {
		for _, p := range m.Payload {
			data = append(data, p...)
		}
	}
	return data, nil
}

func (m Data) Read(ctx *context.T, orig []byte) (Message, error) {
	return m.ReadDirect(ctx, orig)
}

func (m Data) ReadDirect(ctx *context.T, orig []byte) (Data, error) {
	var data []byte
	var valid bool
	if m.ID, data, valid = readVarUint64(orig); !valid {
		return Data{}, NewErrInvalidMsg(ctx, DataType, uint64(len(orig)), 0, nil)
	}
	if m.Flags, data, valid = readVarUint64(data); !valid {
		return Data{}, NewErrInvalidMsg(ctx, DataType, uint64(len(orig)), 1, nil)
	}
	m.Payload = nil
	if m.Flags&DisableEncryptionFlag == 0 && len(data) > 0 {
		m.Payload = [][]byte{data}
	}
	return m, nil
}

func (m Data) Copy() Message {
	if m.nocopy {
		return m
	}
	for i, v := range m.Payload {
		m.Payload[i] = copyBytes(v)
	}
	return m
}

// SetNoCopy is used to indicate whether a subsequent call to Copy
// needs to copy the payload.
func (m Data) SetNoCopy(nocopy bool) Data {
	m.nocopy = nocopy
	return m
}

func (m *Data) String() string {
	return fmt.Sprintf("ID:%d Flags:0x%x Payload:(%d bytes in %d slices)", m.ID, m.Flags, payloadSize(m.Payload), len(m.Payload))
}

// MultiProxyRequest is sent when a proxy wants to accept connections from another proxy.
type MultiProxyRequest struct{}

func (m MultiProxyRequest) Append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, MultiProxyType), nil
}

func (m MultiProxyRequest) Read(ctx *context.T, orig []byte) (Message, error) {
	return MultiProxyRequest{}, nil
}

func (m MultiProxyRequest) Copy() Message { return m }

// ProxyServerRequest is sent when a server wants to listen through a proxy.
type ProxyServerRequest struct{}

func (m ProxyServerRequest) Append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, ProxyServerType), nil
}

func (m ProxyServerRequest) Read(ctx *context.T, orig []byte) (Message, error) {
	return ProxyServerRequest{}, nil
}

func (m ProxyServerRequest) Copy() Message { return m }

// ProxyResponse is sent by a proxy in response to a ProxyServerRequest or
// MultiProxyRequest. It notifies the server of the endpoints it should publish.
// Or, it notifies a requesting proxy of the endpoints it accepts connections from.
type ProxyResponse struct {
	Endpoints []naming.Endpoint
}

func (m ProxyResponse) Append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, ProxyResponseType)
	for _, ep := range m.Endpoints {
		data = appendLenString(ep.String(), data)
	}
	return data, nil
}

func (m ProxyResponse) Read(ctx *context.T, orig []byte) (Message, error) {
	var (
		data    = orig
		epBytes []byte
		valid   bool
	)
	if len(data) == 0 {
		return ProxyResponse{}, nil
	}
	m.Endpoints = make([]naming.Endpoint, 0, len(data))
	for i := 0; len(data) > 0; i++ {
		if epBytes, data, valid = readLenBytes(data); !valid {
			return ProxyResponse{}, NewErrInvalidMsg(ctx, ProxyResponseType, uint64(len(orig)), uint64(i), nil)
		}
		ep, err := naming.ParseEndpoint(string(epBytes))
		if err != nil {
			return ProxyResponse{}, NewErrInvalidMsg(ctx, ProxyResponseType, uint64(len(orig)), uint64(i), err)
		}
		m.Endpoints = append(m.Endpoints, ep)
	}
	return m, nil
}

func (m ProxyResponse) Copy() Message { return m }

func (m ProxyResponse) String() string {
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

func (m ProxyErrorResponse) Append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, ProxyErrorReponseType)
	data = append(data, []byte(m.Error)...)
	return data, nil
}

func (m ProxyErrorResponse) Read(ctx *context.T, orig []byte) (Message, error) {
	m.Error = string(orig)
	return m, nil
}

func (m ProxyErrorResponse) Copy() Message { return m }

func (m ProxyErrorResponse) String() string {
	return m.Error
}

// HealthCheckRequest is periodically sent to test the health of a channel.
type HealthCheckRequest struct{}

func (m HealthCheckRequest) Append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, HealthCheckRequestType), nil
}

func (m HealthCheckRequest) Read(ctx *context.T, data []byte) (Message, error) {
	return HealthCheckRequest{}, nil
}

func (m HealthCheckRequest) Copy() Message { return m }

// HealthCheckResponse messages are sent in response to HealthCheckRequest messages.
type HealthCheckResponse struct{}

func (m HealthCheckResponse) Append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, HealthCheckResponseType), nil
}

func (m HealthCheckResponse) Read(ctx *context.T, data []byte) (Message, error) {
	return HealthCheckResponse{}, nil
}

func (m HealthCheckResponse) Copy() Message { return m }

func copyBytes(b []byte) []byte {
	cpy := make([]byte, len(b))
	copy(cpy, b)
	return cpy
}

func appendLenBytes(b []byte, buf []byte) []byte {
	buf = writeVarUint64(uint64(len(b)), buf)
	return append(buf, b...)
}

func appendLenString(b string, buf []byte) []byte {
	buf = writeVarUint64(uint64(len(b)), buf)
	return append(buf, b...)
}

func readLenBytes(data []byte) (b, rest []byte, valid bool) {
	l, data, valid := readVarUint64(data)
	if !valid || uint64(len(data)) < l {
		return nil, data, false
	}
	if l > 0 {
		return data[:l], data[l:], true
	}
	return nil, data, true
}

func readVarUint64(data []byte) (uint64, []byte, bool) {
	if len(data) == 0 {
		return 0, data, false
	}
	u := uint64(data[0])
	if u <= 0x7f {
		return u, data[1:], true
	}
	switch 0xff - u {
	case 0:
		return uint64(data[1]),
			data[2:], true
	case 1:
		return uint64(data[1])<<8 | uint64(data[2]),
			data[3:], true
	case 2:
		return uint64(data[1])<<16 | uint64(data[2])<<8 | uint64(data[3]),
			data[4:], true
	case 3:
		return uint64(data[1])<<24 | uint64(data[2])<<16 | uint64(data[3])<<8 |
				uint64(data[4]),
			data[5:], true
	case 4:
		return uint64(data[1])<<32 | uint64(data[2])<<24 | uint64(data[3])<<16 |
				uint64(data[4])<<8 | uint64(data[5]),
			data[6:], true
	case 5:
		return uint64(data[1])<<40 | uint64(data[2])<<32 | uint64(data[3])<<24 |
				uint64(data[4])<<16 | uint64(data[5])<<8 | uint64(data[6]),
			data[7:], true
	case 6:
		return uint64(data[1])<<48 | uint64(data[2])<<40 | uint64(data[3])<<32 |
				uint64(data[4])<<24 | uint64(data[5])<<16 | uint64(data[6])<<8 |
				uint64(data[7]),
			data[8:], true
	case 7:
		return uint64(data[1])<<56 | uint64(data[2])<<48 | uint64(data[3])<<40 |
				uint64(data[4])<<32 | uint64(data[5])<<24 | uint64(data[6])<<16 |
				uint64(data[7])<<8 | uint64(data[8]),
			data[9:], true

	}
	return 0, data, false
}

func writeVarUint64(u uint64, buf []byte) []byte {
	switch {
	case u <= 0x7f:
		return append(buf, byte(u))
	case u <= 0xff:
		return append(buf, 0xff,
			byte(u&0xff))
	case u <= 0xffff:
		return append(buf, 0xff-1,
			byte((u&0xff00)>>8),
			byte(u&0xff))
	case u <= 0xffffff:
		return append(buf, 0xff-2,
			byte((u&0xff0000)>>16),
			byte((u&0xff00)>>8),
			byte(u&0xff))
	case u <= 0xffffffff:
		return append(buf, 0xff-3,
			byte((u&0xff000000)>>24),
			byte((u&0xff0000)>>16),
			byte((u&0xff00)>>8),
			byte(u&0xff))
	case u <= 0xffffffffff:
		return append(buf, 0xff-4,
			byte((u&0xff00000000)>>32),
			byte((u&0xff000000)>>24),
			byte((u&0xff0000)>>16),
			byte((u&0xff00)>>8),
			byte(u&0xff))
	case u <= 0xffffffffffff:
		return append(buf, 0xff-5,
			byte((u&0xff0000000000)>>40),
			byte((u&0xff00000000)>>32),
			byte((u&0xff000000)>>24),
			byte((u&0xff0000)>>16),
			byte((u&0xff00)>>8),
			byte(u&0xff))
	case u <= 0xffffffffffffff:
		return append(buf, 0xff-6,
			byte((u&0xff000000000000)>>48),
			byte((u&0xff0000000000)>>40),
			byte((u&0xff00000000)>>32),
			byte((u&0xff000000)>>24),
			byte((u&0xff0000)>>16),
			byte((u&0xff00)>>8),
			byte(u&0xff))
	case u <= 0xffffffffffffffff:
		// Note that using a default statement is significantly slower.
		return append(buf, 0xff-7,
			byte((u&0xff00000000000000)>>56),
			byte((u&0xff000000000000)>>48),
			byte((u&0xff0000000000)>>40),
			byte((u&0xff00000000)>>32),
			byte((u&0xff000000)>>24),
			byte((u&0xff0000)>>16),
			byte((u&0xff00)>>8),
			byte(u&0xff))
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
