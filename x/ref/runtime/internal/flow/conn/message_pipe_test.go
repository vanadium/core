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

	"golang.org/x/crypto/nacl/box"
	"v.io/v23/context"
	"v.io/v23/flow/message"
	"v.io/v23/naming"
	"v.io/v23/rpc/version"
	"v.io/x/ref/runtime/internal/flow/flowtest"
	"v.io/x/ref/test"
)

func TestMessagePipes(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	testMessagePipesRPC11(t, ctx)
}

func newPipes(t *testing.T, ctx *context.T) (dialed, accepted *messagePipe) {
	d, a := flowtest.Pipe(t, ctx, "local", "")
	return newMessagePipe(d), newMessagePipe(a)
}

func testMessagePipesRPC11(t *testing.T, ctx *context.T) {
	dialed, accepted := newPipes(t, ctx)
	// plaintext
	testMessagePipes(t, ctx, dialed, accepted)

	if err := enableRPC11Encryption(ctx, dialed.(*messagePipeRPC11), accepted.(*messagePipeRPC11)); err != nil {
		t.Fatal(err)
	}

	testMessagePipes(t, ctx, dialed, accepted)

	// ensure the messages are encrypted
	testMessageEncryption(t, ctx)

	// test multiple data messages.
	testManyMessages(t, ctx, 100*1024*1024)
}

func testMessageEncryption(t *testing.T, ctx *context.T) {
	// Test that messages are encrypted.
	in, out := flowtest.Pipe(t, ctx, "local", "")
	dialedPipe, acceptedPipe := newMessagePipeRPC11(in), newMessagePipeRPC11(out)
	if err := enableRPC11Encryption(ctx, dialedPipe, acceptedPipe); err != nil {
		t.Fatal(err)
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
			case *message.Data:
				if msg.Flags&message.DisableEncryptionFlag != 0 {
					out.ReadMsg()
				}
			case *message.OpenFlow:
				if msg.Flags&message.DisableEncryptionFlag != 0 {
					out.ReadMsg()
				}
			}
		}(m)
		if err := dialedPipe.writeMsg(ctx, m); err != nil {
			t.Fatal(err)
		}
		if err := <-errCh; err != nil {
			t.Fatal(err)
		}
		buf := <-bufCh
		_, ok := acceptedPipe.cipher.Open(make([]byte, 0, 100), buf)
		if !ok {
			t.Fatalf("message likely not encrypted!")
		}
	}
}

