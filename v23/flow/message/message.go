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
	case dataType:
		m = &Data{}
	case setupType:
		m = &Setup{}
	case authType, authED25519Type, authRSAType:
		m = &Auth{signatureType: msgType}
	case openFlowType:
		m = &OpenFlow{}
	case releaseType:
		m = &Release{}
	case healthCheckRequestType:
		m = &HealthCheckRequest{}
	case healthCheckResponseType:
		m = &HealthCheckResponse{}
	case tearDownType:
		m = &TearDown{}
	case enterLameDuckType:
		m = &EnterLameDuck{}
	case ackLameDuckType:
		m = &AckLameDuck{}
	case multiProxyType:
		m = &MultiProxyRequest{}
	case proxyServerType:
		m = &ProxyServerRequest{}
	case proxyResponseType:
		m = &ProxyResponse{}
	case proxyErrorReponseType:
		m = &ProxyErrorResponse{}
	default:
		return nil, ErrUnknownMsg.Errorf(ctx, "unknown message type: %02x", msgType)
	}
	return m, m.read(ctx, from)
}

// CopyBuffers must be called to make internal copies of any byte slices that
// may be in a shared underlying slice (eg. in a slice allocated for decryting
// a message). It should be called before passing a Message to another
// goroutine for example.
func CopyBuffers(m Message) {
	m.copyBuffers()
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

// PlaintextPayload returns the payload for messages which have the
// DisableEncryptionFlag set and the value of that flag.
// PlaintextPayload should only be called on messages that have been created
// using the Append function of this package to obtain the payload to be
// written separately to the oroginal message.
// If the DisableEncryptionFlag is set, but the payload is of zero length,
// then the flag must be cleared so that the receiver knows not to expect
// a separate frame. This is a backwards compatible change.
func PlaintextPayload(m Message) ([][]byte, bool) {
	switch msg := m.(type) {
	case *Data:
		if msg.Flags&DisableEncryptionFlag != 0 {
			return msg.Payload, true
		}
	case *OpenFlow:
		if msg.Flags&DisableEncryptionFlag != 0 {
			return msg.Payload, true
		}
	}
	return nil, false
}

// ClearDisableEncryptionFlag clears the DisableEncryptionFlag encryption
// flag.
func ClearDisableEncryptionFlag(m Message) {
	switch msg := m.(type) {
	case *Data:
		msg.Flags &^= DisableEncryptionFlag
	case *OpenFlow:
		msg.Flags &^= DisableEncryptionFlag
	}
}

// ExpectsPlaintextPayload returns true if a mesage has a DisableEncryptionFlag
// set, false if it is not set or not supported by the message.
// ExpectsPlaintextPayload should be called after the Read function from
// this package to determine if a subsequent plaintext message frame should
// be read.
func ExpectsPlaintextPayload(m Message) bool {
	switch msg := m.(type) {
	case *Data:
		return msg.Flags&DisableEncryptionFlag != 0
	case *OpenFlow:
		return msg.Flags&DisableEncryptionFlag != 0
	}
	return false
}

// SetPlaintextPayload is used to associate a payload that was sent in the clear
// (following a message with the DisableEncryptionFlag flag set) with the
// immediately preceding received message. The 'nocopy' parameter indicates
// whether a subsequent call to CopyBuffers needs to copy the payload.
func SetPlaintextPayload(m Message, payload []byte, nocopy bool) {
	switch msg := m.(type) {
	case *Data:
		msg.Payload = [][]byte{payload}
		msg.nocopy = nocopy
	case *OpenFlow:
		msg.Payload = [][]byte{payload}
		msg.nocopy = nocopy
	}
}

// Message is implemented by all low-level messages defined by this package.
type Message interface {
	append(ctx *context.T, data []byte) ([]byte, error)
	read(ctx *context.T, data []byte) error
	copyBuffers()
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
	authED25519Type
	authRSAType
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

func (m *Setup) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, setupType)
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

func (m *Setup) read(ctx *context.T, orig []byte) error {
	var (
		data  = orig
		valid bool
		v     uint64
	)
	if v, data, valid = readVarUint64(data); !valid {
		return NewErrInvalidMsg(ctx, setupType, uint64(len(orig)), 0, nil)
	}
	m.Versions.Min = version.RPCVersion(v)
	if v, data, valid = readVarUint64(data); !valid {
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
			if mtu, _, valid := readVarUint64(payload); valid {
				m.Mtu = mtu
			} else {
				return ErrInvalidSetupOption.Errorf(ctx, "setup option: %v failed decoding at field: %v", opt, field)
			}
		case sharedTokensOption:
			if t, _, valid := readVarUint64(payload); valid {
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

func (m *Setup) copyBuffers() {
	for i, v := range m.uninterpretedOptions {
		m.uninterpretedOptions[i].payload = copyBytes(v.payload)
	}
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

func (m *TearDown) copyBuffers() {}

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

func (m *EnterLameDuck) copyBuffers() {}

// AckLameDuck is sent in response to an EnterLameDuck message.  After
// this message is received no more OpenFlow messages should arrive.
type AckLameDuck struct{}

func (m *AckLameDuck) append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, ackLameDuckType), nil
}

func (m *AckLameDuck) read(ctx *context.T, data []byte) error {
	return nil
}

func (m *AckLameDuck) copyBuffers() {}

// Auth is used to complete the auth handshake.
type Auth struct {
	signatureType              byte
	BlessingsKey, DischargeKey uint64
	ChannelBinding             security.Signature
}

func (m *Auth) appendCommon(data []byte) []byte {
	data = writeVarUint64(m.BlessingsKey, data)
	data = writeVarUint64(m.DischargeKey, data)
	data = appendLenBytes(m.ChannelBinding.Purpose, data)
	data = appendLenBytes([]byte(m.ChannelBinding.Hash), data)
	return data
}

func (m *Auth) append(ctx *context.T, data []byte) ([]byte, error) {
	switch {
	case len(m.ChannelBinding.Ed25519) > 0:
		data = append(data, authED25519Type)
		data = m.appendCommon(data)
		data = appendLenBytes(m.ChannelBinding.Ed25519, data)
	case len(m.ChannelBinding.Rsa) > 0:
		data = append(data, authRSAType)
		data = m.appendCommon(data)
		data = appendLenBytes(m.ChannelBinding.Rsa, data)
	default:
		data = append(data, authType)
		data = m.appendCommon(data)
		data = appendLenBytes(m.ChannelBinding.R, data)
		data = appendLenBytes(m.ChannelBinding.S, data)
	}
	return data, nil
}

func (m *Auth) read(ctx *context.T, orig []byte) error {
	var data, tmp []byte
	var valid bool
	if m.BlessingsKey, data, valid = readVarUint64(orig); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 0, nil)
	}
	if m.BlessingsKey == 0 {
		return ErrMissingBlessings.Errorf(ctx, "%02x: message received with no blessings", openFlowType)
	}
	if m.DischargeKey, data, valid = readVarUint64(data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 1, nil)
	}
	if m.ChannelBinding.Purpose, data, valid = readLenBytes(data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 2, nil)
	}
	if tmp, data, valid = readLenBytes(data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 3, nil)
	}
	m.ChannelBinding.Hash = security.Hash(tmp)
	switch m.signatureType {
	case authType:
		if m.ChannelBinding.R, data, valid = readLenBytes(data); !valid {
			return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 4, nil)
		}
		if m.ChannelBinding.S, _, _ = readLenBytes(data); !valid {
			return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 5, nil)
		}
	case authED25519Type:
		if m.ChannelBinding.Ed25519, _, _ = readLenBytes(data); !valid {
			return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 4, nil)
		}
	case authRSAType:
		if m.ChannelBinding.Rsa, _, _ = readLenBytes(data); !valid {
			return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 4, nil)
		}
	}
	return nil
}

