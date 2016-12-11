// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package util

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestBadRequest(t *testing.T) {
	w := httptest.NewRecorder()
	HTTPBadRequest(w, newRequest(), nil)
	if got, want := w.Code, http.StatusBadRequest; got != want {
		t.Errorf("Got %d, want %d", got, want)
	}
}

func TestServerError(t *testing.T) {
	w := httptest.NewRecorder()
	HTTPServerError(w, nil)
	if got, want := w.Code, http.StatusInternalServerError; got != want {
		t.Errorf("Got %d, want %d", got, want)
	}
}
