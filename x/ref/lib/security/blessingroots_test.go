// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"testing"

	"v.io/v23/security"
	"v.io/v23/verror"
)

func createAndMarshalPublicKey() ([]byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return security.NewECDSAPublicKey(&priv.PublicKey).MarshalBinary()
}

type rootsTester [4][]byte

func newRootsTester() *rootsTester {
	var tester rootsTester
	for idx := range tester {
		keybytes, err := createAndMarshalPublicKey()
		if err != nil {
			panic(err)
		}
		tester[idx] = keybytes
	}
	return &tester
}

func (t *rootsTester) add(br security.BlessingRoots) error {
	if err := br.Add(t[0], security.AllPrincipals); err == nil {
		return fmt.Errorf("Add( , %v) succeeded, expected it to fail", security.AllPrincipals)
	}
	testdata := []struct {
		root    []byte
		pattern security.BlessingPattern
	}{
		{t[0], "vanadium"},
		{t[1], "google:foo"},
		{t[2], "google:foo"},
		{t[0], "google:$"},
	}
	for _, d := range testdata {
		if err := br.Add(d.root, d.pattern); err != nil {
			return fmt.Errorf("Add(%v, %q) failed: %s", d.root, d.pattern, err)
		}
	}
	return nil
}

func (t *rootsTester) testRecognized(br security.BlessingRoots) error {
	testdata := []struct {
		root          []byte
		recognized    []string
		notRecognized []string
	}{
		{
			root:          t[0],
			recognized:    []string{"vanadium", "vanadium:foo", "vanadium:foo:bar", "google"},
			notRecognized: []string{"google:foo", "foo", "foo:bar"},
		},
		{
			root:          t[1],
			recognized:    []string{"google:foo", "google:foo:bar"},
			notRecognized: []string{"google", "google:bar", "vanadium", "vanadium:foo", "foo", "foo:bar"},
		},
		{
			root:          t[2],
			recognized:    []string{"google:foo", "google:foo:bar"},
			notRecognized: []string{"google", "google:bar", "vanadium", "vanadium:foo", "foo", "foo:bar"},
		},
		{
			root:          t[3],
			recognized:    []string{},
			notRecognized: []string{"vanadium", "vanadium:foo", "vanadium:bar", "google", "google:foo", "google:bar", "foo", "foo:bar"},
		},
	}
	for _, d := range testdata {
		for _, b := range d.recognized {
			if err := br.Recognized(d.root, b); err != nil {
				return fmt.Errorf("Recognized(%v, %q): got: %v, want nil", d.root, b, err)
			}
		}
		for _, b := range d.notRecognized {
			if err, want := br.Recognized(d.root, b), security.ErrUnrecognizedRoot; !errors.Is(err, want) {
				return fmt.Errorf("Recognized(%v, %q): got %v(errorid=%v), want errorid=%v", d.root, b, err, verror.ErrorID(err), want)
			}
		}
	}
	return nil
}

type pubKeySorter []security.PublicKey

func (s pubKeySorter) Len() int           { return len(s) }
func (s pubKeySorter) Less(i, j int) bool { return s[i].String() < s[j].String() }
func (s pubKeySorter) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (t *rootsTester) testDump(br security.BlessingRoots) error {
	object := func(data []byte) security.PublicKey {
		ret, err := security.UnmarshalPublicKey(data)
		if err != nil {
			panic(err)
		}
		return ret
	}
	want := map[security.BlessingPattern][]security.PublicKey{
		"google:foo": {object(t[1]), object(t[2])},
		"google:$":   {object(t[0])},
		"vanadium":   {object(t[0])},
	}
	got := br.Dump()
	sort.Sort(pubKeySorter(want["google:foo"]))
	sort.Sort(pubKeySorter(got["google:foo"]))
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("Dump(): got %v, want %v", got, want)
	}
	return nil
}

func TestBlessingRoots(t *testing.T) {
	p, err := NewPrincipal()
	if err != nil {
		t.Fatal(err)
	}
	tester := newRootsTester()
	if err := tester.add(p.Roots()); err != nil {
		t.Fatal(err)
	}
	if err := tester.testRecognized(p.Roots()); err != nil {
		t.Fatal(err)
	}
	if err := tester.testDump(p.Roots()); err != nil {
		t.Fatal(err)
	}
}

func TestBlessingRootsPersistence(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestBlessingRootsPersistence")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	tester := newRootsTester()
	p, err := CreatePersistentPrincipal(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tester.add(p.Roots()); err != nil {
		t.Error(err)
	}
	if err := tester.testRecognized(p.Roots()); err != nil {
		t.Error(err)
	}
	if err := tester.testDump(p.Roots()); err != nil {
		t.Error(err)
	}

	// Recreate the principal (and thus BlessingRoots)
	p2, err := LoadPersistentPrincipal(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := tester.testRecognized(p2.Roots()); err != nil {
		t.Error(err)
	}
	if err := tester.testDump(p2.Roots()); err != nil {
		t.Error(err)
	}
}