func (m *Auth) copyBuffers() {
	m.ChannelBinding.Purpose = copyBytes(m.ChannelBinding.Purpose)
	switch m.signatureType {
	case authType:
		m.ChannelBinding.R = copyBytes(m.ChannelBinding.R)
		m.ChannelBinding.S = copyBytes(m.ChannelBinding.S)
	case authED25519Type:
		m.ChannelBinding.Ed25519 = copyBytes(m.ChannelBinding.Ed25519)
	case authRSAType:
		m.ChannelBinding.Rsa = copyBytes(m.ChannelBinding.Rsa)
	}
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
	if m.ID, data, valid = readVarUint64(orig); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 0, nil)
	}
	if m.InitialCounters, data, valid = readVarUint64(data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 1, nil)
	}
	if m.BlessingsKey, data, valid = readVarUint64(data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 2, nil)
	}
	if m.BlessingsKey == 0 {
		return ErrMissingBlessings.Errorf(ctx, "%02x: message received with no blessings", openFlowType)
	}
	if m.DischargeKey, data, valid = readVarUint64(data); !valid {
		return NewErrInvalidMsg(ctx, openFlowType, uint64(len(orig)), 3, nil)
	}
	if m.Flags, data, valid = readVarUint64(data); !valid {
		return NewErrInvalidMsg(ctx, dataType, uint64(len(orig)), 1, nil)
	}
	if m.Flags&DisableEncryptionFlag == 0 && len(data) > 0 {
		m.Payload = [][]byte{data}
	}
	return nil
}

