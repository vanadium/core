// Copyright 2022 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	gocontext "context"
	"crypto"
	"crypto/x509"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/security"
	seclib "v.io/x/ref/lib/security"
	"v.io/x/ref/lib/security/keys"
	"v.io/x/ref/test/sectestdata"
)

func newPrincipalWithX509Opts(ctx gocontext.Context, t testing.TB, key crypto.PrivateKey, opts x509.VerifyOptions) security.Principal {
	signer, err := seclib.NewSignerFromKey(ctx, key)
	if err != nil {
		t.Fatal(err)
	}

	roots, err := seclib.NewBlessingRootsOpts(ctx, seclib.BlessingRootsX509VerifyOptions(opts))
	if err != nil {
		t.Fatal(err)
	}

	p, err := seclib.CreatePrincipalOpts(ctx,
		seclib.WithSigner(signer),
		seclib.WithBlessingRoots(roots))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func newX509ServerPrincipal(ctx gocontext.Context, t testing.TB, key crypto.PrivateKey, host string, certs []*x509.Certificate, opts x509.VerifyOptions) security.Principal {
	privKeyBytes, err := seclib.MarshalPrivateKey(key, nil)
	if err != nil {
		t.Fatal(err)
	}
	pubKeyBytes, err := seclib.MarshalPublicKey(certs[0])
	if err != nil {
		t.Fatal(err)
	}

	roots, err := seclib.NewBlessingRootsOpts(ctx, seclib.BlessingRootsX509VerifyOptions(opts))
	if err != nil {
		t.Fatal(err)
	}

	p, err := seclib.CreatePrincipalOpts(ctx,
		seclib.WithPrivateKeyBytes(ctx, pubKeyBytes, privKeyBytes, nil),
		seclib.WithBlessingRoots(roots))
	if err != nil {
		t.Fatal(err)
	}
	cert := security.ExposeX509Certificate(p)
	if cert == nil {
		t.Fatalf("no x509 certificate found in principal")
	} else {
		if got, want := p.PublicKey().String(), security.ExposeFingerPrint(cert.PublicKey); got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	}

	blessings, err := p.BlessSelf(host)
	if err != nil {
		t.Fatalf("BlessSelf: %v", err)
	}
	if err := p.BlessingStore().SetDefault(blessings); err != nil {
		t.Fatalf("failed to set defaut blessings: %v", err)
	}
	return p
}

func appendPatterns(hosts []string, patterns ...string) []string {
	bp := []string{}
	for _, h := range hosts {
		n := h
		bp = append(bp, n)
		for _, p := range patterns {
			if len(n) > 0 {
				bp = append(bp, n+security.ChainSeparator+p)
				continue
			}
			bp = append(bp, p)
		}
	}
	return bp
}

func TestX509(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()

	s := func(a ...string) []string {
		return a
	}

	for _, tc := range []struct {
		certType   sectestdata.CertType
		host       string
		hosts      []string
		validHosts []string
	}{
		{sectestdata.SingleHostCert, "", s("www.labdrive.io"), nil},
		{sectestdata.SingleHostCert, "www.labdrive.io", s("www.labdrive.io"), nil},
		{sectestdata.SingleHostCert, "www.labdrive.io:friend:alice", s("www.labdrive.io:friend:alice"), nil},
		{sectestdata.MultipleHostsCert, "c.labdrive.io", s("c.labdrive.io"), nil},
		{sectestdata.MultipleHostsCert, "", s("a.labdrive.io", "b.labdrive.io", "c.labdrive.io"), nil},
		{sectestdata.MultipleHostsCert, "b.labdrive.io", s("b.labdrive.io"), nil},
		{sectestdata.WildcardCert, "", s("*.labdrive.io"), s("foo.labdrive.io", "bar.labdrive.io")},
		{sectestdata.WildcardCert, "foo.labdrive.io", s("foo.labdrive.io"), s("foo.labdrive.io")},
		{sectestdata.WildcardCert, "foo.labdrive.io:friend", s("foo.labdrive.io:friend"), s("foo.labdrive.io:friend")},
		{sectestdata.MultipleWildcardCert, "foo.labdr.io", s("foo.labdr.io"), s("foo.labdr.io")},
		{sectestdata.MultipleWildcardCert, "foo.labdr.io", s("foo.labdr.io"), s("foo.labdr.io")},
		{sectestdata.MultipleWildcardCert, "bar.labdrive.io", s("bar.labdrive.io"), s("bar.labdrive.io")},
	} {
		privKey, pubCerts, opts := sectestdata.LetsEncryptData(tc.certType)
		server := newX509ServerPrincipal(ctx, t, privKey, tc.host, pubCerts, opts)
		blessings, _ := server.BlessingStore().Default()
		verifyBlessingSignatures(t, blessings)
		names := security.BlessingNames(server, blessings)

		if got, want := names, tc.hosts; !reflect.DeepEqual(got, want) {
			t.Errorf("%v: got %v, want %v", tc.certType, got, want)
		}
		if got, want := blessings.Expiry(), pubCerts[0].NotAfter; got != want {
			t.Errorf("%v: got %v, want %v", tc.certType, got, want)
		}

		clientKey := sectestdata.V23PrivateKey(keys.ED25519, sectestdata.V23KeySetB)
		client := newPrincipalWithX509Opts(ctx, t, clientKey, opts)
		call := security.NewCall(&security.CallParams{
			LocalPrincipal:  client,
			RemoteBlessings: blessings,
			Timestamp:       pubCerts[0].NotBefore.Add(48 * time.Hour),
		})
		names, rejected := security.RemoteBlessingNames(ctx, call)
		if len(rejected) != 0 {
			t.Errorf("%v: rejected blessings: %v", tc.certType, rejected)
		}
		if got, want := names, tc.hosts; !reflect.DeepEqual(got, want) {
			t.Errorf("%v: got %v, want %v", tc.certType, got, want)
		}
		validHosts := tc.validHosts
		if validHosts == nil {
			validHosts = tc.hosts
		}

		merged := blessings
		chained := validHosts
		if len(tc.host) > 0 {
			friend, err := server.BlessSelf(tc.host + security.ChainSeparator + "friend")
			if err != nil {
				t.Fatal(err)
			}
			merged, err = security.UnionOfBlessings(blessings, friend)
			if err != nil {
				t.Fatal(err)
			}
			chained = appendPatterns(validHosts, "friend")
		}
		if !merged.CouldHaveNames(chained) {
			t.Errorf("%v: CouldHaveNames is false for: %q: %v", tc.certType, tc.host, chained)
		}
	}
}

func validateRejected(t *testing.T, names []string, rejected []security.RejectedBlessing, msgs []string) {
	_, _, line, _ := runtime.Caller(2)
	if len(names) != 0 {
		t.Errorf("line: %v: unexpected blessing names: %v", line, names)
	}
	if len(rejected) == 0 {
		t.Errorf("line: %v: no blessings were rejected", line)
	}
	for _, msg := range msgs {
		if strings.Contains(rejected[0].Err.Error(), msg) {
			return
		}
	}
	t.Errorf("line: %v: blessings error message %v is not one of: %v", line, rejected[0].Err.Error(), msgs)
}

func TestX509CouldHaveNames(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()

	s := func(a ...string) []string {
		return a
	}

	for _, tc := range []struct {
		certType sectestdata.CertType
		hosts    []string
	}{
		{sectestdata.SingleHostCert, s("www.labdrive.io")},
		{sectestdata.MultipleHostsCert, s("a.labdrive.io", "b.labdrive.io")},
		{sectestdata.WildcardCert, s("foo.labdrive.io", "bar.labdrive.io")},
		{sectestdata.MultipleWildcardCert, s("foo.labdrive.io", "bar.labdr.io")},
	} {
		privKey, pubCerts, opts := sectestdata.LetsEncryptData(tc.certType)
		server := newX509ServerPrincipal(ctx, t, privKey, "", pubCerts, opts)
		blessings, _ := server.BlessingStore().Default()

		merged := blessings
		for _, host := range tc.hosts {
			friend, err := server.BlessSelf(host + security.ChainSeparator + "friend")
			if err != nil {
				t.Fatal(err)
			}
			merged, err = security.UnionOfBlessings(merged, friend)
			if err != nil {
				t.Fatal(err)
			}
		}
		chained := appendPatterns(tc.hosts, "friend")

		if !merged.CouldHaveNames(chained) {
			t.Errorf("%v: CouldHaveNames is false for: %q: %v", tc.certType, tc.hosts, chained)
		}

		shouldFail := appendPatterns([]string{"x.y.z"}, "friend")
		if merged.CouldHaveNames(shouldFail) {
			t.Errorf("%v: CouldHaveNames is true for: %q: %v", tc.certType, tc.hosts, chained)
		}

		shouldFail = appendPatterns(append([]string{}, tc.hosts[0], "x.y.z"), "friend")
		if merged.CouldHaveNames(shouldFail) {
			t.Errorf("%v: CouldHaveNames is true for: %q: %v", tc.certType, tc.hosts, chained)
		}

	}
}

func TestX509Errors(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()

	var names []string
	var rejected []security.RejectedBlessing
	validate := func(msgs ...string) {
		validateRejected(t, names, rejected, msgs)
	}

	s := func(a ...string) []string {
		return a
	}

	for _, tc := range []struct {
		certType     sectestdata.CertType
		host         string
		hosts        []string
		invalidHosts []string
	}{
		{sectestdata.SingleHostCert, "www.labdrive.io", s("www.labdrive.io"), s("foo:bar")},
		{sectestdata.SingleHostCert, "", s("www.labdrive.io"), s("x.labdrive.io")},
		{sectestdata.SingleHostCert, "", s("www.labdrive.io"), s("x.labdr.io")},
		{sectestdata.MultipleHostsCert, "", s("a.labdrive.io", "b.labdrive.io", "c.labdrive.io"), s("x.labdrive.io")},
		{sectestdata.MultipleHostsCert, "b.labdrive.io", s("b.labdrive.io"), s("a.labdrive.io")},
		{sectestdata.WildcardCert, "", s("*.labdrive.io"), s("foo.bar.labdrive.io", ".labdrive.io")},
		{sectestdata.WildcardCert, "foo.labdrive.io", s("foo.labdrive.io"), s("bar.labdrive.io", ".labdrive.io")},
		{sectestdata.MultipleWildcardCert, "", s("*.labdrive.io"), s("foo.bar.labdrive.io", ".labdrive.io")},
		{sectestdata.MultipleWildcardCert, "", s("*.labdr.io"), s("foo.bar.labdr.io", ".labdr.io")},
		{sectestdata.MultipleWildcardCert, "bar.labdr.io", s("bar.labdr.io"), s("bar.labdrive.io")},
	} {
		privKey, pubCerts, opts := sectestdata.LetsEncryptData(tc.certType)
		server := newX509ServerPrincipal(ctx, t, privKey, tc.host, pubCerts, opts)
		blessings, _ := server.BlessingStore().Default()

		clientKey := sectestdata.V23PrivateKey(keys.ED25519, sectestdata.V23KeySetB)
		client := newPrincipalWithX509Opts(ctx, t, clientKey, opts)

		// After expiration, ie. 10000 days into the future
		call := security.NewCall(&security.CallParams{
			LocalPrincipal:  client,
			RemoteBlessings: blessings,
			Timestamp:       time.Now().Add(24 * 10000 * time.Hour),
		})
		names, rejected = security.RemoteBlessingNames(ctx, call)
		validate("is after expiry")

		// Before coming into effect.
		call = security.NewCall(&security.CallParams{
			LocalPrincipal:  client,
			RemoteBlessings: blessings,
			Timestamp:       pubCerts[0].NotBefore.Add(-48 * time.Hour),
		})
		names, rejected = security.RemoteBlessingNames(ctx, call)
		validate("is not before")

		// Without a custom cert pool the validation should fail with a
		// complaint about being signed by an unknown authority.
		client = newPrincipalWithX509Opts(ctx, t, clientKey, x509.VerifyOptions{
			CurrentTime: pubCerts[0].NotBefore.Add(48 * time.Hour),
		})
		call = security.NewCall(&security.CallParams{
			LocalPrincipal:  client,
			RemoteBlessings: blessings,
			Timestamp:       pubCerts[0].NotBefore.Add(48 * time.Hour),
		})

		names, rejected = security.RemoteBlessingNames(ctx, call)
		validate("x509: certificate signed by unknown authority",
			"certificate is not trusted")

		// No custom options.
		client = newPrincipalWithX509Opts(ctx, t, clientKey, x509.VerifyOptions{})
		call = security.NewCall(&security.CallParams{
			LocalPrincipal:  client,
			RemoteBlessings: blessings,
			Timestamp:       pubCerts[0].NotBefore.Add(48 * time.Hour),
		})

		names, rejected = security.RemoteBlessingNames(ctx, call)
		validate("x509: certificate has expired or is not yet valid",
			"x509: certificate signed by unknown authority",
			`certificate is not trusted`,
			`x509: “www.labdrive.io” certificate is not trusted`,
		)

		if !blessings.CouldHaveNames(tc.hosts) {
			t.Errorf("%v: CouldHaveNames is false for: %v", tc.certType, tc.hosts)
		}

		if blessings.CouldHaveNames(tc.invalidHosts) {
			t.Errorf("%v: CouldHaveNames is true for: %v", tc.certType, tc.invalidHosts)
		}

		merged := blessings
		chained := tc.hosts
		chainedInvalidHosts := tc.invalidHosts

		if len(tc.host) > 0 {
			friend, err := server.BlessSelf(tc.host + security.ChainSeparator + "friend")
			if err != nil {
				t.Fatal(err)
			}
			merged, err = security.UnionOfBlessings(blessings, friend)
			if err != nil {
				t.Fatal(err)
			}
			chained = appendPatterns(tc.hosts, "friend")
			chainedInvalidHosts = appendPatterns(tc.invalidHosts, "friend")

		}
		if !merged.CouldHaveNames(chained) {
			t.Errorf("%v: CouldHaveNames is false for: %v", tc.certType, chained)
		}

		if merged.CouldHaveNames(chainedInvalidHosts) {
			t.Errorf("%v: CouldHaveNames is true for: %v", tc.certType, chainedInvalidHosts)
		}
	}
}

func TestX509ServerErrors(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	for _, tc := range []struct {
		certType    sectestdata.CertType
		invalidHost string
	}{
		{sectestdata.SingleHostCert, "x.labdrive.io"},
		{sectestdata.MultipleHostsCert, "x.labdrive.io"},
		{sectestdata.WildcardCert, "bar.labdr.io"},
		{sectestdata.MultipleWildcardCert, "bar.labdrx.io"},
	} {
		privKey, pubCerts, opts := sectestdata.LetsEncryptData(tc.certType)
		server := newX509ServerPrincipal(ctx, t, privKey, "", pubCerts, x509.VerifyOptions{})
		blessings, _ := server.BlessingStore().Default()
		verifyBlessingSignatures(t, blessings)

		// No names will returned since none of the x509 certs are valid without
		// the custom x509.VerifyOptions being supplied to NewX509ServerPrincipal.
		names := security.BlessingNames(server, blessings)
		if len(names) > 0 {
			t.Errorf("no names should be returned: %v\n", names)
		}

		dir := t.TempDir()
		store, err := seclib.CreateFilesystemStore(dir)
		if err != nil {
			t.Fatal(err)
		}

		signer, err := seclib.NewSignerFromKey(ctx, privKey)
		if err != nil {
			t.Fatal(err)
		}

		roots, err := seclib.NewBlessingRootsOpts(ctx,
			seclib.BlessingRootsX509VerifyOptions(opts),
			seclib.BlessingRootsWriteable(seclib.FilesystemStoreWriter(dir), signer))
		if err != nil {
			t.Fatal(err)
		}

		// The following will result in an error from BlessSelf since
		// the requested tc.invalidHost is not supported by the certificate.
		server, err = seclib.CreatePrincipalOpts(ctx,
			seclib.WithPrivateKey(privKey, nil),
			seclib.WithStore(store),
			seclib.WithX509Certificate(pubCerts[0]),
			seclib.WithBlessingRoots(roots))
		if err != nil {
			t.Fatalf("failed to create principal: %v", err)
		}

		_, err = server.BlessSelf(tc.invalidHost)

		want := fmt.Sprintf(", not %v", tc.invalidHost)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("unexpected or missing error: %q does not contain %q", err, want)
		}

		server, err = seclib.LoadPrincipalOpts(ctx,
			seclib.FromReadonly(seclib.FilesystemStoreReader(dir)),
		)

		if err != nil {
			t.Fatalf("failed to create principal: %v", err)
		}

		_, err = server.BlessSelf(tc.invalidHost)

		want = fmt.Sprintf(", not %v", tc.invalidHost)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("unexpected or missing error: %q does not contain %q", err, want)
		}
	}
}
