// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package conn

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"testing"

	"v.io/v23/context"
	"v.io/v23/flow/message"
	"v.io/v23/naming"
	"v.io/v23/rpc/version"
	"v.io/x/ref/runtime/internal/flow/cipher/aead"
	"v.io/x/ref/runtime/internal/flow/cipher/naclbox"
	"v.io/x/ref/runtime/internal/flow/flowtest"
	"v.io/x/ref/runtime/internal/test/cipher"
	"v.io/x/ref/test"
)

type keyset struct {
	pk1, sk1, pk2, sk2 *[32]byte
}

func (ks *keyset) initUsing(fn func() (pk1, sk1, pk2, sk2 *[32]byte, err error)) {
	var err error
	ks.pk1, ks.sk1, ks.pk2, ks.sk2, err = fn()
	if err != nil {
		panic(err)
	}
}

var (
	rpc11Keyset keyset
	rpc15Keyset keyset
	mixedKeyset keyset
)

func init() {
	rpc11Keyset.initUsing(cipher.NewRPC11Keys)
	rpc15Keyset.initUsing(cipher.NewRPC15Keys)
	mixedKeyset.initUsing(cipher.NewMixedKeys)
}

func TestMessagePipeRPC11(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	testMessagePipesVersioned(t, ctx, "local", rpc11Keyset, version.RPCVersion11)
	// framing will be bypassed for the tcp connections.
	testMessagePipesVersioned(t, ctx, "tcp", rpc11Keyset, version.RPCVersion11)
}

func TestMessagePipesRPC15(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	testMessagePipesVersioned(t, ctx, "local", rpc15Keyset, version.RPCVersion15)
	// framing will be bypassed for the tcp connections.
	testMessagePipesVersioned(t, ctx, "tcp", rpc15Keyset, version.RPCVersion15)
}

func newPipes(t *testing.T, ctx *context.T, protocol string) (dialed, accepted *messagePipe) {
	d, a := flowtest.Pipe(t, ctx, protocol, "")
	return newMessagePipe(d), newMessagePipe(a)
}

func testMessagePipesVersioned(t *testing.T, ctx *context.T, protocol string, ks keyset, version version.RPCVersion) {
	dialed, accepted := newPipes(t, ctx, protocol)
	// plaintext
	testMessageRoundTrip(t, ctx, dialed, accepted)
	testManyMessages(t, ctx, dialed, accepted, 100*1024*1024)

	if err := enableEncryption(ctx, dialed, accepted, ks, version); err != nil {
		t.Fatal(err)
	}

	// encrypted
	testMessageRoundTrip(t, ctx, dialed, accepted)
	testManyMessages(t, ctx, dialed, accepted, 100*1024*1024)

	// ensure the messages are encrypted by independently decrypting them.
	testMessageEncryption(t, ctx, "local", ks, version)

}

func testMessageEncryption(t *testing.T, ctx *context.T, protocol string, ks keyset, rpcversion version.RPCVersion) {

	// Test that messages are encrypted.
	in, out := flowtest.Pipe(t, ctx, protocol, "")
	dialedPipe, acceptedPipe := newMessagePipe(in), newMessagePipe(out)
	if err := enableEncryption(ctx, dialedPipe, acceptedPipe, ks, rpcversion); err != nil {
		t.Fatal(err)
	}

	var openFunc func(out, data []byte) ([]byte, bool)

	switch rpcversion {
	case version.RPCVersion11, version.RPCVersion12, version.RPCVersion13, version.RPCVersion14:
		cipher, err := naclbox.NewCipher(ks.pk2, ks.sk2, ks.pk1)
		if err != nil {
			t.Fatal(err)
		}
		openFunc = cipher.Open
	case version.RPCVersion15:
		cipher, err := aead.NewCipher(ks.pk2, ks.sk2, ks.pk1)
		if err != nil {
			t.Fatal(err)
		}
		openFunc = cipher.Open
	}

	errCh := make(chan error, 1)
	bufCh := make(chan []byte, 1)
	for _, m := range testMessages(t) {
		go func(m message.Message) {
			buf, err := out.ReadMsg()
			errCh <- err
			bufCh <- buf
			// clear out the unencrypted payloads.
			switch msg := m.(type) {
			case message.Data:
				if msg.Flags&message.DisableEncryptionFlag != 0 {
					out.ReadMsg()
				}
			case message.OpenFlow:
				if msg.Flags&message.DisableEncryptionFlag != 0 {
					out.ReadMsg()
				}
			}
		}(m)
		if err := writeToMessagePipe(ctx, dialedPipe, m); err != nil {
			t.Fatal(err)
		}
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
		buf := <-bufCh

		_, ok := openFunc(make([]byte, 0, 100), buf)
		if !ok {
			t.Fatalf("message likely not encrypted!")
		}
	}

}