func (m *OpenFlow) copyBuffers() {
	if m.nocopy {
		return
	}
	for i, v := range m.Payload {
		m.Payload[i] = copyBytes(v)
	}
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
		if fid, data, valid = readVarUint64(data); !valid {
			return NewErrInvalidMsg(ctx, releaseType, uint64(len(orig)), n, nil)
		}
		if val, data, valid = readVarUint64(data); !valid {
			return NewErrInvalidMsg(ctx, releaseType, uint64(len(orig)), n+1, nil)
		}
		m.Counters[fid] = val
		n += 2
	}
	return nil
}

func (m *Release) copyBuffers() {}

func (m *Release) String() string {
	return fmt.Sprintf("release #%v counters", len(m.Counters))
}

// Data carries encrypted data for a specific flow.
type Data struct {
	ID      uint64
	Flags   uint64
	Payload [][]byte
	nocopy  bool
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
	if m.ID, data, valid = readVarUint64(orig); !valid {
		return NewErrInvalidMsg(ctx, dataType, uint64(len(orig)), 0, nil)
	}
	if m.Flags, data, valid = readVarUint64(data); !valid {
		return NewErrInvalidMsg(ctx, dataType, uint64(len(orig)), 1, nil)
	}
	m.Payload = nil
	if m.Flags&DisableEncryptionFlag == 0 && len(data) > 0 {
		m.Payload = [][]byte{data}
	}
	return nil
}

func (m *Data) copyBuffers() {
	if m.nocopy {
		return
	}
	for i, v := range m.Payload {
		m.Payload[i] = copyBytes(v)
	}
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

func (m *MultiProxyRequest) copyBuffers() {}

// ProxyServerRequest is sent when a server wants to listen through a proxy.
type ProxyServerRequest struct{}

func (m *ProxyServerRequest) append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, proxyServerType), nil
}

func (m *ProxyServerRequest) read(ctx *context.T, orig []byte) error {
	return nil
}

func (m *ProxyServerRequest) copyBuffers() {}

// ProxyResponse is sent by a proxy in response to a ProxyServerRequest or
// MultiProxyRequest. It notifies the server of the endpoints it should publish.
// Or, it notifies a requesting proxy of the endpoints it accepts connections from.
type ProxyResponse struct {
	Endpoints []naming.Endpoint
}

func (m *ProxyResponse) append(ctx *context.T, data []byte) ([]byte, error) {
	data = append(data, proxyResponseType)
	for _, ep := range m.Endpoints {
		data = appendLenString(ep.String(), data)
	}
	return data, nil
}

func (m *ProxyResponse) read(ctx *context.T, orig []byte) error {
	var (
		data    = orig
		epBytes []byte
		valid   bool
	)
	if len(data) == 0 {
		return nil
	}
	m.Endpoints = make([]naming.Endpoint, 0, len(data))
	for i := 0; len(data) > 0; i++ {
		if epBytes, data, valid = readLenBytes(data); !valid {
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

func (m *ProxyResponse) copyBuffers() {}

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

func (m *ProxyErrorResponse) copyBuffers() {}

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

func (m *HealthCheckRequest) copyBuffers() {}

// HealthCheckResponse messages are sent in response to HealthCheckRequest messages.
type HealthCheckResponse struct{}

func (m *HealthCheckResponse) append(ctx *context.T, data []byte) ([]byte, error) {
	return append(data, healthCheckResponseType), nil
}

func (m *HealthCheckResponse) read(ctx *context.T, data []byte) error {
	return nil
}

func (m *HealthCheckResponse) copyBuffers() {}

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
