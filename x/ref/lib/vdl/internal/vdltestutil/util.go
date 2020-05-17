// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package vdltestutil provides testing utilities for v.io/x/ref/lib/vdl/...
package vdltestutil

import (
	"io"
	"io/ioutil"
	"regexp"
	"strings"
	"testing"

	"v.io/x/ref/lib/vdl/build"
	"v.io/x/ref/lib/vdl/vdlutil"
)

// ExpectPass makes sure errs has no errors.
func ExpectPass(t *testing.T, errs *vdlutil.Errors, testName string) {
	if !errs.IsEmpty() {
		t.Errorf("%v expected no errors but saw: %v", testName, errs.ToError())
		errs.Reset()
	}
}

// ExpectFail makes sure errs has an error that matches all the re regexps.
func ExpectFail(t *testing.T, errs *vdlutil.Errors, testName string, re ...string) {
	if errs.IsEmpty() {
		t.Errorf("%v expected errors but didn't see any", testName)
		return
	}
	actual := errs.ToError().Error()
	errs.Reset()
	for index, errRe := range re {
		matched, err := regexp.Match(errRe, []byte(actual))
		if err != nil {
			t.Errorf("%v bad regexp pattern [%v] %q", testName, index, errRe)
			return
		}
		if !matched {
			t.Errorf("%v couldn't match pattern [%v] %q against %q", testName, index, errRe, actual)
		}
	}
}

// ExpectResult ensures errs has an error that matches all the re regexps, or
// that errs has no errors if no regexps were provided, or only one was provided
// with the empty string.
func ExpectResult(t *testing.T, errs *vdlutil.Errors, testName string, re ...string) {
	if len(re) == 0 || len(re) == 1 && re[0] == "" {
		ExpectPass(t, errs, testName)
	} else {
		ExpectFail(t, errs, testName, re...)
	}
}

// FakeBuildPackage constructs a fake build package for testing, with files
// mapping from file names to file contents.
func FakeBuildPackage(name, path string, files map[string]string) *build.Package {
	var fnames []string
	for fname := range files {
		fnames = append(fnames, fname)
	}
	return &build.Package{
		Dir:           "",
		Name:          name,
		Path:          path,
		BaseFileNames: fnames,
		OpenFilesFunc: FakeOpenFiles(files),
	}
}

// FakeOpenFiles returns a function that obeys the build.Package.OpenFilesFunc
// signature, that simply uses the files map to return readers.
func FakeOpenFiles(files map[string]string) func(fnames []string) (map[string]io.ReadCloser, error) {
	return func(fnames []string) (map[string]io.ReadCloser, error) {
		ret := make(map[string]io.ReadCloser, len(fnames))
		for _, fname := range fnames {
			ret[fname] = ioutil.NopCloser(strings.NewReader(files[fname]))
		}
		return ret, nil
	}
}