func testManyMessages(t *testing.T, ctx *context.T, dialedPipe, acceptedPipe *messagePipe, size int) {

	payload := make([]byte, size)
	_, err := io.ReadFull(rand.Reader, payload)
	if err != nil {
		t.Fatal(err)
	}

	for _, rxbuf := range [][]byte{nil, make([]byte, DefaultMTU)} {

		received, txErr, rxErr := runMany(ctx, dialedPipe, acceptedPipe, rxbuf, payload)

		if err := txErr; err != nil {
			t.Fatal(err)
		}
		if err := rxErr; err != nil {
			t.Fatal(err)
		}

		if got, want := payload, received; !bytes.Equal(got, want) {
			t.Errorf("data mismatch")
		}
	}

}

func runMany(ctx *context.T, dialedPipe, acceptedPipe *messagePipe, rxbuf, payload []byte) (received []byte, writeErr, readErr error) {
	errCh := make(chan error, 2)
	go func() {
		sent := 0
		for sent < len(payload) {
			payload := payload[sent:]
			if len(payload) > DefaultMTU {
				payload = payload[:DefaultMTU]
			}
			msg := message.Data{ID: 1123, Payload: payload}
			err := dialedPipe.writeData(ctx, msg)
			if err != nil {
				errCh <- err
				return
			}
			sent += len(payload)
		}
		errCh <- nil
	}()

	go func() {
		for {
			m, err := acceptedPipe.readAnyMsg(ctx, rxbuf)
			if err != nil {
				errCh <- err
				return
			}
			m = m.Copy()
			received = append(received, m.(message.Data).Payload...)
			if len(received) == len(payload) {
				break
			}
		}
		errCh <- nil
	}()

	writeErr = <-errCh
	readErr = <-errCh
	return
}

func enableEncryption(ctx *context.T, dialed, accepted *messagePipe, ks keyset, version version.RPCVersion) error {
	b1, err := dialed.enableEncryption(ctx, ks.pk1, ks.sk1, ks.pk2, version)
	if err != nil {
		return fmt.Errorf("can't enabled encryption %v", err)
	}
	b2, err := accepted.enableEncryption(ctx, ks.pk2, ks.sk2, ks.pk1, version)
	if err != nil {
		return fmt.Errorf("can't enabled encryption %v", err)
	}
	if got, want := b1, b2; !bytes.Equal(got, want) {
		return fmt.Errorf("bindings differ: got %v, want %v", got, want)
	}
	return nil
}

