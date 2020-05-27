// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	v23 "v.io/v23"
	"v.io/v23/context"
	"v.io/v23/vom"
)

const (
	cookieLen = 16
)

// CSRFCop implements utilities for generating and validating tokens for
// cross-site-request-forgery prevention (also called XSRF).
type CSRFCop struct {
	ctx *context.T
}

func NewCSRFCop(ctx *context.T) *CSRFCop {
	return &CSRFCop{ctx: ctx}
}

// NewToken creates an anti-cross-site-request-forgery, aka CSRF aka XSRF token
// with some data bound to it that can be obtained by ValidateToken.
// It returns an error if the token could not be created.
func (c *CSRFCop) NewToken(w http.ResponseWriter, r *http.Request, cookieName string, data interface{}) (string, error) {
	cookieValue, err := c.MaybeSetCookie(w, r, cookieName)
	if err != nil {
		return "", fmt.Errorf("bad cookie: %v", err)
	}
	var encData []byte
	if data != nil {
		if encData, err = vom.Encode(data); err != nil {
			return "", err
		}
	}
	hash := sha256.Sum256(cookieValue)
	mac, err := NewMacaroon(v23.GetPrincipal(c.ctx), append(hash[:], encData...))
	return string(mac), err
}

// ValidateToken checks the validity of the provided CSRF token for the
// provided request, and extracts the data encoded in the token into 'decoded'.
// If the token is invalid, return an error. This error should not be shown to end users,
// it is meant for the consumption by the server process only.
func (c *CSRFCop) ValidateToken(token string, req *http.Request, cookieName string, decoded interface{}) error {
	cookie, err := req.Cookie(cookieName)
	if err != nil {
		return err
	}
	cookieValue, err := decodeCookieValue(cookie.Value)
	if err != nil {
		return fmt.Errorf("invalid cookie")
	}
	encodedInput, err := Macaroon(token).Decode(v23.GetPrincipal(c.ctx))
	if err != nil {
		return err
	}
	if len(encodedInput) < sha256.Size {
		return fmt.Errorf("invalid token data: too short")
	}
	hash := sha256.Sum256(cookieValue)
	if !bytes.Equal(hash[:], encodedInput[:sha256.Size]) {
		return fmt.Errorf("invalid token data")
	}
	if decoded != nil {
		if err := vom.Decode(encodedInput[sha256.Size:], decoded); err != nil {
			return fmt.Errorf("invalid token data: %v", err)
		}
	}
	return nil
}

func (c *CSRFCop) MaybeSetCookie(w http.ResponseWriter, req *http.Request, cookieName string) ([]byte, error) {
	cookie, err := req.Cookie(cookieName)
	switch err {
	case nil:
		if v, err := decodeCookieValue(cookie.Value); err == nil {
			return v, nil
		}
		c.ctx.Infof("Invalid cookie: %#v, err: %v. Regenerating one.", cookie, err)
	case http.ErrNoCookie:
		// Intentionally blank: Cookie will be generated below.
	default:
		c.ctx.Infof("Error decoding cookie %q in request: %v. Regenerating one.", cookieName, err)
	}
	cookie, v := newCookie(c.ctx, cookieName)
	if cookie == nil || v == nil {
		return nil, fmt.Errorf("failed to create cookie")
	}
	http.SetCookie(w, cookie)
	// We need to add the cookie to the request also to prevent repeatedly resetting cookies on multiple
	// calls from the same request.
	req.AddCookie(cookie)
	return v, nil
}

func newCookie(ctx *context.T, cookieName string) (*http.Cookie, []byte) {
	b := make([]byte, cookieLen)
	if _, err := rand.Read(b); err != nil {
		ctx.Errorf("newCookie failed: %v", err)
		return nil, nil
	}
	return &http.Cookie{
		Name:     cookieName,
		Value:    b64encode(b),
		Expires:  time.Now().Add(time.Hour * 24),
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
	}, b
}

func decodeCookieValue(v string) ([]byte, error) {
	b, err := b64decode(v)
	if err != nil {
		return nil, err
	}
	if len(b) != cookieLen {
		return nil, fmt.Errorf("invalid cookie length[%d]", len(b))
	}
	return b, nil
}

// Shorthands.
func b64encode(b []byte) string          { return base64.URLEncoding.EncodeToString(b) }
func b64decode(s string) ([]byte, error) { return base64.URLEncoding.DecodeString(s) }
