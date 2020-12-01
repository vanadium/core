// Copyright 2020 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package verror

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestLCS(t *testing.T) {
	for i, tc := range []struct {
		pkg, path       string
		lcsPkg, lcsPath string
	}{
		{"a", "b", "", ""},
		{"/a/b/c", "/a/b/c", "/a/b/c", "/a/b/c"},
		{"/a/b/c/d", "/x/y/c/d", "c/d", "c/d"},
	} {
		lcsPkg, lcsPath := longestCommonSuffix(tc.pkg, tc.path)
		if got, want := lcsPkg, tc.lcsPkg; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
		if got, want := lcsPath, tc.lcsPath; got != want {
			t.Errorf("%v: got %v, want %v", i, got, want)
		}
	}
}

func TestPkgPath(t *testing.T) {
	type dummy int
	ps := &pathState{}
	_, file, _, _ := runtime.Caller(0)
	ps.init(reflect.TypeOf(dummy(0)).PkgPath(), file)
	if got, want := ps.pkg, "v.io/v23/verror"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := ps.dir, filepath.Dir(file); got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := strings.HasPrefix(file, ps.filePrefix), true; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := ps.pkgPrefix, "v.io"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
	ps.init("github.com/grailbio/base", "/a/b/src/base/file")
	if got, want := ps.pkgPrefix, "github.com/grailbio"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