func testMessages(t *testing.T) []message.Message {
	largePayload := make([]byte, 2*DefaultMTU)
	_, err := io.ReadFull(rand.Reader, largePayload)
	if err != nil {
		t.Fatal(err)
	}
	ep1, err := naming.ParseEndpoint(
		"@6@tcp@foo.com:1234@a,b@00112233445566778899aabbccddeeff@m@v.io/foo")
	if err != nil {
		t.Fatal(err)
	}
	ep2, err := naming.ParseEndpoint(
		"@6@tcp@bar.com:1234@a,b@00112233445566778899aabbccddeeff@m@v.io/bar")
	if err != nil {
		t.Fatal(err)
	}
	return []message.Message{
		message.OpenFlow{
			ID:              23,
			InitialCounters: 1 << 20,
			BlessingsKey:    42,
			DischargeKey:    55,
			Flags:           message.CloseFlag,
			Payload:         []byte("fake payload"),
		},
		message.OpenFlow{ID: 23, InitialCounters: 1 << 20, BlessingsKey: 42, DischargeKey: 55},
		message.OpenFlow{ID: 23, Flags: message.DisableEncryptionFlag,
			InitialCounters: 1 << 18, BlessingsKey: 42, DischargeKey: 55,
			Payload: []byte("fake payload"),
		},

		message.Setup{Versions: version.RPCVersionRange{Min: 3, Max: 5}},
		message.Setup{
			Versions: version.RPCVersionRange{Min: 3, Max: 5},
			PeerNaClPublicKey: [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13,
				14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
			PeerRemoteEndpoint: ep1,
			PeerLocalEndpoint:  ep2,
		},
		message.Setup{
			Versions:     version.RPCVersionRange{Min: 3, Max: 5},
			Mtu:          1 << 16,
			SharedTokens: 1 << 20,
		},
		message.Data{ID: 1123, Flags: message.CloseFlag, Payload: []byte("fake payload")},
		message.Data{ID: 1123, Flags: message.CloseFlag, Payload: largePayload},

		message.Data{},
		message.Data{ID: 1123, Flags: message.DisableEncryptionFlag, Payload: []byte("fake payload")},
		message.Data{ID: 1123, Flags: message.DisableEncryptionFlag, Payload: largePayload},
	}
}

func testMessageRoundTrip(t *testing.T, ctx *context.T, dialed, accepted *messagePipe) {
	for _, m := range testMessages(t) {
		messageRoundTrip(t, ctx, dialed, accepted, m)
	}
}

func writeToMessagePipe(ctx *context.T, mp *messagePipe, m message.Message) error {
	switch msg := m.(type) {
	case message.Data:
		return mp.writeData(ctx, msg)
	case message.OpenFlow:
		return mp.writeOpenFlow(ctx, msg)
	case message.Release:
		return mp.writeRelease(ctx, msg)
	default:
		return mp.writeAnyMsg(ctx, m.Append)
	}
}

func messageRoundTrip(t *testing.T, ctx *context.T, dialed, accepted *messagePipe, m message.Message) {
	var err error
	assert := func() {
		if err != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Fatalf("line: %v: err %v", line, err)
		}
	}
	errCh := make(chan error, 1)
	msgCh := make(chan message.Message, 1)
	reader := func(mp *messagePipe) {
		m, err := mp.readAnyMsg(ctx, nil)
		errCh <- err
		msgCh <- m
	}

	go reader(accepted)
	writeToMessagePipe(ctx, dialed, m)
	assert()
	err = <-errCh
	assert()
	acceptedMessage := <-msgCh
	go reader(dialed)
	err = writeToMessagePipe(ctx, accepted, acceptedMessage)
	assert()
	err = <-errCh
	assert()
	responseMessage := <-msgCh

	// Mimic the handling of plaintext playloads.
	switch msg := m.(type) {
	case message.Data:
		if msg.Flags&message.DisableEncryptionFlag == 0 {
			break
		}
		m = msg.SetNoCopy(true)

	case message.OpenFlow:
		if msg.Flags&message.DisableEncryptionFlag == 0 {
			break
		}
		m = msg.SetNoCopy(true)
	}

	if got, want := acceptedMessage, m; !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
	if got, want := responseMessage, m; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func runMessagePipeBenchmark(b *testing.B, ctx *context.T, dialed, accepted *messagePipe, rxbuf []byte, payload []byte) {
	errCh := make(chan error, 1)

	msg := message.Data{ID: 1123, Payload: payload}

	go func() {
		for i := 0; i < b.N; i++ {
			if err := dialed.writeData(ctx, msg); err != nil {
				errCh <- err
				return
			}
		}
		errCh <- nil
	}()

	for i := 0; i < b.N; i++ {
		_, err := accepted.readAnyMsg(ctx, rxbuf)
		if err != nil {
			b.Fatal(err)
		}
	}

	if err := <-errCh; err != nil {
		b.Fatal(err)
	}
}

func benchmarkMessagePipe(b *testing.B, allowFramerBypass bool, size int, userxbuf bool, ks keyset, rpcversion version.RPCVersion) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	payload := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, payload); err != nil {
		b.Fatal(err)
	}

	var rxbuf []byte
	if userxbuf {
		rxbuf = make([]byte, size+2048)
	}

	d, a, err := flowtest.NewPipe(ctx, "tcp", "")
	if err != nil {
		b.Fatal(err)
	}
	var dialed, accepted *messagePipe
	if allowFramerBypass {
		dialed, accepted = newMessagePipe(d), newMessagePipe(a)
	} else {
		dialed, accepted = newMessagePipeUseFramer(d), newMessagePipeUseFramer(a)

	}

	if err := enableEncryption(ctx, dialed, accepted, ks, rpcversion); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.SetBytes(int64(size) * 2)
	runMessagePipeBenchmark(b, ctx, dialed, accepted, rxbuf, payload)
}