func testManyMessages(t *testing.T, ctx *context.T, size int) {
	dialed, accepted := flowtest.Pipe(t, ctx, "tcp", "")
	dialedPipe, acceptedPipe := newMessagePipeRPC11(dialed), newMessagePipeRPC11(accepted)
	if err := enableRPC11Encryption(ctx, dialedPipe, acceptedPipe); err != nil {
		t.Fatal(err)
	}

	payload := make([]byte, size)
	_, err := io.ReadFull(rand.Reader, payload)
	if err != nil {
		t.Fatal(err)
	}

	received, txErr, rxErr := runMany(ctx, dialedPipe, acceptedPipe, payload)

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

func runMany(ctx *context.T, dialedPipe, acceptedPipe *messagePipeRPC11, payload []byte) (received []byte, writeErr, readErr error) {
	errCh := make(chan error, 2)
	go func() {
		sent := 0
		for sent < len(payload) {
			payload := payload[sent:]
			if len(payload) > defaultMTU {
				payload = payload[:defaultMTU]
			}
			msg := &message.Data{ID: 1123, Payload: [][]byte{payload}}
			err := dialedPipe.writeMsg(ctx, msg)
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
			m, err := acceptedPipe.readMsg(ctx)
			if err != nil {
				errCh <- err
				return
			}
			received = append(received, m.(*message.Data).Payload[0]...)
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

func enableRPC11Encryption(ctx *context.T, dialed, accepted *messagePipeRPC11) error {
	pk1, sk1, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("can't generate key: %v", err)
	}
	pk2, sk2, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("can't generate key: %v", err)
	}
	b1, err := dialed.enableEncryption(ctx, pk1, sk1, pk2)
	if err != nil {
		return fmt.Errorf("can't enabled encryption %v", err)
	}
	b2, err := accepted.enableEncryption(ctx, pk2, sk2, pk1)
	if err != nil {
		return fmt.Errorf("can't enabled encryption %v", err)
	}
	if got, want := b1, b2; !bytes.Equal(got, want) {
		return fmt.Errorf("bindings differ: got %v, want %v", got, want)
	}
	return nil
}

func testMessages(t *testing.T) []message.Message {
	largePayload := make([]byte, 2*defaultMTU)
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
		&message.OpenFlow{
			ID:              23,
			InitialCounters: 1 << 20,
			BlessingsKey:    42,
			DischargeKey:    55,
			Flags:           message.CloseFlag,
			Payload:         [][]byte{[]byte("fake payload")},
		},
		&message.OpenFlow{ID: 23, InitialCounters: 1 << 20, BlessingsKey: 42, DischargeKey: 55},
		&message.OpenFlow{ID: 23, Flags: message.DisableEncryptionFlag,
			InitialCounters: 1 << 20, BlessingsKey: 42, DischargeKey: 55,
			Payload: [][]byte{[]byte("fake payload")},
		},

		&message.Setup{Versions: version.RPCVersionRange{Min: 3, Max: 5}},
		&message.Setup{
			Versions: version.RPCVersionRange{Min: 3, Max: 5},
			PeerNaClPublicKey: &[32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13,
				14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
			PeerRemoteEndpoint: ep1,
			PeerLocalEndpoint:  ep2,
		},
		&message.Setup{
			Versions:     version.RPCVersionRange{Min: 3, Max: 5},
			Mtu:          1 << 16,
			SharedTokens: 1 << 20,
		},
		&message.Data{ID: 1123, Flags: message.CloseFlag, Payload: [][]byte{[]byte("fake payload")}},
		&message.Data{ID: 1123, Flags: message.CloseFlag, Payload: [][]byte{largePayload}},

		&message.Data{},
		&message.Data{ID: 1123, Flags: message.DisableEncryptionFlag, Payload: [][]byte{[]byte("fake payload")}},
		&message.Data{ID: 1123, Flags: message.DisableEncryptionFlag, Payload: [][]byte{largePayload}},
	}
}

func testMessagePipes(t *testing.T, ctx *context.T, dialed, accepted messagePipeAPI) {
	messages := testMessages(t)
	for _, m := range messages {
		messageRoundTrip(t, ctx, dialed, accepted, m)
	}
}

func messageRoundTrip(t *testing.T, ctx *context.T, dialed, accepted messagePipeAPI, m message.Message) {
	var err error
	assert := func() {
		if err != nil {
			_, _, line, _ := runtime.Caller(1)
			t.Fatalf("line: %v: err %v", line, err)
		}
	}
	errCh := make(chan error, 1)
	msgCh := make(chan message.Message, 1)
	reader := func(mp messagePipeAPI) {
		m, err := mp.readMsg(ctx)
		errCh <- err
		msgCh <- m
	}

	go reader(accepted)
	err = dialed.writeMsg(ctx, m)
	assert()
	err = <-errCh
	assert()
	acceptedMessage := <-msgCh

	go reader(dialed)
	err = accepted.writeMsg(ctx, acceptedMessage)
	assert()
	err = <-errCh
	assert()
	responseMessage := <-msgCh

	if got, want := m, acceptedMessage; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := m, responseMessage; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func BenchmarkMessagePipeSend(b *testing.B) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	payload := make([]byte, 1024)
	if _, err := io.ReadFull(rand.Reader, payload); err != nil {
		b.Fatal(err)
	}

	dialed, accepted, err := flowtest.NewPipe(ctx, "tcp", "")
	if err != nil {
		b.Fatal(err)
	}

	dialedPipe, acceptedPipe := newMessagePipeRPC11(dialed), newMessagePipeRPC11(accepted)
	if err := enableRPC11Encryption(ctx, dialedPipe, acceptedPipe); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := &message.Data{ID: 1123, Payload: [][]byte{payload}}
		if err := dialedPipe.writeMsg(ctx, msg); err != nil {
			b.Fatal(err)
		}
		if _, err := acceptedPipe.readMsg(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessagePipeCreate(b *testing.B) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	dialed, accepted, err := flowtest.NewPipe(ctx, "tcp", "")
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dialedPipe, acceptedPipe := newMessagePipeRPC11(dialed), newMessagePipeRPC11(accepted)
		if err := enableRPC11Encryption(ctx, dialedPipe, acceptedPipe); err != nil {
			b.Fatal(err)
		}
	}
}
