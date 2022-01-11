// Copyright 2021 The Vanadium Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package security_test

import (
	"crypto/x509"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"v.io/v23/context"
	"v.io/v23/internal/sectest"
	"v.io/v23/security"
	"v.io/x/ref/test/sectestdata"
)

func TestX509(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()

	s := func(a ...string) []string {
		return a
	}

	for _, tc := range []struct {
		certType   sectestdata.CertType
		hosts      []string
		validHosts []string
	}{
		{sectestdata.SingleHostCert, s("www.labdrive.io"), nil},
		{sectestdata.MultipleHostsCert, s("a.labdrive.io", "b.labdrive.io", "c.labdrive.io"), nil},
		{sectestdata.WildcardCert, s("*.labdrive.io"), s("foo.labdrive.io", "bar.labdrive.io")},
	} {
		privKey, pubCerts, opts := sectestdata.LetsEncryptData(tc.certType)
		server := sectest.NewX509ServerPrincipal(t, privKey, pubCerts, &opts)
		blessings, _ := server.BlessingStore().Default()
		verifyBlessingSignatures(t, blessings)
		names := security.BlessingNames(server, blessings)
		if got, want := names, tc.hosts; !reflect.DeepEqual(got, want) {
			t.Errorf("%v: got %v, want %v", tc.certType, got, want)
		}
		if got, want := blessings.Expiry(), pubCerts[0].NotAfter; got != want {
			t.Errorf("%v: got %v, want %v", tc.certType, got, want)
		}
		client := sectest.NewX509Principal(t, &opts)
		call := security.NewCall(&security.CallParams{
			LocalPrincipal:  client,
			RemoteBlessings: blessings,
			Timestamp:       pubCerts[0].NotBefore.Add(48 * time.Hour),
		})
		names, rejected := security.RemoteBlessingNames(ctx, call)
		if len(rejected) != 0 {
			t.Errorf("%v: ejected blessings: %v", tc.certType, rejected)
		}
		if got, want := names, tc.hosts; !reflect.DeepEqual(got, want) {
			t.Errorf("%v: got %v, want %v", tc.certType, got, want)
		}
		validHosts := tc.validHosts
		if validHosts == nil {
			validHosts = tc.hosts
		}
		if !blessings.CouldHaveNames(validHosts) {
			t.Errorf("%v: CouldHaveNames is false for: %v", tc.certType, validHosts)
		}
	}
}

func TestX509WildcardDomain(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()
	privKey, pubCerts, opts := sectestdata.LetsEncryptData(sectestdata.WildcardCert)
	server := sectest.NewX509ServerPrincipal(t, privKey, pubCerts, &opts)
	blessings, _ := server.BlessingStore().Default()
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
	t.Errorf("line: %v: blessings error message is not one of %v: %v", line, rejected[0].Err.Error(), msgs)
}

func TestX509ClientErrors(t *testing.T) {
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
		hosts        []string
		invalidHosts []string
	}{
		{sectestdata.SingleHostCert, s("www.labdrive.io"), s("x.labdrive.io")},
		{sectestdata.MultipleHostsCert, s("a.labdrive.io", "b.labdrive.io", "c.labdrive.io"), s("x.labdrive.io")},
		{sectestdata.WildcardCert, s("*.labdrive.io"), s("foo.bar.labdrive.io", ".labdrive.io")},
	} {
		privKey, pubCerts, opts := sectestdata.LetsEncryptData(tc.certType)
		server := sectest.NewX509ServerPrincipal(t, privKey, pubCerts, &opts)
		blessings, _ := server.BlessingStore().Default()

		client := sectest.NewX509Principal(t, &opts)

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
		client = sectest.NewX509Principal(t, &x509.VerifyOptions{
			CurrentTime: pubCerts[0].NotBefore.Add(48 * time.Hour),
		})
		call = security.NewCall(&security.CallParams{
			LocalPrincipal:  client,
			RemoteBlessings: blessings,
			Timestamp:       pubCerts[0].NotBefore.Add(48 * time.Hour),
		})

		names, rejected = security.RemoteBlessingNames(ctx, call)
		validate("x509: certificate signed by unknown authority")

		// No custom options.
		client = sectest.NewX509Principal(t, &x509.VerifyOptions{})
		call = security.NewCall(&security.CallParams{
			LocalPrincipal:  client,
			RemoteBlessings: blessings,
			Timestamp:       pubCerts[0].NotBefore.Add(48 * time.Hour),
		})

		names, rejected = security.RemoteBlessingNames(ctx, call)
		validate("x509: certificate has expired or is not yet valid", "x509: certificate signed by unknown authority")

		if !blessings.CouldHaveNames(tc.hosts) {
			t.Errorf("%v: CouldHaveNames is false for: %v", tc.certType, tc.hosts)
		}

		if blessings.CouldHaveNames(tc.invalidHosts) {
			t.Errorf("%v: CouldHaveNames is true for: %v", tc.certType, tc.invalidHosts)
		}
	}
}

func TestX509ServerErrors(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()

	var names []string
	var rejected []security.RejectedBlessing
	validate := func(msgs ...string) {
		validateRejected(t, names, rejected, msgs)
	}

	for _, tc := range []struct {
		certType     sectestdata.CertType
		hosts        []string
		invalidHosts []string
	}{
		{sectestdata.SingleHostCert, []string{"www.labdrive.io"}, []string{"x.labdrive.io"}},
		{sectestdata.MultipleHostsCert, []string{"a.labdrive.io", "b.labdrive.io", "c.labdrive.io"}, []string{"x.labdrive.io"}},
		{sectestdata.WildcardCert, []string{"*.labdrive.io"}, []string{"foo.labdrive.io", "bar.labdrive.io"}},
	} {

		privKey, pubCerts, _ := sectestdata.LetsEncryptData(tc.certType)

		server := sectest.NewX509ServerPrincipal(t, privKey, pubCerts, nil)
		blessings, _ := server.BlessingStore().Default()

		names = security.BlessingNames(server, blessings)
		if len(names) != 0 {
			t.Errorf("no blessing names should be valid without custom x509 verification options")
		}

		client := sectest.NewX509Principal(t, nil)
		call := security.NewCall(&security.CallParams{
			LocalPrincipal:  client,
			RemoteBlessings: blessings,
		})
		names, rejected = security.RemoteBlessingNames(ctx, call)
		validate("x509: certificate has expired or is not yet valid", "x509: certificate signed by unknown authority")

		if !blessings.CouldHaveNames(tc.hosts) {
			t.Errorf("%v: CouldHaveNames is false for: %v", tc.certType, tc.hosts)
		}
		if blessings.CouldHaveNames(tc.invalidHosts) {
			t.Errorf("%v: CouldHaveNames is true for: %v", tc.certType, tc.invalidHosts)
		}
	}
}