func BenchmarkMessagePipe__RPC11__NewBuf__UseFramer____1KB(b *testing.B) {
	benchmarkMessagePipe(b, false, 1000, false, rpc11Keyset, version.RPCVersion11)
}

func BenchmarkMessagePipe__RPC11__NewBuf__UseFramer____MTU(b *testing.B) {
	benchmarkMessagePipe(b, false, DefaultMTU, false, rpc11Keyset, version.RPCVersion11)
}

func BenchmarkMessagePipe__RPC11__UseBuf__UseFramer____1KB(b *testing.B) {
	benchmarkMessagePipe(b, false, 1000, true, rpc11Keyset, version.RPCVersion11)
}

func BenchmarkMessagePipe__RPC11__UseBuf__UseFramer____MTU(b *testing.B) {
	benchmarkMessagePipe(b, false, DefaultMTU, true, rpc11Keyset, version.RPCVersion11)
}

func BenchmarkMessagePipe__RPC15__NewBuf__UseFramer____1KB(b *testing.B) {
	benchmarkMessagePipe(b, false, 1000, false, rpc15Keyset, version.RPCVersion15)
}

func BenchmarkMessagePipe__RPC15__NewBuf__UseFramer____MTU(b *testing.B) {
	benchmarkMessagePipe(b, false, DefaultMTU, false, rpc15Keyset, version.RPCVersion15)
}

func BenchmarkMessagePipe__RPC15__UseBuf__UseFramer____1KB(b *testing.B) {
	benchmarkMessagePipe(b, false, 1000, true, rpc15Keyset, version.RPCVersion15)
}

func BenchmarkMessagePipe__RPC15__UseBuf__UseFramer____MTU(b *testing.B) {
	benchmarkMessagePipe(b, false, DefaultMTU, true, rpc15Keyset, version.RPCVersion15)
}

func BenchmarkMessagePipe__RPC11__NewBuf__BypassFramer__1KB(b *testing.B) {
	benchmarkMessagePipe(b, true, 1000, false, rpc11Keyset, version.RPCVersion11)
}

func BenchmarkMessagePipe__RPC11__NewBuf__BypassFramer__MTU(b *testing.B) {
	benchmarkMessagePipe(b, true, DefaultMTU, false, rpc11Keyset, version.RPCVersion11)
}

func BenchmarkMessagePipe__RPC11__UseBuf__BypassFramer__1KB(b *testing.B) {
	benchmarkMessagePipe(b, true, 1000, true, rpc11Keyset, version.RPCVersion11)
}

func BenchmarkMessagePipe__RPC11__UseBuf__BypassFramer__MTU(b *testing.B) {
	benchmarkMessagePipe(b, true, DefaultMTU, true, rpc11Keyset, version.RPCVersion11)
}

func BenchmarkMessagePipe__RPC15__NewBuf__BypassFramer__1KB(b *testing.B) {
	benchmarkMessagePipe(b, true, 1000, false, rpc15Keyset, version.RPCVersion15)
}

func BenchmarkMessagePipe__RPC15__NewBuf__BypassFramer__MTU(b *testing.B) {
	benchmarkMessagePipe(b, true, DefaultMTU, false, rpc15Keyset, version.RPCVersion15)
}

func BenchmarkMessagePipe__RPC15__UseBuf__BypassFramer__1KB(b *testing.B) {
	benchmarkMessagePipe(b, true, 1000, true, rpc15Keyset, version.RPCVersion15)
}

func BenchmarkMessagePipe__RPC15__UseBuf__BypassFramer__MTU(b *testing.B) {
	benchmarkMessagePipe(b, true, DefaultMTU, true, rpc15Keyset, version.RPCVersion15)
}
