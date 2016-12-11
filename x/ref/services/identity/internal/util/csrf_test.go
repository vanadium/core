// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "v.io/x/ref/runtime/factories/generic"
	"v.io/x/ref/test"
)

const (
	cookieName     = "VeyronCSRFTestCookie"
	failCookieName = "FailCookieName"
)

func TestCSRFTokenWithoutCookie(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()

	r := newRequest()
	c := NewCSRFCop(ctx)
	w := httptest.NewRecorder()
	tok, err := c.NewToken(w, r, cookieName, nil)
	if err != nil {
		t.Errorf("NewToken failed: %v", err)
	}
	cookie, err := cookieVal(w, cookieName)
	if err != nil {
		t.Error(err)
	}
	if len(cookie) == 0 {
		t.Errorf("Cookie should have been set. Request: [%v], Response: [%v]", r, w)
	}
	// Cookie needs to be present for validation
	r.AddCookie(&http.Cookie{Name: cookieName, Value: cookie})
	if err := c.ValidateToken(tok, r, cookieName, nil); err != nil {
		t.Error("CSRF token failed validation:", err)
	}

	w = httptest.NewRecorder()
	if _, err = c.MaybeSetCookie(w, r, failCookieName); err != nil {
		t.Error("failed to create cookie: ", err)
	}
	cookie, err = cookieVal(w, failCookieName)
	if err != nil {
		t.Error(err)
	}
	if len(cookie) == 0 {
		t.Errorf("Cookie should have been set. Request: [%v], Response: [%v]", r, w)
	}

	if err := c.ValidateToken(tok, r, failCookieName, nil); err == nil {
		t.Error("CSRF token should have failed validation")
	}
}

func TestCSRFTokenWithCookie(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	r := newRequest()
	c := NewCSRFCop(ctx)
	w := httptest.NewRecorder()
	r.AddCookie(&http.Cookie{Name: cookieName, Value: "u776AC7hf794pTtGVlO50w=="})
	tok, err := c.NewToken(w, r, cookieName, nil)
	if err != nil {
		t.Errorf("NewToken failed: %v", err)
	}
	cookie, err := cookieVal(w, cookieName)
	if err != nil {
		t.Error(err)
	}
	if len(cookie) > 0 {
		t.Errorf("Cookie should not be set when it is already present. Request: [%v], Response: [%v]", r, w)
	}
	if err := c.ValidateToken(tok, r, cookieName, nil); err != nil {
		t.Error("CSRF token failed validation:", err)
	}

	r.AddCookie(&http.Cookie{Name: failCookieName, Value: "u864AC7gf794pTtCAlO40w=="})
	if err := c.ValidateToken(tok, r, failCookieName, nil); err == nil {
		t.Error("CSRF token should have failed validation")
	}
}

func TestCSRFTokenWithData(t *testing.T) {
	ctx, shutdown := test.V23Init()
	defer shutdown()
	r := newRequest()
	c := NewCSRFCop(ctx)
	w := httptest.NewRecorder()
	r.AddCookie(&http.Cookie{Name: cookieName, Value: "u776AC7hf794pTtGVlO50w=="})
	tok, err := c.NewToken(w, r, cookieName, 1)
	if err != nil {
		t.Errorf("NewToken failed: %v", err)
	}
	cookie, err := cookieVal(w, cookieName)
	if err != nil {
		t.Error(err)
	}
	if len(cookie) > 0 {
		t.Errorf("Cookie should not be set when it is already present. Request: [%v], Response: [%v]", r, w)
	}
	var got int
	if err := c.ValidateToken(tok, r, cookieName, &got); err != nil {
		t.Error("CSRF token failed validation:", err)
	}
	if want := 1; got != want {
		t.Errorf("Got %v, want %v", got, want)
	}

	r.AddCookie(&http.Cookie{Name: failCookieName, Value: "u864AC7gf794pTtCAlO40w=="})
	if err := c.ValidateToken(tok, r, failCookieName, &got); err == nil {
		t.Error("CSRF token should have failed validation")
	}
}

func cookieVal(w *httptest.ResponseRecorder, cookieName string) (string, error) {
	cookie := w.Header().Get("Set-Cookie")
	if len(cookie) == 0 {
		return "", nil
	}
	var (
		val              string
		httpOnly, secure bool
	)
	for _, part := range strings.Split(cookie, "; ") {
		switch {
		case strings.HasPrefix(part, cookieName):
			val = strings.TrimPrefix(part, cookieName+"=")
		case part == "HttpOnly":
			httpOnly = true
		case part == "Secure":
			secure = true
		}
	}
	if !httpOnly {
		return "", fmt.Errorf("cookie for name %v is not HttpOnly", cookieName)
	}
	if !secure {
		return "", fmt.Errorf("cookie for name %v is not Secure", cookieName)
	}
	return val, nil
}
