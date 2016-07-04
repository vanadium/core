// Copyright 2016 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type cookieBaker interface {
	// set creates the cookie with the given name and payload, and also
	// stores the given csrf token in the cookie.
	set(w http.ResponseWriter, name, payload, csrfToken string) error
	// get retrieves the payload and csrf token from the cookie.  If the
	// cookie does not exist, get returns ("", "", nil).
	get(req *http.Request, name string) (string, string, error)
}

type signedCookieBaker struct {
	secure   bool
	signKey  string
	validity time.Duration
}

type signedCookie struct {
	// Payload is the value the cookie is meant to keep.
	Payload string `json:"payload"`
	// Expiry ensures cookies cannot be used beyond their intended validity.
	Expiry time.Time `json:"expiry"`
	// HMAC signs all the above fields and prevents tampering with the
	// cookie.
	HMAC []byte `json:"hmac"`
	// CSRFToken prevents CSRF attacks by ensuring that requests must
	// contain a csrf token that matches the cookie.
	CSRFToken string `'json:"csrfToken"`
}

func computeHMAC(signKey string, fields ...string) []byte {
	mac := hmac.New(sha256.New, []byte(signKey))
	for _, f := range fields {
		fmt.Fprintf(mac, "%08x%s", len(f), f)
	}
	return mac.Sum(nil)
}

func (c *signedCookie) computeHMAC(name, signKey string) []byte {
	return computeHMAC(signKey, name, c.Payload, c.Expiry.String(), c.CSRFToken)
}

func (c *signedCookie) verifyHMAC(name, signKey string) bool {
	return hmac.Equal(c.HMAC, c.computeHMAC(name, signKey))
}

func (b *signedCookieBaker) packCookie(name, payload, csrfToken string) (string, error) {
	c := signedCookie{
		Payload:   payload,
		Expiry:    time.Now().Add(b.validity),
		CSRFToken: csrfToken,
	}
	c.HMAC = c.computeHMAC(name, b.signKey)
	jsonData, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(jsonData), nil
}

func (b *signedCookieBaker) unpackCookie(name, value string) (string, string, error) {
	jsonData, err := base64.URLEncoding.DecodeString(value)
	if err != nil {
		return "", "", err
	}
	var c signedCookie
	if err := json.Unmarshal(jsonData, &c); err != nil {
		return "", "", err
	}
	if time.Now().After(c.Expiry) {
		return "", "", fmt.Errorf("cookie expired on %v", c.Expiry)
	}
	if !c.verifyHMAC(name, b.signKey) {
		return "", "", fmt.Errorf("HMAC mismatching for cookie %v", c)
	}
	return c.Payload, c.CSRFToken, nil
}

func (b *signedCookieBaker) set(w http.ResponseWriter, name, payload, csrfToken string) error {
	cookieValue, err := b.packCookie(name, payload, csrfToken)
	if err != nil {
		return err
	}
	cookie := http.Cookie{
		Name:     name,
		Value:    cookieValue,
		Expires:  time.Now().Add(b.validity),
		HttpOnly: true,
		Secure:   b.secure,
		Path:     routeRoot,
	}
	http.SetCookie(w, &cookie)
	return nil
}

func (b *signedCookieBaker) get(req *http.Request, name string) (string, string, error) {
	cookie, err := req.Cookie(name)
	if err == http.ErrNoCookie {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	return b.unpackCookie(name, cookie.Value)
}
