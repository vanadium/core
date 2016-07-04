// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSignedCookie(t *testing.T) {
	currTime := time.Now()
	c := signedCookie{
		Payload: "flour, butter, eggs, chocolate",
		Expiry:  currTime.Add(time.Hour),
	}
	hmac := c.computeHMAC("chocolate_chip", "key west")
	if want, got := 32, len(hmac); want != got {
		t.Errorf("Expected %d bytes, got %d instead: %v", want, got, hmac)
	}
	c.HMAC = hmac
	if !c.verifyHMAC("chocolate_chip", "key west") {
		t.Errorf("HMAC verification failed for %v | %v", hmac, c)
	}
	// Trying a different sign key should not work.
	if c.verifyHMAC("chocolate_chip", "key east") {
		t.Errorf("HMAC verification should have failed for %v | %v", hmac, c)
	}
	// Fudging the cookie's stored HMAC value should fail validation.
	c.HMAC[0]++
	if c.verifyHMAC("chocolate_chip", "key west") {
		t.Errorf("HMAC verification should have failed for %v | %v", hmac, c)
	}
}

func TestPackUnpack(t *testing.T) {
	baker := &signedCookieBaker{
		secure:   true,
		signKey:  "key west",
		validity: time.Hour,
	}
	c, err := baker.packCookie("gingersnap", "flour, butter, ginger", "Rumpelstiltskin")
	if err != nil {
		t.Fatalf("packCookie failed: %v", err)
	}
	p, csrf, err := baker.unpackCookie("gingersnap", c)
	if err != nil {
		t.Fatalf("unpackCookie failed: %v", err)
	}
	if want, got := "flour, butter, ginger", p; want != got {
		t.Errorf("Expected payload %v, got %v instead", want, got)
	}
	if want, got := "Rumpelstiltskin", csrf; want != got {
		t.Errorf("Expected csrf token %v, got %v instead", want, got)
	}

	// Using bogus string as cookie should not work (and not crash).
	if _, _, err := baker.unpackCookie("gingersnap", "blah"); err == nil {
		t.Errorf("unpackCookie should have failed")
	}

	// Using wrong sign key should not work.
	baker.signKey = "key east"
	if _, _, err := baker.unpackCookie("gingersnap", c); err == nil {
		t.Errorf("unpackCookie should have failed")
	}

	// Expired cookie should not work.
	baker.validity = -time.Minute
	if c, err = baker.packCookie("gingersnap", "flour, butter, ginger", "Rumpelstiltskin"); err != nil {
		t.Fatalf("packCookie failed: %v", err)
	}
	if _, _, err := baker.unpackCookie("gingersnap", c); err == nil {
		t.Errorf("unpackCookie should have failed")
	}
}

type mockResponseWriter struct {
	buf        bytes.Buffer
	header     http.Header
	statusCode int
}

func (w *mockResponseWriter) Header() http.Header {
	return w.header
}

func (w *mockResponseWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

func (w *mockResponseWriter) WriteHeader(c int) {
	w.statusCode = c
}

func TestBaker(t *testing.T) {
	var baker cookieBaker
	baker = &signedCookieBaker{
		secure:   true,
		signKey:  "key west",
		validity: time.Minute,
	}
	w := &mockResponseWriter{
		header: make(http.Header),
	}
	if err := baker.set(w, "butter_pecan", "flour, butter, pecans", "Rumpelstiltskin"); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	c := w.Header().Get("Set-Cookie")
	parts := strings.SplitN(c, "=", 2)
	if len(parts) != 2 {
		t.Fatalf("malformed cookie %v", c)
	}
	req, err := http.NewRequest("GET", "http://example.com", nil)
	if err != nil {
		t.Fatalf("Failed to create request")
	}
	req.AddCookie(&http.Cookie{Name: "butter_pecan", Value: parts[1][:strings.Index(parts[1], ";")]})

	if p, csrf, err := baker.get(req, "butter_pecan"); err != nil {
		t.Fatalf("get failed: %v", err)
	} else {
		if want, got := "flour, butter, pecans", p; want != got {
			t.Errorf("Expected payload %v, got %v instead", want, got)
		}
		if want, got := "Rumpelstiltskin", csrf; want != got {
			t.Errorf("Expected csrf token %v, got %v instead", want, got)
		}
	}

	if p, csrf, err := baker.get(req, "chocolate_mint"); p != "" || csrf != "" || err != nil {
		t.Errorf("get should have returned empty payload and csrf token for non-existant cookie")
	}
}
