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

	privKey, pubCerts, opts := sectestdata.LetsEncryptData()

	server := sectest.NewX509ServerPrincipal(t, privKey, pubCerts, &opts)
	blessings, _ := server.BlessingStore().Default()
	names := security.BlessingNames(server, blessings)
	if got, want := names, []string{"www.labdrive.io"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if got, want := blessings.Expiry(), pubCerts[0].NotAfter; got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	client := sectest.NewX509Principal(t, &opts)
	call := security.NewCall(&security.CallParams{
		LocalPrincipal:  client,
		RemoteBlessings: blessings,
		Timestamp:       pubCerts[0].NotBefore.Add(48 * time.Hour),
	})
	names, rejected := security.RemoteBlessingNames(ctx, call)
	if len(rejected) != 0 {
		t.Errorf("rejected blessings: %v", rejected)
	}
	if got, want := names, []string{"www.labdrive.io"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

}

func TestX509ClientErrors(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()

	privKey, pubCerts, opts := sectestdata.LetsEncryptData()

	server := sectest.NewX509ServerPrincipal(t, privKey, pubCerts, &opts)
	blessings, _ := server.BlessingStore().Default()

	client := sectest.NewX509Principal(t, &opts)

	var names []string
	var rejected []security.RejectedBlessing
	validate := func(msg string) {
		_, _, line, _ := runtime.Caller(1)
		if len(names) != 0 {
			t.Errorf("line: %v: unexpected blessing names: %v", line, names)
		}
		if len(rejected) == 0 || !strings.Contains(rejected[0].Err.Error(), msg) {
			t.Errorf("line: %v: incorrect rejected blessings: %v", line, rejected)
		}
	}

	// After expiration, ie. now.
	call := security.NewCall(&security.CallParams{
		LocalPrincipal:  client,
		RemoteBlessings: blessings,
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
	validate("x509: certificate has expired or is not yet valid")
}

func TestX509ServerErrors(t *testing.T) {
	ctx, cancel := context.RootContext()
	defer cancel()

	privKey, pubCerts, _ := sectestdata.LetsEncryptData()

	server := sectest.NewX509ServerPrincipal(t, privKey, pubCerts, nil)
	blessings, _ := server.BlessingStore().Default()

	var names []string
	var rejected []security.RejectedBlessing
	validate := func(msg string) {
		_, _, line, _ := runtime.Caller(1)
		if len(names) != 0 {
			t.Errorf("line: %v: unexpected blessing names: %v", line, names)
		}
		if len(rejected) == 0 || !strings.Contains(rejected[0].Err.Error(), msg) {
			t.Errorf("line: %v: incorrect rejected blessings: %v", line, rejected)
		}
	}

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
	validate("x509: certificate has expired or is not yet valid")

}
