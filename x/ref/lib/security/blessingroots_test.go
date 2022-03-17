// Copyright 2015 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"v.io/v23/security"
	"v.io/v23/verror"
	"v.io/x/ref/test/sectestdata"
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
			if err := br.RecognizedCert(&security.Certificate{PublicKey: d.root}, b); err != nil {
				return fmt.Errorf("Recognized(%v, %q): got: %v, want nil", d.root, b, err)
			}
		}
		for _, b := range d.notRecognized {
			if err, want := br.Recognized(d.root, b), security.ErrUnrecognizedRoot; !errors.Is(err, want) {
				return fmt.Errorf("Recognized(%v, %q): got %v(errorid=%v), want errorid=%v", d.root, b, err, verror.ErrorID(err), want)
			}
			if err, want := br.RecognizedCert(&security.Certificate{PublicKey: d.root}, b), security.ErrUnrecognizedRoot; !errors.Is(err, want) {
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
	dir := t.TempDir()
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

func matchError(err error, msgs ...string) bool {
	emsg := err.Error()
	for _, m := range msgs {
		if strings.Contains(emsg, m) {
			return true
		}
	}
	return false
}

func TestBlessingRootsX509(t *testing.T) {
	ctx := context.Background()

	for i, tc := range []struct {
		certType sectestdata.CertType
		pattern  string
	}{
		{sectestdata.SingleHostCert, "www.labdrive.io"},
		{sectestdata.SingleHostCert, "www.labdrive.io:a:b"},
		{sectestdata.MultipleHostsCert, "a.labdrive.io"},
		{sectestdata.MultipleHostsCert, "b.labdrive.io"},
		{sectestdata.MultipleHostsCert, "b.labdrive.io:a:b"},
		{sectestdata.WildcardCert, "foo.labdrive.io"},
		{sectestdata.WildcardCert, "bar.labdrive.io"},
		{sectestdata.WildcardCert, "bar.labdrive.io:x:y"},
		{sectestdata.MultipleWildcardCert, "foo.labdr.io"},
		{sectestdata.MultipleWildcardCert, "bar.labdrive.io"},
		{sectestdata.MultipleWildcardCert, "bar.labdr.io:x:y"},
	} {
		_, certs, opts := sectestdata.LetsEncryptData(tc.certType)
		x509Cert := certs[0]
		roots, err := NewBlessingRootsOpts(ctx, BlessingRootsX509VerifyOptions(opts))
		if err != nil {
			t.Fatal(err)
		}
		pkBytes, err := x509.MarshalPKIXPublicKey(x509Cert.PublicKey)
		if err != nil {
			t.Fatal(err)
		}
		err = roots.RecognizedCert(&security.Certificate{
			PublicKey: pkBytes,
			X509Raw:   x509Cert.Raw,
		}, tc.pattern)
		if err != nil {
			t.Errorf("%v: %v: %v: %v", i, tc.certType, tc.pattern, err)
		}

		roots, err = NewBlessingRootsOpts(ctx)
		if err != nil {
			t.Fatal(err)
		}
		err = roots.RecognizedCert(&security.Certificate{
			PublicKey: pkBytes,
			X509Raw:   x509Cert.Raw,
		}, tc.pattern)
		if err == nil || !matchError(err,
			"x509: certificate signed by unknown authority",
			"x509: certificate has expired or is not yet valid",
			"x509: “www.labdrive.io” certificate is not trusted",
			"x509: “(STAGING) Pretend Pear X1” certificate is not trusted") {
			t.Errorf("%v: %v: %v: missing or wrong error: %v", i, tc.certType, tc.pattern, err)
		}
	}

	for i, tc := range []struct {
		certType sectestdata.CertType
		pattern  string
	}{
		{sectestdata.SingleHostCert, "ww.labdrive.io"},
		{sectestdata.MultipleHostsCert, ".labdrive.io"},
		{sectestdata.MultipleHostsCert, "bdrive.io"},
		{sectestdata.WildcardCert, "labdrive.io"},
		{sectestdata.WildcardCert, "abdrive.io"},
		{sectestdata.MultipleWildcardCert, "labdr.io"},
		{sectestdata.MultipleWildcardCert, "abdrive.io"},
	} {
		_, certs, opts := sectestdata.LetsEncryptData(tc.certType)
		x509Cert := certs[0]
		roots, err := NewBlessingRootsOpts(ctx, BlessingRootsX509VerifyOptions(opts))
		if err != nil {
			t.Fatal(err)
		}
		pkBytes, err := x509.MarshalPKIXPublicKey(x509Cert.PublicKey)
		if err != nil {
			t.Fatal(err)
		}
		err = roots.RecognizedCert(&security.Certificate{
			PublicKey: pkBytes,
			X509Raw:   x509Cert.Raw,
		}, tc.pattern)
		if err == nil || !strings.Contains(err.Error(), "unrecognized public key") {
			t.Errorf("%v: %v: %v: missing or wrong error: %v", i, tc.certType, tc.pattern, err)
		}
	}
}
