// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package basics

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"
)

func BenchmarkTLSConnectionEstablishment(b *testing.B) {
	client, server, err := newTLSConfigs()
	if err != nil {
		b.Fatal(err)
	}
	ln, err := tls.Listen("tcp4", "127.0.0.1:0", server)
	if err != nil {
		b.Fatal(err)
	}

	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	defer func() {
		ln.Close()
		wg.Wait()
		close(errCh)
		for err := range errCh {
			if err != nil {
				b.Fatal(err)
			}
		}
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 1)
		for {
			c, err := ln.Accept()
			if err != nil {
				errCh <- nil
				return
			}
			if n, err := c.Read(buf); n != 1 {
				errCh <- fmt.Errorf("Got (%d, %v), expected (1, <nil or io.EOF>)", n, err)
				return
			}
			if _, err := c.Write(buf); err != nil {
				errCh <- nil
				return
			}
			c.Close()
		}
	}()
	network, addr := ln.Addr().Network(), ln.Addr().String()
	buf := []byte{'A'}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c, err := tls.Dial(network, addr, client)
		if err != nil {
			b.Fatalf("%d/%d: %v", i, b.N, err)
		}
		if _, err := c.Write(buf); err != nil {
			b.Fatalf("%d/%d: %v", i, b.N, err)
		}
		buf[0] = 'B'
		if n, err := c.Read(buf); n != 1 {
			b.Fatalf("%d/%d: Got (%d, %v), want (1, <nil or io.EOF>)", i, b.N, n, err)
		}
		if buf[0] != 'A' {
			b.Fatalf("%d/%d: Got %d, want %d", i, b.N, buf[0], 'A')
		}
		// Close the connection to prevent hitting the ulimit on open
		// file descriptors.
		if err := c.Close(); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()

}

// Ideally this benchmark wouldn't be required having just
// BenchmarkTLSConnectionEstablishment would suffice - the two should have very
// similar run times.
//
// Hoever, on some machines I was having trouble getting consistent numbers
// with BenchmarkTLSConnectionEstablishment - possibly because of some
// network monitoring software installed on those. Till that is figured out,
// have this additional benchmark that measures just the CPU work done for
// mutual TLS authentication.
func BenchmarkTLSHandshake(b *testing.B) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	clientCfg, serverCfg, err := newTLSConfigs()
	if err != nil {
		b.Fatal(err)
	}
	ch := make(chan *tls.Conn)
	go func() {
		for c := range ch {
			c.Handshake() //nolint:errcheck
		}
	}()
	defer close(ch)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ch <- tls.Client(client, clientCfg)
		serverConn := tls.Server(server, serverCfg)
		// Use the server's Handshake and not the client's here since
		// the TLS handshake ends with the server verifying the
		// client's certificate. If we used the client's Handshake then
		// server verification time would be excluded from this loop.
		if err := serverConn.Handshake(); err != nil {
			b.Fatal(err)
		}
		if !serverConn.ConnectionState().HandshakeComplete {
			b.Fatal("Handshake not completed!")
		}
	}
}

func newTLSConfigs() (client, server *tls.Config, err error) {
	certpool := x509.NewCertPool()
	serverCert, err := newTLSCert(certpool)
	if err != nil {
		return nil, nil, err
	}
	clientCert, err := newTLSCert(certpool)
	if err != nil {
		return nil, nil, err
	}
	server = &tls.Config{
		Certificates:           []tls.Certificate{serverCert},
		ClientCAs:              certpool,
		ClientAuth:             tls.RequireAndVerifyClientCert,
		SessionTicketsDisabled: true,
	}
	client = &tls.Config{
		Certificates:           []tls.Certificate{clientCert},
		RootCAs:                certpool,
		ServerName:             "127.0.0.1",
		SessionTicketsDisabled: true,
	}
	return client, server, nil
}

func newTLSCert(pool *x509.CertPool) (tls.Certificate, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		BasicConstraintsValid: true,
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, err
	}
	x509Cert, err := x509.ParseCertificate(certBytes)
	if err != nil {
		return tls.Certificate{}, err
	}
	pool.AddCert(x509Cert)
	return tls.Certificate{
		Certificate: [][]byte{certBytes},
		PrivateKey:  priv,
		Leaf:        x509Cert, // Set to avoid per-handshake calls to x509.ParseCertificate
	}, nil
}
